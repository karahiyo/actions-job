package adapter

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v52/github"
	"github.com/karahiyo/actions-job/config"
)

type GitHubAdapter interface {
	DownloadContent(ctx context.Context, owner, repo, path, ref string) (string, error)
}

type gitHubAdapter struct {
	ghClient *github.Client
}

func NewGitHubAdapter(conf config.GitHubAppConfig) (GitHubAdapter, error) {
	// Shared transport to reuse TCP connections
	tr := http.DefaultTransport

	// Wrap the shared transport for use with GitHub App authentication.
	itr, err := ghinstallation.New(
		tr,
		conf.AppID,
		conf.InstallationID,
		[]byte(conf.PrivateKey),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to ghinstallation.New: %w", err)
	}

	// Use installation transport
	c := github.NewClient(&http.Client{
		Transport: itr,
		Timeout:   conf.RequestTimeout,
	})

	return &gitHubAdapter{ghClient: c}, nil
}

func (c *gitHubAdapter) DownloadContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	content, _, _, err := c.ghClient.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{Ref: ref})
	if err != nil {
		return "", fmt.Errorf("failed to download github repository content: owner=%s, repo=%s, path=%s, ref=%s, err=%w", owner, repo, path, ref, err)
	}

	decoded, err := content.GetContent()
	if err != nil {
		return "", fmt.Errorf("failed to decode content body: %w", err)
	}

	return decoded, nil
}
