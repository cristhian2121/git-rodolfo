package fakes

import (
	"fmt"

	"github.com/lean-tech/git-rodolfo/internal/app"
)

// FakeReleaseFetcher is an in-memory app.ReleaseFetcher double.
type FakeReleaseFetcher struct {
	LatestRelease app.ReleaseInfo
	LatestErr     error
	ByTagReleases map[string]app.ReleaseInfo
	ByTagErr      error
}

// NewFakeReleaseFetcher returns an empty FakeReleaseFetcher ready to use.
func NewFakeReleaseFetcher() *FakeReleaseFetcher {
	return &FakeReleaseFetcher{ByTagReleases: map[string]app.ReleaseInfo{}}
}

func (f *FakeReleaseFetcher) Latest() (app.ReleaseInfo, error) {
	return f.LatestRelease, f.LatestErr
}

func (f *FakeReleaseFetcher) ByTag(tag string) (app.ReleaseInfo, error) {
	if f.ByTagErr != nil {
		return app.ReleaseInfo{}, f.ByTagErr
	}
	r, ok := f.ByTagReleases[tag]
	if !ok {
		return app.ReleaseInfo{}, fmt.Errorf("no release tagged %q", tag)
	}
	return r, nil
}
