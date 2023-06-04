package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/google/go-github/v52/github"
	"github.com/karahiyo/actions-job/adapter"
	"github.com/karahiyo/actions-job/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Controller struct {
	ghAdapter   adapter.GitHubAdapter
	jobsAdapter adapter.JobsAdapter
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

	jobsAdapter, err := adapter.NewJobsAdapter(ctx, config.GetGCPConfig().CloudRunAdminApiEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize jobs client: %w", err)
	}

	return &Controller{
		ghAdapter:   ghAdapter,
		jobsAdapter: jobsAdapter,
	}, nil
}

func (c *Controller) ReceiveWorkflowJobEvent(ctx context.Context, event *github.WorkflowJobEvent) error {
	logger := zerolog.Ctx(ctx)
	logger.Debug().Msgf("workflow current event detail: action=%s , event=%s", event.GetAction(), spew.Sdump(event.GetWorkflowJob()))

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

	project, ok := getTargetProjectFromLabels(labels)
	if !ok {
		return fmt.Errorf("failed to get target project from labels: %w", ErrNonTargetEvent)
	}

	location, ok := getTargetLocationFromLabels(labels)
	if !ok {
		return fmt.Errorf("failed to get target location from labels: %w", ErrNonTargetEvent)
	}

	runnerConfigPath, ok := getActionsRunnerConfigPathLabel(labels)
	if !ok {
		return fmt.Errorf("failed to get runner config path label: %w", ErrNonTargetEvent)
	}

	// dispatch cloud run jobs
	// get current configuration from event payload
	ownerRepo := strings.Split(event.GetRepo().GetFullName(), "/")
	owner := ownerRepo[0]
	repo := ownerRepo[1]

	// default use current commit SHA as ref.
	// TODO: support specify ref manually.
	ref := event.GetWorkflowJob().GetHeadSHA()

	runnerManifest, err := c.ghAdapter.DownloadContent(ctx, owner, repo, runnerConfigPath, ref)
	if err != nil {
		return fmt.Errorf("failed to download actions runner config: %w", err)
	}
	logger.Debug().Msgf("runner config yaml: %s", runnerManifest)

	job, err := adapter.ParseJobManifest([]byte(runnerManifest))
	if err != nil {
		return fmt.Errorf("failed to parse job manifest: manifest=%s, %w", runnerManifest, err)
	}

	jobName := job.Metadata.Name
	additionalEnvs := adapter.AdditionalEnvs{
		Location:        location,
		RepositoryOwner: owner,
		RepositoryName:  repo,
		Labels:          labels,
	}

	// check job already exists or not.
	currentJob, err := c.jobsAdapter.GetJob(ctx, project, jobName)
	if err != nil {
		// TODO: confirm error handling, if job not found then throw error?
		return fmt.Errorf("failed to get job: %w", err)
	}
	if currentJob != nil {
		log.Debug().Msgf("job already exists")
		// TODO: check need to update current

		_, err = c.jobsAdapter.UpdateJob(ctx, project, jobName, job, additionalEnvs)
		if err != nil {
			return fmt.Errorf("failed to update job: job=%+v, %w", job, err)
		}
	} else {
		log.Debug().Msgf("job does not exists. creating new job...")

		if _, err = c.jobsAdapter.CreateJob(ctx, project, job, additionalEnvs); err != nil {
			return fmt.Errorf("failed to create current: %w", err)
		}
	}

	// Wait until the job's Ready status condition is True
	_, err = c.jobsAdapter.WaitJobReady(ctx, project, jobName)
	if err != nil {
		return fmt.Errorf("failed to wait job ready: %w", err)
	}

	execution, err := c.jobsAdapter.StartJob(ctx, project, jobName)
	if err != nil {
		return fmt.Errorf("failed to start job: %w", err)
	}

	logger.Info().Msgf("success to start job: %+v", execution)

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

func getTargetProjectFromLabels(labels []string) (string, bool) {
	for _, label := range labels {
		if strings.HasPrefix(label, "project=") {
			return strings.TrimPrefix(label, "project="), true
		}
	}

	return "", false
}

func getTargetLocationFromLabels(labels []string) (string, bool) {
	for _, label := range labels {
		if strings.HasPrefix(label, "location=") {
			return strings.TrimPrefix(label, "location="), true
		}
	}

	return "", false
}

func getActionsRunnerConfigPathLabel(labels []string) (string, bool) {
	for _, label := range labels {
		if strings.HasPrefix(label, "runner-config=") {
			return strings.TrimPrefix(label, "runner-config="), true
		}
	}

	return "", false
}
