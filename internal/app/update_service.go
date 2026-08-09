package app

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// checksumsFileName is the release asset produced by `make release`
// ("cd $(DIST) && shasum -a 256 *.tar.gz > checksums.txt") and expected to
// be published alongside the platform tarballs.
const checksumsFileName = "checksums.txt"

// ErrAlreadyUpToDate is returned when the requested release (latest, or a
// specific tag) is the version already running.
var ErrAlreadyUpToDate = errors.New("already up to date")

// ErrReleaseAssetNotFound is returned when a release has no tarball for
// the current OS/architecture.
var ErrReleaseAssetNotFound = errors.New("no release asset found for this platform")

// ErrChecksumsUnavailable is returned when a release doesn't publish
// checksums.txt, or publishes one with no entry for our platform's
// tarball — Update refuses to install a binary it can't verify rather
// than silently trusting an unverified download.
var ErrChecksumsUnavailable = errors.New("no checksum available to verify this release")

// ErrChecksumMismatch is returned when a downloaded tarball's SHA-256
// doesn't match checksums.txt — the download may be corrupted, or the
// release tampered with; either way Update refuses to install it.
var ErrChecksumMismatch = errors.New("checksum mismatch")

// UpdateResult is what a successful Update produces: the extracted
// git-rodolfo binary, ready for SelfUpdater to install, plus the version
// transition for the caller to report.
type UpdateResult struct {
	FromVersion string
	ToVersion   string
	BinaryData  []byte
}

// UpdateService implements "git rodolfo update" (§26): download a release
// tarball for this platform and hand back the extracted binary. It never
// touches disk itself — installing the result is SelfUpdater's job, kept
// separate so the CLI layer can ask for confirmation in between.
type UpdateService struct {
	fetcher        ReleaseFetcher
	downloader     AssetDownloader
	goos, goarch   string
	currentVersion string
}

// NewUpdateService builds an UpdateService. goos/goarch are normally
// runtime.GOOS/runtime.GOARCH; passed in explicitly so tests can target a
// platform without needing to run on it.
func NewUpdateService(fetcher ReleaseFetcher, downloader AssetDownloader, goos, goarch, currentVersion string) *UpdateService {
	return &UpdateService{fetcher: fetcher, downloader: downloader, goos: goos, goarch: goarch, currentVersion: currentVersion}
}

// Update resolves the requested release (the latest, if tag is empty, or
// exactly tag otherwise), downloads its tarball for this platform, and
// extracts the git-rodolfo binary from it.
func (s *UpdateService) Update(tag string) (UpdateResult, error) {
	var release ReleaseInfo
	var err error
	if tag == "" {
		release, err = s.fetcher.Latest()
	} else {
		release, err = s.fetcher.ByTag(tag)
	}
	if err != nil {
		return UpdateResult{}, err
	}

	if normalizeVersion(release.TagName) == normalizeVersion(s.currentVersion) {
		return UpdateResult{}, ErrAlreadyUpToDate
	}

	assetName := fmt.Sprintf("git-rodolfo-%s-%s-%s.tar.gz", release.TagName, s.goos, s.goarch)
	assetURL := findAssetURL(release.Assets, assetName)
	if assetURL == "" {
		return UpdateResult{}, fmt.Errorf("%w: release %s has no asset named %q", ErrReleaseAssetNotFound, release.TagName, assetName)
	}

	checksumsURL := findAssetURL(release.Assets, checksumsFileName)
	if checksumsURL == "" {
		return UpdateResult{}, fmt.Errorf("%w: release %s does not publish %s", ErrChecksumsUnavailable, release.TagName, checksumsFileName)
	}
	checksumsData, err := s.downloader.Download(checksumsURL)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("download %s: %w", checksumsFileName, err)
	}
	expectedHash, ok := parseChecksumsFile(checksumsData, assetName)
	if !ok {
		return UpdateResult{}, fmt.Errorf("%w: %s has no entry for %q", ErrChecksumsUnavailable, checksumsFileName, assetName)
	}

	data, err := s.downloader.Download(assetURL)
	if err != nil {
		return UpdateResult{}, err
	}

	gotHash := fmt.Sprintf("%x", sha256.Sum256(data))
	if gotHash != expectedHash {
		return UpdateResult{}, fmt.Errorf("%w: %s: expected %s, got %s", ErrChecksumMismatch, assetName, expectedHash, gotHash)
	}

	binary, err := extractBinaryFromTarGz(data)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("extract %s: %w", assetName, err)
	}

	return UpdateResult{FromVersion: s.currentVersion, ToVersion: release.TagName, BinaryData: binary}, nil
}

func findAssetURL(assets []ReleaseAsset, name string) string {
	for _, a := range assets {
		if a.Name == name {
			return a.DownloadURL
		}
	}
	return ""
}

// parseChecksumsFile parses the standard `shasum -a 256` output format
// ("<hex>  <filename>" per line) and returns the lowercase hex digest for
// name, if present. A "*" prefix on the filename (shasum's binary-mode
// marker) is stripped before matching.
func parseChecksumsFile(data []byte, name string) (hash string, ok bool) {
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		filename := strings.TrimPrefix(fields[1], "*")
		if filename == name {
			return strings.ToLower(fields[0]), true
		}
	}
	return "", false
}

// normalizeVersion strips a leading "v" so "v0.2.0" and "0.2.0" compare
// equal — release tags conventionally have the prefix, cli.Version's
// dev default doesn't.
func normalizeVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}

// extractBinaryFromTarGz reads a gzipped tarball and returns the contents
// of the entry named "git-rodolfo", however deep its path in the archive.
func extractBinaryFromTarGz(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("not a valid gzip archive: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar archive: %w", err)
		}
		if hdr.Typeflag == tar.TypeReg && filepath.Base(hdr.Name) == "git-rodolfo" {
			return io.ReadAll(tr)
		}
	}
	return nil, errors.New("archive did not contain a git-rodolfo binary")
}
