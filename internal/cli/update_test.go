package cli_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/cli"
	"github.com/lean-tech/git-rodolfo/internal/infra/fakes"
)

// buildFakeReleaseTarGz mirrors a real release tarball's shape: a single
// "git-rodolfo" file at its root.
func buildFakeReleaseTarGz(t *testing.T, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "git-rodolfo", Mode: 0o755, Size: int64(len(content))}); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

// registerFakeRelease wires a release's platform tarball and its matching
// checksums.txt into fetcher/downloader, so tests don't have to repeat
// update's now-mandatory checksum-verification setup by hand.
func registerFakeRelease(fetcher *fakes.FakeReleaseFetcher, downloader *fakes.FakeAssetDownloader, byTag bool, tag, assetName, tarballURL, checksumsURL string, tarball []byte) {
	release := app.ReleaseInfo{
		TagName: tag,
		Assets: []app.ReleaseAsset{
			{Name: assetName, DownloadURL: tarballURL},
			{Name: "checksums.txt", DownloadURL: checksumsURL},
		},
	}
	if byTag {
		fetcher.ByTagReleases[tag] = release
	} else {
		fetcher.LatestRelease = release
	}
	downloader.Assets[tarballURL] = tarball
	downloader.Assets[checksumsURL] = []byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(tarball), assetName))
}

func updateDeps(t *testing.T, stdin *os.File, fetcher *fakes.FakeReleaseFetcher, downloader *fakes.FakeAssetDownloader, updater *fakes.FakeSelfUpdater, currentVersion string) (cli.Deps, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	return cli.Deps{
		Update:      app.NewUpdateService(fetcher, downloader, "darwin", "arm64", currentVersion),
		SelfUpdater: updater,
		Stdout:      &stdout,
		Stderr:      &stderr,
		Stdin:       stdin,
		Now:         fixedNow,
	}, &stdout, &stderr
}

func TestRun_Update_NonInteractive_Latest(t *testing.T) {
	fetcher := fakes.NewFakeReleaseFetcher()
	downloader := fakes.NewFakeAssetDownloader()
	registerFakeRelease(fetcher, downloader, false, "v0.2.0", "git-rodolfo-v0.2.0-darwin-arm64.tar.gz",
		"https://example.com/a.tar.gz", "https://example.com/a-checksums.txt", buildFakeReleaseTarGz(t, "new-binary"))
	updater := &fakes.FakeSelfUpdater{}
	deps, stdout, stderr := updateDeps(t, nil, fetcher, downloader, updater, "v0.1.0")

	code := cli.Run([]string{"update", "--non-interactive", "--yes"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "v0.1.0 -> v0.2.0 available.") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "✓ Updated to v0.2.0.") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
	if len(updater.Replaced) != 1 || string(updater.Replaced[0]) != "new-binary" {
		t.Fatalf("expected the binary to be replaced, got %+v", updater.Replaced)
	}
}

func TestRun_Update_NonInteractive_RequiresYes(t *testing.T) {
	fetcher := fakes.NewFakeReleaseFetcher()
	downloader := fakes.NewFakeAssetDownloader()
	registerFakeRelease(fetcher, downloader, false, "v0.2.0", "git-rodolfo-v0.2.0-darwin-arm64.tar.gz",
		"https://example.com/a.tar.gz", "https://example.com/a-checksums.txt", buildFakeReleaseTarGz(t, "new-binary"))
	updater := &fakes.FakeSelfUpdater{}
	deps, _, stderr := updateDeps(t, nil, fetcher, downloader, updater, "v0.1.0")

	code := cli.Run([]string{"update", "--non-interactive"}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "requires --yes") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
	if len(updater.Replaced) != 0 {
		t.Fatal("must not replace the binary without confirmation")
	}
}

func TestRun_Update_AlreadyUpToDate(t *testing.T) {
	fetcher := fakes.NewFakeReleaseFetcher()
	fetcher.LatestRelease = app.ReleaseInfo{TagName: "v0.1.0"}
	updater := &fakes.FakeSelfUpdater{}
	deps, stdout, stderr := updateDeps(t, nil, fetcher, fakes.NewFakeAssetDownloader(), updater, "v0.1.0")

	code := cli.Run([]string{"update", "--non-interactive", "--yes"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Already up to date") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
	if len(updater.Replaced) != 0 {
		t.Fatal("must not replace the binary when already up to date")
	}
}

func TestRun_Update_SpecificVersion(t *testing.T) {
	fetcher := fakes.NewFakeReleaseFetcher()
	downloader := fakes.NewFakeAssetDownloader()
	registerFakeRelease(fetcher, downloader, true, "v0.3.0", "git-rodolfo-v0.3.0-darwin-arm64.tar.gz",
		"https://example.com/v3.tar.gz", "https://example.com/v3-checksums.txt", buildFakeReleaseTarGz(t, "v3-binary"))
	updater := &fakes.FakeSelfUpdater{}
	deps, stdout, stderr := updateDeps(t, nil, fetcher, downloader, updater, "v0.1.0")

	code := cli.Run([]string{"update", "--version", "v0.3.0", "--non-interactive", "--yes"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "v0.1.0 -> v0.3.0 available.") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
}

func TestRun_Update_AssetNotFoundForPlatform(t *testing.T) {
	fetcher := fakes.NewFakeReleaseFetcher()
	fetcher.LatestRelease = app.ReleaseInfo{
		TagName: "v0.2.0",
		Assets:  []app.ReleaseAsset{{Name: "git-rodolfo-v0.2.0-linux-amd64.tar.gz", DownloadURL: "https://example.com/linux.tar.gz"}},
	}
	deps, _, stderr := updateDeps(t, nil, fetcher, fakes.NewFakeAssetDownloader(), &fakes.FakeSelfUpdater{}, "v0.1.0")

	code := cli.Run([]string{"update", "--non-interactive", "--yes"}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "no release asset found for this platform") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

// TestRun_Update_ChecksumMismatch_RefusesToInstall confirms the CLI
// surfaces a checksum mismatch as a clear error and never calls
// SelfUpdater — the security property "update" exists to enforce.
func TestRun_Update_ChecksumMismatch_RefusesToInstall(t *testing.T) {
	fetcher := fakes.NewFakeReleaseFetcher()
	fetcher.LatestRelease = app.ReleaseInfo{
		TagName: "v0.2.0",
		Assets: []app.ReleaseAsset{
			{Name: "git-rodolfo-v0.2.0-darwin-arm64.tar.gz", DownloadURL: "https://example.com/a.tar.gz"},
			{Name: "checksums.txt", DownloadURL: "https://example.com/checksums.txt"},
		},
	}
	downloader := fakes.NewFakeAssetDownloader()
	downloader.Assets["https://example.com/a.tar.gz"] = buildFakeReleaseTarGz(t, "new-binary")
	downloader.Assets["https://example.com/checksums.txt"] = []byte(
		"0000000000000000000000000000000000000000000000000000000000000000  git-rodolfo-v0.2.0-darwin-arm64.tar.gz\n")
	updater := &fakes.FakeSelfUpdater{}
	deps, _, stderr := updateDeps(t, nil, fetcher, downloader, updater, "v0.1.0")

	code := cli.Run([]string{"update", "--non-interactive", "--yes"}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "checksum mismatch") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
	if len(updater.Replaced) != 0 {
		t.Fatal("must never install a binary that failed checksum verification")
	}
}

// TestRun_Update_Interactive_Confirm drives the confirmation prompt
// through a pipe (never a TTY, so Confirm uses its fallback).
func TestRun_Update_Interactive_Confirm(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	fetcher := fakes.NewFakeReleaseFetcher()
	downloader := fakes.NewFakeAssetDownloader()
	registerFakeRelease(fetcher, downloader, false, "v0.2.0", "git-rodolfo-v0.2.0-darwin-arm64.tar.gz",
		"https://example.com/a.tar.gz", "https://example.com/a-checksums.txt", buildFakeReleaseTarGz(t, "new-binary"))
	updater := &fakes.FakeSelfUpdater{}
	deps, stdout, stderr := updateDeps(t, r, fetcher, downloader, updater, "v0.1.0")

	go func() {
		defer w.Close()
		w.WriteString("y\n")
	}()

	code := cli.Run([]string{"update"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s, stdout = %s", code, stderr.String(), stdout.String())
	}
	if len(updater.Replaced) != 1 {
		t.Fatal("expected the binary to be replaced after confirming")
	}
}

func TestRun_Update_Interactive_Decline(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	fetcher := fakes.NewFakeReleaseFetcher()
	downloader := fakes.NewFakeAssetDownloader()
	registerFakeRelease(fetcher, downloader, false, "v0.2.0", "git-rodolfo-v0.2.0-darwin-arm64.tar.gz",
		"https://example.com/a.tar.gz", "https://example.com/a-checksums.txt", buildFakeReleaseTarGz(t, "new-binary"))
	updater := &fakes.FakeSelfUpdater{}
	deps, stdout, _ := updateDeps(t, r, fetcher, downloader, updater, "v0.1.0")

	go func() {
		defer w.Close()
		w.WriteString("n\n")
	}()

	code := cli.Run([]string{"update"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0, stdout = %s", code, stdout.String())
	}
	if len(updater.Replaced) != 0 {
		t.Fatal("declining must not replace the binary")
	}
}
