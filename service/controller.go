package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/go-playground/validator/v10"
	"github.com/google/go-github/v52/github"
	"github.com/karahiyo/actions-job/adapter"
	"github.com/karahiyo/actions-job/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/run/v1"
	k8syaml "sigs.k8s.io/yaml"
)

type Controller struct {
	ghAdapter   adapter.GitHubAdapter
	jobsAdapter adapter.JobsAdapter
	validate    *validator.Validate
}

var (
	ErrNonTargetEvent = fmt.Errorf("non target event")
	ErrBadRequest     = fmt.Errorf("bad request")
)

func NewController(ctx context.Context) (*Controller, error) {
	ghAdapter, err := adapter.NewGitHubAdapter(config.GetGitHubAppConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize github client: %w", err)
	}

	jobsAdapter, err := adapter.NewJobsAdapter(ctx, config.GetGCPConfig().Region)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize jobs client: %w", err)
	}

	return &Controller{
		ghAdapter:   ghAdapter,
		jobsAdapter: jobsAdapter,
		validate:    validator.New(),
	}, nil
}

func (c *Controller) ReceiveWorkflowJobEvent(ctx context.Context, event *github.WorkflowJobEvent) error {
	logger := zerolog.Ctx(ctx)

	if !event.GetRepo().GetPrivate() {
		return fmt.Errorf("skipped. using self-hosted runner with public repositories is very dangerous: %w", ErrBadRequest)
	}

	if event.GetRepo().GetFork() {
		return fmt.Errorf("skipped. using self-hosted runner with forked repositories is very dangerous: %w", ErrBadRequest)
	}

	if event.GetAction() != "queued" {
		return fmt.Errorf("event is not \"queued\" action: %w", ErrNonTargetEvent)
	}

	labels := event.GetWorkflowJob().Labels
	if !includeSelfHostedLabel(labels) {
		return fmt.Errorf("label \"self-hosted\" is not found in labels: %w", ErrNonTargetEvent)
	}

	labeledOpts := getOptionsFromLabels(labels)
	if err := c.validate.Struct(labeledOpts); err != nil {
		var ve validator.ValidationErrors
		if errors.As(err, &ve) {
			return fmt.Errorf("validation error: %w, %w", ve, ErrNonTargetEvent)
		}

		return fmt.Errorf("validation error: err = %w", err)
	}

	ownerRepo := strings.Split(event.GetRepo().GetFullName(), "/")
	owner := ownerRepo[0]
	repo := ownerRepo[1]

	// default use current commit SHA as ref.
	// TODO: support manual specification of ref
	ref := event.GetWorkflowJob().GetHeadSHA()

	runnerManifest, err := c.ghAdapter.DownloadContent(ctx, owner, repo, labeledOpts.jobManifest, ref)
	if err != nil {
		return fmt.Errorf("failed to download actions runner config: %w", err)
	}
	logger.Debug().Msgf("runner config yaml: %s", runnerManifest)

	job, err := parseJobManifest([]byte(runnerManifest))
	if err != nil {
		return fmt.Errorf("failed to parse job manifest: manifest=%s, %w", runnerManifest, err)
	}

	jobName := job.Metadata.Name
	job = updateJobManifest(job, jobEnvs{
		owner:  owner,
		repo:   repo,
		labels: labels,
	})

	if err := c.dispatchJobTransaction(ctx, labeledOpts.project, jobName, job); err != nil {
		return fmt.Errorf("failed to dispatch job: %w", err)
	}

	return nil
}

// dispatchJobTransaction Create/Update Cloud Run Jobs and start a job execution
func (c *Controller) dispatchJobTransaction(ctx context.Context, project, jobName string, job *run.Job) error {
	logger := zerolog.Ctx(ctx)
	var err error

	exists, err := c.jobsAdapter.GetJob(ctx, project, jobName)
	if err != nil && !errors.Is(err, adapter.ErrJobNotFound) {
		return fmt.Errorf("failed to check job exists: %w", err)
	}

	if exists == nil {
		logger.Info().Msgf("job does not exists. creating new job...")

		created, err := c.jobsAdapter.CreateJob(ctx, project, job)
		if err != nil {
			return fmt.Errorf("failed to create job: %w", err)
		}

		logger.Info().Msgf("success to create a new job: %s", spew.Sdump(created))
	} else {
		log.Info().Msgf("job already exists. updating job...")
		// TODO: check need to update current job

		updated, err := c.jobsAdapter.UpdateJob(ctx, project, jobName, job)
		if err != nil {
			return fmt.Errorf("failed to update job: job=%s, %w", spew.Sdump(job), err)
		}

		logger.Info().Msgf("success to update the job: %s", spew.Sdump(updated))
	}

	_, err = c.jobsAdapter.WaitJobReady(ctx, project, jobName)
	if err != nil {
		return fmt.Errorf("failed to wait job ready: %w", err)
	}

	newExecution, err := c.jobsAdapter.StartJob(ctx, project, jobName)
	if err != nil {
		return fmt.Errorf("failed to start job: %w", err)
	}

	logger.Info().Msgf("success to start job: %v", spew.Sdump(newExecution))

	return nil
}

// includeSelfHostedLabel check if label "self-hosted" is included in labels
func includeSelfHostedLabel(labels []string) bool {
	for _, label := range labels {
		if strings.EqualFold(label, "self-hosted") {
			return true
		}
	}
	return false
}

type labeledOptions struct {
	project     string `required:"true"`
	region      string `required:"true"`
	jobManifest string `required:"true"`
}

func getOptionsFromLabels(labels []string) labeledOptions {
	opts := labeledOptions{}

	for _, label := range labels {
		kv := strings.SplitN(label, "=", 2)
		if len(kv) != 2 {
			continue
		}

		switch kv[0] {
		case "project":
			opts.project = kv[1]
		case "region":
			opts.region = kv[1]
		case "job-manifest":
			opts.jobManifest = kv[1]
		default:
			continue
		}
	}

	return opts
}

func parseJobManifest(in []byte) (*run.Job, error) {
	var j run.Job
	if err := k8syaml.Unmarshal(in, &j); err != nil {
		return nil, fmt.Errorf("failed to k8syaml unmarshall: %w", err)
	}

	return &j, nil
}

type jobEnvs struct {
	// location string // TODO: use this field to create job
	owner  string
	repo   string
	labels []string
}

func updateJobManifest(job *run.Job, opts jobEnvs) *run.Job {
	envs := job.Spec.Template.Spec.Template.Spec.Containers[0].Env

	envs = append(envs, &run.EnvVar{Name: "OWNER", Value: opts.owner})
	envs = append(envs, &run.EnvVar{Name: "REPO", Value: opts.repo})
	envs = append(envs, &run.EnvVar{Name: "LABELS", Value: strings.Join(opts.labels, ",")})

	job.Spec.Template.Spec.Template.Spec.Containers[0].Env = envs

	return job
}
