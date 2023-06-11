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
	ghAdapter adapter.GitHubAdapter
	validate  *validator.Validate
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

	return &Controller{
		ghAdapter: ghAdapter,
		validate:  validator.New(),
	}, nil
}

func (c *Controller) ReceiveWorkflowJobEvent(ctx context.Context, event *github.WorkflowJobEvent) error {
	logger := zerolog.Ctx(ctx)

	if !event.GetRepo().GetPrivate() {
		return fmt.Errorf("skipped. using self-hosted runner with public repositories is a security vulnerability: %w", ErrBadRequest)
	}

	if event.GetRepo().GetFork() {
		return fmt.Errorf("skipped. using self-hosted runner with forked repositories is a security vulnerability: %w", ErrBadRequest)
	}

	if event.GetAction() != "queued" {
		return fmt.Errorf("event is not \"queued\" action: %w", ErrNonTargetEvent)
	}

	ownerRepo := strings.Split(event.GetRepo().GetFullName(), "/")
	owner := ownerRepo[0]
	repo := ownerRepo[1]

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

	// if job has dood enabled option, add sidecar container to job manifest
	if isDockerEnabled(job) {
		job = useDoodContainer(job)
		job = addSidecarDockerContainer(job)
	}

	project := labeledOpts.project
	region := labeledOpts.region
	if project == "" || region == "" {
		instanceMeta, err := adapter.GetInstanceMetadata(ctx)
		if err != nil {
			return fmt.Errorf("failed to get instance metadata: %w", err)
		}

		logger.Debug().Msgf("get actions-job-controller instance metadata: %s", spew.Sdump(instanceMeta))

		if project == "" {
			project = instanceMeta.ProjectID
		}

		if region == "" {
			region = instanceMeta.Region
		}
	}

	logger.Info().Msgf("detected job launch options: project=%s, region=%s", project, region)

	if err := c.dispatchJobTransaction(ctx, project, region, jobName, job); err != nil {
		return fmt.Errorf("failed to dispatch job: %w", err)
	}

	return nil
}

// dispatchJobTransaction Create/Update Cloud Run Jobs and start a job execution
func (c *Controller) dispatchJobTransaction(ctx context.Context, project, region, jobName string, job *run.Job) error {
	logger := zerolog.Ctx(ctx)
	var err error

	jobsAdapter, err := adapter.NewJobsAdapter(ctx, project, region)
	if err != nil {
		return fmt.Errorf("failed to initialize jobs client: %w", err)
	}

	exists, err := jobsAdapter.GetJob(ctx, jobName)
	if err != nil && !errors.Is(err, adapter.ErrJobNotFound) {
		return fmt.Errorf("failed to check job exists: %w", err)
	}

	if exists == nil {
		logger.Info().Msgf("job does not exists. creating new job...")

		created, err := jobsAdapter.CreateJob(ctx, job)
		if err != nil {
			return fmt.Errorf("failed to create job: %w", err)
		}

		logger.Info().Msgf("success to create a new job: %s", spew.Sdump(created))
	} else {
		log.Info().Msgf("job already exists. updating job...")
		// TODO: check need to update current job

		updated, err := jobsAdapter.UpdateJob(ctx, jobName, job)
		if err != nil {
			return fmt.Errorf("failed to update job: job=%s, %w", spew.Sdump(job), err)
		}

		logger.Info().Msgf("success to update the job: %s", spew.Sdump(updated))
	}

	_, err = jobsAdapter.WaitJobReady(ctx, jobName)
	if err != nil {
		return fmt.Errorf("failed to wait job ready: %w", err)
	}

	newExecution, err := jobsAdapter.StartJob(ctx, jobName)
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

func isDockerEnabled(job *run.Job) bool {
	for _, container := range job.Spec.Template.Spec.Template.Spec.Containers {
		for _, env := range container.Env {
			if env.Name == "DOCKER_ENABLED" && env.Value == "true" {
				return true
			}
		}
	}
	return false
}

func useDoodContainer(job *run.Job) *run.Job {
	updated := job.Spec.Template.Spec.Template.Spec.Containers[0]
	if !strings.Contains(updated.Image, "/actions-job-dood") {
		updated.Image = strings.Replace(updated.Image, "/actions-job", "/actions-job-dood", 1)
	}

	job.Spec.Template.Spec.Template.Spec.Containers[0] = updated

	return job
}

func addSidecarDockerContainer(job *run.Job) *run.Job {
	job.Spec.Template.Spec.Template.Spec.Containers = append(
		job.Spec.Template.Spec.Template.Spec.Containers,
		&run.Container{
			Name:  "docker",
			Image: "docker:dind-rootless",
		})

	return job
}

type labeledOptions struct {
	project     string
	region      string
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
