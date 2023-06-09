package adapter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/rs/zerolog"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/run/v1"
)

type JobsAdapter interface {
	GetJob(ctx context.Context, name string) (*run.Job, error)
	CreateJob(ctx context.Context, job *run.Job) (*run.Job, error)
	UpdateJob(ctx context.Context, name string, job *run.Job) (*run.Job, error)
	StartJob(ctx context.Context, name string) (*run.Execution, error)
	WaitJobReady(ctx context.Context, name string) (bool, error)
}

type jobsAdapter struct {
	api     *run.APIService
	project string
	region  string
}

var ErrJobNotFound = errors.New("job not found")

func NewJobsAdapter(ctx context.Context, project, region string) (JobsAdapter, error) {
	var opts []option.ClientOption
	opts = append(opts, option.WithEndpoint(fmt.Sprintf("https://%s-run.googleapis.com", region)))

	s, err := run.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize jobs v1 api jobsAdapter: %w", err)
	}
	return &jobsAdapter{
		project: project,
		region:  region,
		api:     s,
	}, nil
}

func (a *jobsAdapter) CreateJob(ctx context.Context, job *run.Job) (*run.Job, error) {
	parent := fmt.Sprintf("namespaces/%s", a.project)
	res, err := a.api.Namespaces.Jobs.Create(parent, job).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to CreateJob: job=%s, %w", spew.Sdump(job), err)
	}

	return res, nil
}

func (a *jobsAdapter) UpdateJob(ctx context.Context, name string, job *run.Job) (*run.Job, error) {
	jobID := fmt.Sprintf("namespaces/%s/jobs/%s", a.project, name)
	res, err := a.api.Namespaces.Jobs.ReplaceJob(jobID, job).Do()
	if err != nil {
		var gErr *googleapi.Error
		if errors.As(err, &gErr) && gErr.Code == 404 {
			return nil, fmt.Errorf("job dose not found: name=%s, %w", jobID, ErrJobNotFound)
		}

		return nil, fmt.Errorf("failed to ReplaceJob: name=%s, job=%s, %w", name, spew.Sdump(job), err)
	}

	return res, nil
}

func (a *jobsAdapter) GetJob(ctx context.Context, name string) (*run.Job, error) {
	jobID := fmt.Sprintf("namespaces/%s/jobs/%s", a.project, name)
	job, err := a.api.Namespaces.Jobs.Get(jobID).Do()
	if err != nil {
		var gErr *googleapi.Error
		if errors.As(err, &gErr) && gErr.Code == 404 {
			return nil, fmt.Errorf("job dose not found: name=%s, %w", jobID, ErrJobNotFound)
		}

		return nil, fmt.Errorf("failed to get job: name=%s, %w", name, err)
	}

	return job, nil
}

func (a *jobsAdapter) StartJob(ctx context.Context, name string) (*run.Execution, error) {
	jobID := fmt.Sprintf("namespaces/%s/jobs/%s", a.project, name)
	runJobRequest := &run.RunJobRequest{
		// FIXME: Using the 'namespaces.jobs.run' api with a container override option will result in a 403 permission denied error, even through a service account that has the run.jobs.runWithOverrides permission.
		Overrides: nil,
	}

	execution, err := a.api.Namespaces.Jobs.Run(jobID, runJobRequest).Do()
	if err != nil {
		var gErr *googleapi.Error
		if errors.As(err, &gErr) && gErr.Code == 404 {
			return nil, fmt.Errorf("job dose not found: name=%s, %w", jobID, ErrJobNotFound)
		}

		return nil, fmt.Errorf("failed to start job: name=%s, req=%s, res=%s, %w", name, spew.Sdump(runJobRequest), spew.Sdump(execution), err)
	}

	return execution, nil
}

// WaitJobReady Wait until the job's Ready status condition is True
func (a *jobsAdapter) WaitJobReady(ctx context.Context, name string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	logger := zerolog.Ctx(ctx)
	logger.Debug().Msgf("waiting for the job to be ready: name=%s", name)

	nextWait := 10 * time.Millisecond
	for {
		job, err := a.GetJob(ctx, name)
		if err != nil {
			return false, fmt.Errorf("failed to get job: name=%s, %w", name, err)
		}

		for _, condition := range job.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				logger.Debug().Msgf("job ready: name=%s", name)
				return true, nil
			}
		}

		time.Sleep(nextWait)
		nextWait = nextWait * 2
	}
}
