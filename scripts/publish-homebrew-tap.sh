#!/usr/bin/env bash
# Renders Formula/git-rodolfo.rb for a real release and publishes it to the
# Homebrew tap (RF-43), so `brew install cristhian2121/tap/git-rodolfo`
# installs the version this script was run for.
#
# Usage (as run by .github/workflows/release.yml):
#   VERSION=v0.3.0 ./scripts/publish-homebrew-tap.sh
#
# Required env:
#   VERSION              Release tag, e.g. "v0.3.0" (with or without the
#                         leading "v" — normalized below).
#   HOMEBREW_TAP_TOKEN    PAT with push access to TAP_REPO. Not needed if
#                         TAP_CLONE_URL is set instead (local testing).
#
# Optional env:
#   CHECKSUMS_FILE   Default: dist/checksums.txt (from `make release`)
#   FORMULA_TEMPLATE Default: Formula/git-rodolfo.rb (this repo's template)
#   TAP_REPO         Default: cristhian2121/homebrew-tap
#   TAP_CLONE_URL    Overrides the URL/path git clones the tap from —
#                     lets this script be dry-run against a local bare repo
#                     instead of the real tap (see docs/TESTPLAN.md).
set -euo pipefail

rendered=""
workdir=""
# A plain `trap 'rm -f "$rendered"' EXIT` would look right but isn't: once
# set, the trap's own last command (rm, which succeeds) becomes the
# process's exit status, silently turning every `exit 1` above it into a
# 0 — exactly the failure mode a release pipeline must not have. Capturing
# $? first and re-raising it explicitly is what keeps a real error real.
cleanup() {
  local status=$?
  [ -n "$rendered" ] && rm -f "$rendered"
  [ -n "$workdir" ] && rm -rf "$workdir"
  exit "$status"
}
trap cleanup EXIT

if [ -z "${VERSION:-}" ]; then
  echo "publish-homebrew-tap: VERSION is required, e.g. VERSION=v0.3.0" >&2
  exit 1
fi
CHECKSUMS_FILE="${CHECKSUMS_FILE:-dist/checksums.txt}"
FORMULA_TEMPLATE="${FORMULA_TEMPLATE:-Formula/git-rodolfo.rb}"
TAP_REPO="${TAP_REPO:-cristhian2121/homebrew-tap}"

tag="$VERSION"
version_no_v="${tag#v}"

if [ ! -f "$CHECKSUMS_FILE" ]; then
  echo "publish-homebrew-tap: checksums file not found: $CHECKSUMS_FILE (run 'make release' first)" >&2
  exit 1
fi
if [ ! -f "$FORMULA_TEMPLATE" ]; then
  echo "publish-homebrew-tap: formula template not found: $FORMULA_TEMPLATE" >&2
  exit 1
fi

sha_for() {
  local os="$1" arch="$2"
  local line
  line="$(grep -F -e "-${os}-${arch}.tar.gz" "$CHECKSUMS_FILE" || true)"
  if [ -z "$line" ]; then
    echo "publish-homebrew-tap: no checksum for ${os}/${arch} in $CHECKSUMS_FILE" >&2
    exit 1
  fi
  awk '{print $1}' <<<"$line"
}

darwin_amd64_sha="$(sha_for darwin amd64)"
darwin_arm64_sha="$(sha_for darwin arm64)"
linux_amd64_sha="$(sha_for linux amd64)"
linux_arm64_sha="$(sha_for linux arm64)"

rendered="$(mktemp)"

# Drop this repo's own explanatory header comment first: the tap only
# needs the formula itself, and it also means the substitutions below
# can't mangle a comment that happens to mention "VERSION" in prose.
sed -n '/^class /,$p' "$FORMULA_TEMPLATE" | sed \
  -e "s/\"VERSION\"/\"${version_no_v}\"/g" \
  -e "s/vVERSION/v${version_no_v}/g" \
  -e "s/REPLACE_WITH_SHA256_OF_darwin_amd64_TARBALL/${darwin_amd64_sha}/g" \
  -e "s/REPLACE_WITH_SHA256_OF_darwin_arm64_TARBALL/${darwin_arm64_sha}/g" \
  -e "s/REPLACE_WITH_SHA256_OF_linux_amd64_TARBALL/${linux_amd64_sha}/g" \
  -e "s/REPLACE_WITH_SHA256_OF_linux_arm64_TARBALL/${linux_arm64_sha}/g" \
  >"$rendered"

if grep -q 'REPLACE_WITH_SHA256\|"VERSION"\|vVERSION' "$rendered"; then
  echo "publish-homebrew-tap: rendered formula still has unfilled placeholders:" >&2
  grep -n 'REPLACE_WITH_SHA256\|"VERSION"\|vVERSION' "$rendered" >&2
  exit 1
fi

clone_url="${TAP_CLONE_URL:-}"
if [ -z "$clone_url" ]; then
  if [ -z "${HOMEBREW_TAP_TOKEN:-}" ]; then
    echo "publish-homebrew-tap: HOMEBREW_TAP_TOKEN is required unless TAP_CLONE_URL is set" >&2
    exit 1
  fi
  clone_url="https://x-access-token:${HOMEBREW_TAP_TOKEN}@github.com/${TAP_REPO}.git"
fi

workdir="$(mktemp -d)"

git clone --quiet --depth 1 "$clone_url" "$workdir"
mkdir -p "$workdir/Formula"
cp "$rendered" "$workdir/Formula/git-rodolfo.rb"

git -C "$workdir" -c user.name="git-rodolfo release bot" -c user.email="actions@users.noreply.github.com" add Formula/git-rodolfo.rb

if git -C "$workdir" diff --cached --quiet; then
  echo "publish-homebrew-tap: formula already up to date for $tag, nothing to push."
  exit 0
fi

git -C "$workdir" -c user.name="git-rodolfo release bot" -c user.email="actions@users.noreply.github.com" commit --quiet -m "git-rodolfo ${tag}"
git -C "$workdir" push --quiet

echo "✓ Published Formula/git-rodolfo.rb for ${tag} to ${TAP_REPO}"
