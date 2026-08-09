package app_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/infra/fakes"
)

// buildTarGz builds an in-memory .tar.gz containing a single file, the
// same shape as a real release tarball (a lone "git-rodolfo" binary).
func buildTarGz(t *testing.T, filename string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: filename, Mode: 0o755, Size: int64(len(content))}); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar Close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip Close: %v", err)
	}
	return buf.Bytes()
}

// checksumsTxt builds a checksums.txt in the exact format `shasum -a 256`
// (and so `make release`) produces, with one correct entry for
// (assetName, assetData).
func checksumsTxt(assetName string, assetData []byte) []byte {
	return []byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(assetData), assetName))
}

func TestUpdateService_Latest_Success(t *testing.T) {
	fetcher := fakes.NewFakeReleaseFetcher()
	fetcher.LatestRelease = app.ReleaseInfo{
		TagName: "v0.2.0",
		Assets: []app.ReleaseAsset{
			{Name: "git-rodolfo-v0.2.0-darwin-arm64.tar.gz", DownloadURL: "https://example.com/asset.tar.gz"},
			{Name: "checksums.txt", DownloadURL: "https://example.com/checksums.txt"},
		},
	}
	tarball := buildTarGz(t, "git-rodolfo", []byte("new-binary-bytes"))
	downloader := fakes.NewFakeAssetDownloader()
	downloader.Assets["https://example.com/asset.tar.gz"] = tarball
	downloader.Assets["https://example.com/checksums.txt"] = checksumsTxt("git-rodolfo-v0.2.0-darwin-arm64.tar.gz", tarball)

	svc := app.NewUpdateService(fetcher, downloader, "darwin", "arm64", "v0.1.0")
	result, err := svc.Update("")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if result.FromVersion != "v0.1.0" || result.ToVersion != "v0.2.0" {
		t.Fatalf("unexpected versions: %+v", result)
	}
	if string(result.BinaryData) != "new-binary-bytes" {
		t.Fatalf("unexpected binary data: %q", result.BinaryData)
	}
}

func TestUpdateService_ByTag(t *testing.T) {
	fetcher := fakes.NewFakeReleaseFetcher()
	fetcher.ByTagReleases["v0.3.0"] = app.ReleaseInfo{
		TagName: "v0.3.0",
		Assets: []app.ReleaseAsset{
			{Name: "git-rodolfo-v0.3.0-linux-amd64.tar.gz", DownloadURL: "https://example.com/v3.tar.gz"},
			{Name: "checksums.txt", DownloadURL: "https://example.com/v3-checksums.txt"},
		},
	}
	tarball := buildTarGz(t, "git-rodolfo", []byte("v3-bytes"))
	downloader := fakes.NewFakeAssetDownloader()
	downloader.Assets["https://example.com/v3.tar.gz"] = tarball
	downloader.Assets["https://example.com/v3-checksums.txt"] = checksumsTxt("git-rodolfo-v0.3.0-linux-amd64.tar.gz", tarball)

	svc := app.NewUpdateService(fetcher, downloader, "linux", "amd64", "v0.1.0")
	result, err := svc.Update("v0.3.0")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if result.ToVersion != "v0.3.0" {
		t.Fatalf("ToVersion = %q", result.ToVersion)
	}
}

func TestUpdateService_AlreadyUpToDate(t *testing.T) {
	fetcher := fakes.NewFakeReleaseFetcher()
	fetcher.LatestRelease = app.ReleaseInfo{TagName: "v0.1.0"}
	svc := app.NewUpdateService(fetcher, fakes.NewFakeAssetDownloader(), "darwin", "arm64", "0.1.0") // no "v" prefix on our side

	_, err := svc.Update("")
	if !errors.Is(err, app.ErrAlreadyUpToDate) {
		t.Fatalf("expected ErrAlreadyUpToDate (v-prefix normalized), got %v", err)
	}
}

func TestUpdateService_AssetNotFoundForPlatform(t *testing.T) {
	fetcher := fakes.NewFakeReleaseFetcher()
	fetcher.LatestRelease = app.ReleaseInfo{
		TagName: "v0.2.0",
		Assets:  []app.ReleaseAsset{{Name: "git-rodolfo-v0.2.0-linux-amd64.tar.gz", DownloadURL: "https://example.com/linux.tar.gz"}},
	}
	svc := app.NewUpdateService(fetcher, fakes.NewFakeAssetDownloader(), "darwin", "arm64", "v0.1.0")

	_, err := svc.Update("")
	if !errors.Is(err, app.ErrReleaseAssetNotFound) {
		t.Fatalf("expected ErrReleaseAssetNotFound, got %v", err)
	}
}

// TestUpdateService_ChecksumsFileMissingFromRelease is the "fail closed"
// case: a release with the right platform tarball but no checksums.txt at
// all must be refused, not installed unverified.
func TestUpdateService_ChecksumsFileMissingFromRelease(t *testing.T) {
	fetcher := fakes.NewFakeReleaseFetcher()
	fetcher.LatestRelease = app.ReleaseInfo{
		TagName: "v0.2.0",
		Assets:  []app.ReleaseAsset{{Name: "git-rodolfo-v0.2.0-darwin-arm64.tar.gz", DownloadURL: "https://example.com/asset.tar.gz"}},
	}
	downloader := fakes.NewFakeAssetDownloader()
	downloader.Assets["https://example.com/asset.tar.gz"] = buildTarGz(t, "git-rodolfo", []byte("bytes"))

	svc := app.NewUpdateService(fetcher, downloader, "darwin", "arm64", "v0.1.0")
	_, err := svc.Update("")
	if !errors.Is(err, app.ErrChecksumsUnavailable) {
		t.Fatalf("expected ErrChecksumsUnavailable, got %v", err)
	}
}

// TestUpdateService_ChecksumsFileMissingEntryForAsset covers checksums.txt
// existing but not listing our specific platform's asset.
func TestUpdateService_ChecksumsFileMissingEntryForAsset(t *testing.T) {
	fetcher := fakes.NewFakeReleaseFetcher()
	fetcher.LatestRelease = app.ReleaseInfo{
		TagName: "v0.2.0",
		Assets: []app.ReleaseAsset{
			{Name: "git-rodolfo-v0.2.0-darwin-arm64.tar.gz", DownloadURL: "https://example.com/asset.tar.gz"},
			{Name: "checksums.txt", DownloadURL: "https://example.com/checksums.txt"},
		},
	}
	downloader := fakes.NewFakeAssetDownloader()
	downloader.Assets["https://example.com/asset.tar.gz"] = buildTarGz(t, "git-rodolfo", []byte("bytes"))
	downloader.Assets["https://example.com/checksums.txt"] = checksumsTxt("git-rodolfo-v0.2.0-linux-amd64.tar.gz", []byte("unrelated"))

	svc := app.NewUpdateService(fetcher, downloader, "darwin", "arm64", "v0.1.0")
	_, err := svc.Update("")
	if !errors.Is(err, app.ErrChecksumsUnavailable) {
		t.Fatalf("expected ErrChecksumsUnavailable, got %v", err)
	}
}

// TestUpdateService_ChecksumMismatch_Rejected is the core security
// property: a downloaded tarball whose hash doesn't match checksums.txt
// must never be installed.
func TestUpdateService_ChecksumMismatch_Rejected(t *testing.T) {
	fetcher := fakes.NewFakeReleaseFetcher()
	fetcher.LatestRelease = app.ReleaseInfo{
		TagName: "v0.2.0",
		Assets: []app.ReleaseAsset{
			{Name: "git-rodolfo-v0.2.0-darwin-arm64.tar.gz", DownloadURL: "https://example.com/asset.tar.gz"},
			{Name: "checksums.txt", DownloadURL: "https://example.com/checksums.txt"},
		},
	}
	downloader := fakes.NewFakeAssetDownloader()
	downloader.Assets["https://example.com/asset.tar.gz"] = buildTarGz(t, "git-rodolfo", []byte("actual-bytes"))
	// checksums.txt claims a hash for different content than what's
	// actually served — simulating corruption or tampering.
	downloader.Assets["https://example.com/checksums.txt"] = checksumsTxt("git-rodolfo-v0.2.0-darwin-arm64.tar.gz", []byte("different-bytes"))

	svc := app.NewUpdateService(fetcher, downloader, "darwin", "arm64", "v0.1.0")
	result, err := svc.Update("")
	if !errors.Is(err, app.ErrChecksumMismatch) {
		t.Fatalf("expected ErrChecksumMismatch, got %v", err)
	}
	if result.BinaryData != nil {
		t.Fatal("no binary data must be returned on a checksum mismatch")
	}
}

func TestUpdateService_MalformedArchive(t *testing.T) {
	fetcher := fakes.NewFakeReleaseFetcher()
	fetcher.LatestRelease = app.ReleaseInfo{
		TagName: "v0.2.0",
		Assets: []app.ReleaseAsset{
			{Name: "git-rodolfo-v0.2.0-darwin-arm64.tar.gz", DownloadURL: "https://example.com/bad.tar.gz"},
			{Name: "checksums.txt", DownloadURL: "https://example.com/checksums.txt"},
		},
	}
	badData := []byte("not a real gzip archive")
	downloader := fakes.NewFakeAssetDownloader()
	downloader.Assets["https://example.com/bad.tar.gz"] = badData
	// The checksum must match so the test actually exercises extraction
	// failure, not a checksum rejection.
	downloader.Assets["https://example.com/checksums.txt"] = checksumsTxt("git-rodolfo-v0.2.0-darwin-arm64.tar.gz", badData)

	svc := app.NewUpdateService(fetcher, downloader, "darwin", "arm64", "v0.1.0")
	if _, err := svc.Update(""); err == nil {
		t.Fatal("expected an error extracting a malformed archive")
	}
}

func TestUpdateService_ArchiveMissingBinary(t *testing.T) {
	fetcher := fakes.NewFakeReleaseFetcher()
	fetcher.LatestRelease = app.ReleaseInfo{
		TagName: "v0.2.0",
		Assets: []app.ReleaseAsset{
			{Name: "git-rodolfo-v0.2.0-darwin-arm64.tar.gz", DownloadURL: "https://example.com/wrong.tar.gz"},
			{Name: "checksums.txt", DownloadURL: "https://example.com/checksums.txt"},
		},
	}
	wrongTarball := buildTarGz(t, "README.md", []byte("not the binary"))
	downloader := fakes.NewFakeAssetDownloader()
	downloader.Assets["https://example.com/wrong.tar.gz"] = wrongTarball
	downloader.Assets["https://example.com/checksums.txt"] = checksumsTxt("git-rodolfo-v0.2.0-darwin-arm64.tar.gz", wrongTarball)

	svc := app.NewUpdateService(fetcher, downloader, "darwin", "arm64", "v0.1.0")
	if _, err := svc.Update(""); err == nil {
		t.Fatal("expected an error when the archive has no git-rodolfo entry")
	}
}

func TestUpdateService_FetchError(t *testing.T) {
	fetcher := fakes.NewFakeReleaseFetcher()
	fetcher.LatestErr = errors.New("network is down")
	svc := app.NewUpdateService(fetcher, fakes.NewFakeAssetDownloader(), "darwin", "arm64", "v0.1.0")

	if _, err := svc.Update(""); err == nil {
		t.Fatal("expected the fetch error to propagate")
	}
}

func TestUpdateService_DownloadError(t *testing.T) {
	fetcher := fakes.NewFakeReleaseFetcher()
	fetcher.LatestRelease = app.ReleaseInfo{
		TagName: "v0.2.0",
		Assets: []app.ReleaseAsset{
			{Name: "git-rodolfo-v0.2.0-darwin-arm64.tar.gz", DownloadURL: "https://example.com/x.tar.gz"},
			{Name: "checksums.txt", DownloadURL: "https://example.com/checksums.txt"},
		},
	}
	downloader := fakes.NewFakeAssetDownloader()
	downloader.Err = errors.New("connection reset")

	svc := app.NewUpdateService(fetcher, downloader, "darwin", "arm64", "v0.1.0")
	if _, err := svc.Update(""); err == nil {
		t.Fatal("expected the download error to propagate")
	}
}
