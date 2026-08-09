#!/usr/bin/env bash
# Installs git-rodolfo by downloading the latest release tarball for the
# current OS/architecture and placing the binary on PATH.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/cristhian2121/git-rodolfo/main/scripts/install.sh | bash
#
# Requires: curl, tar. Set GIT_RODOLFO_REPO to point at a different
# owner/repo (e.g. a fork), and GIT_RODOLFO_INSTALL_DIR to install
# somewhere other than /usr/local/bin.
set -euo pipefail

REPO="${GIT_RODOLFO_REPO:-cristhian2121/git-rodolfo}"
INSTALL_DIR="${GIT_RODOLFO_INSTALL_DIR:-/usr/local/bin}"

os="$(uname -s)"
arch="$(uname -m)"

case "$os" in
  Darwin) goos="darwin" ;;
  Linux)  goos="linux" ;;
  *)
    echo "git-rodolfo: unsupported OS: $os (only macOS and Linux are supported)" >&2
    exit 1
    ;;
esac

case "$arch" in
  x86_64|amd64) goarch="amd64" ;;
  arm64|aarch64) goarch="arm64" ;;
  *)
    echo "git-rodolfo: unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

version="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep -m1 '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')"
if [ -z "$version" ]; then
  echo "git-rodolfo: could not determine the latest release version from https://github.com/${REPO}/releases" >&2
  exit 1
fi

tarball="git-rodolfo-${version}-${goos}-${goarch}.tar.gz"
url="https://github.com/${REPO}/releases/download/${version}/${tarball}"

echo "Downloading ${url}..."
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
curl -fsSL "$url" -o "$tmp/$tarball"
tar -xzf "$tmp/$tarball" -C "$tmp"

if [ ! -w "$INSTALL_DIR" ]; then
  echo "Installing to $INSTALL_DIR (requires sudo)..."
  sudo install -m 0755 "$tmp/git-rodolfo" "$INSTALL_DIR/git-rodolfo"
else
  install -m 0755 "$tmp/git-rodolfo" "$INSTALL_DIR/git-rodolfo"
fi

echo "✓ Installed git-rodolfo $version to $INSTALL_DIR/git-rodolfo"
echo
if ! command -v git-rodolfo >/dev/null 2>&1; then
  echo "Note: $INSTALL_DIR does not appear to be on your PATH."
  echo "Add it to your shell profile, e.g.:"
  echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
fi
echo "Run \"git-rodolfo help\" or \"git rodolfo help\" to get started."
