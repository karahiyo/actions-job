package adapter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const endpoint = "http://metadata.google.internal"

type Metadata struct {
	ProjectID string
	Region    string
}

func GetInstanceMetadata(ctx context.Context) (*Metadata, error) {
	meta := new(Metadata)
	metadataClient, err := NewMetadataClient()
	if err != nil {
		return nil, fmt.Errorf("failed to NewMetadataClient: %w", err)
	}

	if meta.ProjectID, err = metadataClient.getProjectID(ctx); err != nil {
		return nil, fmt.Errorf("failed to getProjectID: %w", err)
	}

	if meta.Region, err = metadataClient.getRegion(ctx); err != nil {
		return nil, fmt.Errorf("failed to getRegion: %w", err)
	}

	return meta, nil
}

type MetadataClient struct {
	cli *http.Client
}

func NewMetadataClient() (*MetadataClient, error) {
	cli := &http.Client{
		Timeout: 1 * time.Second,
	}

	return &MetadataClient{cli: cli}, nil
}

func (m *MetadataClient) getProjectID(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/computeMetadata/v1/project/project-id", endpoint), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Metadata-Flavor", "Google")

	resp, err := m.cli.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get region: %w", err)
	}
	defer resp.Body.Close() // nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}

func (m *MetadataClient) getRegion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/computeMetadata/v1/instance/region", endpoint), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Metadata-Flavor", "Google")

	resp, err := m.cli.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get region: %w", err)
	}
	defer resp.Body.Close() // nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	region := string(body)
	// for example, for the following string: `/project/xxx/regions/yyy`
	if strings.Contains(region, "/regions/") {
		splitted := strings.Split(region, "/")
		region = splitted[len(splitted)-1]
	}

	return region, nil
}
