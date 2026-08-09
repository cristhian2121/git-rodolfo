package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/lean-tech/git-rodolfo/internal/app"
)

const (
	// releaseAPITimeout bounds the metadata lookup (RNF-03).
	releaseAPITimeout = 10 * time.Second
	// releaseDownloadTimeout is longer: a release tarball is a few MB, not
	// the handful of bytes a metadata call exchanges.
	releaseDownloadTimeout = 60 * time.Second
)

// GitHubReleaseFetcher implements app.ReleaseFetcher against the real
// GitHub Releases API for one repo ("owner/repo").
type GitHubReleaseFetcher struct {
	Repo string
}

// NewGitHubReleaseFetcher builds a GitHubReleaseFetcher for repo
// ("owner/repo").
func NewGitHubReleaseFetcher(repo string) *GitHubReleaseFetcher {
	return &GitHubReleaseFetcher{Repo: repo}
}

type ghReleaseResponse struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func (g *GitHubReleaseFetcher) Latest() (app.ReleaseInfo, error) {
	return g.fetch(fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", g.Repo))
}

func (g *GitHubReleaseFetcher) ByTag(tag string) (app.ReleaseInfo, error) {
	return g.fetch(fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", g.Repo, tag))
}

func (g *GitHubReleaseFetcher) fetch(url string) (app.ReleaseInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), releaseAPITimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return app.ReleaseInfo{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return app.ReleaseInfo{}, fmt.Errorf("fetch release metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return app.ReleaseInfo{}, fmt.Errorf("no release found (repo %q, %s)", g.Repo, url)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return app.ReleaseInfo{}, fmt.Errorf("GitHub API returned %s: %s", resp.Status, string(body))
	}

	var r ghReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return app.ReleaseInfo{}, fmt.Errorf("parse release metadata: %w", err)
	}

	info := app.ReleaseInfo{TagName: r.TagName}
	for _, a := range r.Assets {
		info.Assets = append(info.Assets, app.ReleaseAsset{Name: a.Name, DownloadURL: a.BrowserDownloadURL})
	}
	return info, nil
}

// HTTPAssetDownloader implements app.AssetDownloader via a plain HTTP GET.
type HTTPAssetDownloader struct{}

// NewHTTPAssetDownloader builds an HTTPAssetDownloader.
func NewHTTPAssetDownloader() *HTTPAssetDownloader {
	return &HTTPAssetDownloader{}
}

func (d *HTTPAssetDownloader) Download(url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), releaseDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: unexpected status %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}
