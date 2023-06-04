package adapter

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/run/v1"
)

func TestJobsClient_ParseJobManifest(t *testing.T) {
	type args struct {
		manifest string
	}
	tests := []struct {
		want *run.Job
		name string
		args args
	}{
		{
			name: "sample manifest",
			args: args{manifest: `
apiVersion: "run.googleapis.com/v1"
kind: Job
metadata:
  name: actions-runner-job
spec:
  template:
    spec:
      template:
        spec:
          containers:
            - image: karahiyo/actions-runner:latest
`},
			want: &run.Job{
				ApiVersion: "run.googleapis.com/v1",
				Kind:       "Job",
				Metadata:   &run.ObjectMeta{Name: "actions-runner-job"},
				Spec: &run.JobSpec{
					Template: &run.ExecutionTemplateSpec{
						Spec: &run.ExecutionSpec{
							Template: &run.TaskTemplateSpec{
								Spec: &run.TaskSpec{
									Containers: []*run.Container{
										{
											Image: "karahiyo/actions-runner:latest",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "empty manifest",
			args: args{manifest: ""},
			want: &run.Job{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseJobManifest([]byte(tt.args.manifest))
			if err != nil {
				t.Errorf("failed to parse manifest: %v", err)
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("jobsAdapter.ParseManifest() mismatch (-want +got):\n%s", d)
			}
		})
	}
}
