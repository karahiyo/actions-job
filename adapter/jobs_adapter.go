package adapter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/rs/zerolog"
	"google.golang.org/api/option"
	"google.golang.org/api/run/v1"
	k8syaml "sigs.k8s.io/yaml"
)

type JobsAdapter interface {
	GetJob(ctx context.Context, project, name string) (*run.Job, error)
	CreateJob(ctx context.Context, project string, job *run.Job, opts AdditionalEnvs) (*run.Job, error)
	UpdateJob(ctx context.Context, project, name string, job *run.Job, opts AdditionalEnvs) (*run.Job, error)
	StartJob(ctx context.Context, project, name string) (*run.Execution, error)
	WaitJobReady(ctx context.Context, project, name string) (bool, error)
}

type jobsAdapter struct {
	api *run.APIService
}

func NewJobsAdapter(ctx context.Context, adminApiEndpoint string) (JobsAdapter, error) {
	s, err := run.NewService(ctx, option.WithEndpoint(adminApiEndpoint))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize jobs v1 api jobsAdapter: %w", err)
	}
	return &jobsAdapter{
		api: s,
	}, nil
}

type AdditionalEnvs struct {
	Location        string // TODO: use this field to create job
	RepositoryOwner string
	RepositoryName  string
	Labels          []string
}

func (a *jobsAdapter) CreateJob(ctx context.Context, project string, job *run.Job, opts AdditionalEnvs) (*run.Job, error) {
	logger := zerolog.Ctx(ctx)

	envAdded := updateJobEnvs(job, opts)

	parent := fmt.Sprintf("namespaces/%s", project)
	res, err := a.api.Namespaces.Jobs.Create(parent, envAdded).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to CreateJob: project=%s, job=%s, %w", project, spew.Sdump(job), err)
	}

	logger.Info().Msgf("success to create job: %+v", res)

	return res, nil
}

func (a *jobsAdapter) UpdateJob(ctx context.Context, project, name string, job *run.Job, opts AdditionalEnvs) (*run.Job, error) {
	logger := zerolog.Ctx(ctx)

	envAdded := updateJobEnvs(job, opts)

	jobID := fmt.Sprintf("namespaces/%s/jobs/%s", project, name)
	res, err := a.api.Namespaces.Jobs.ReplaceJob(jobID, envAdded).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to ReplaceJob: project=%s, name=%s, job=%s, %w", project, name, spew.Sdump(job), err)
	}

	logger.Info().Msgf("success to update job: %s", spew.Sdump(res))

	return res, nil
}

func (a *jobsAdapter) GetJob(ctx context.Context, project, name string) (*run.Job, error) {
	jobID := fmt.Sprintf("namespaces/%s/jobs/%s", project, name)
	job, err := a.api.Namespaces.Jobs.Get(jobID).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get job: project=%s, name=%s, %w", project, name, err)
	}

	return job, nil
}

func (a *jobsAdapter) StartJob(ctx context.Context, project, name string) (*run.Execution, error) {
	jobID := fmt.Sprintf("namespaces/%s/jobs/%s", project, name)
	runJobRequest := &run.RunJobRequest{
		// FIXME: Getting a permission error when trying to run.jobs.run api with a container override, even though a service account that has the run.jobs.runWithOverrides permission.
		Overrides: nil,
	}

	execution, err := a.api.Namespaces.Jobs.Run(jobID, runJobRequest).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to start job: project=%s, name=%s, req=%s, %w", project, name, spew.Sdump(runJobRequest), err)
	}

	return execution, nil
}

// WaitJobReady Wait until the job's Ready status condition is True
func (a *jobsAdapter) WaitJobReady(ctx context.Context, project, name string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	logger := zerolog.Ctx(ctx)
	logger.Debug().Msgf("waiting for job ready: project=%s, name=%s", project, name)

	nextWait := 10 * time.Millisecond
	for {
		job, err := a.GetJob(ctx, project, name)
		if err != nil {
			return false, fmt.Errorf("failed to get job: project=%s, name=%s, %w", project, name, err)
		}

		for _, condition := range job.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				logger.Debug().Msgf("job ready: project=%s, name=%s", project, name)
				return true, nil
			}
		}

		logger.Debug().Msgf("waiting for job ready %v: project=%s, name=%s", nextWait, project, name)
		time.Sleep(nextWait)
		nextWait = nextWait * 2
	}
}

func ParseJobManifest(in []byte) (*run.Job, error) {
	var j run.Job
	if err := k8syaml.Unmarshal(in, &j); err != nil {
		return nil, fmt.Errorf("failed to k8syaml unmarshall: %w", err)
	}

	return &j, nil
}

func updateJobEnvs(job *run.Job, opts AdditionalEnvs) *run.Job {
	envs := job.Spec.Template.Spec.Template.Spec.Containers[0].Env

	envs = append(envs, &run.EnvVar{Name: "OWNER", Value: opts.RepositoryOwner})
	envs = append(envs, &run.EnvVar{Name: "REPO", Value: opts.RepositoryName})
	envs = append(envs, &run.EnvVar{Name: "LABELS", Value: strings.Join(opts.Labels, ",")})

	job.Spec.Template.Spec.Template.Spec.Containers[0].Env = envs

	return job
}
