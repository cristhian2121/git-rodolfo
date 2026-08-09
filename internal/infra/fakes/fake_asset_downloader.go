package fakes

import "fmt"

// FakeAssetDownloader is an in-memory app.AssetDownloader double, keyed by
// the exact download URL a FakeReleaseFetcher reported.
type FakeAssetDownloader struct {
	Assets map[string][]byte
	Err    error
}

// NewFakeAssetDownloader returns an empty FakeAssetDownloader ready to use.
func NewFakeAssetDownloader() *FakeAssetDownloader {
	return &FakeAssetDownloader{Assets: map[string][]byte{}}
}

func (f *FakeAssetDownloader) Download(url string) ([]byte, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	data, ok := f.Assets[url]
	if !ok {
		return nil, fmt.Errorf("no fake asset registered for %s", url)
	}
	return data, nil
}
