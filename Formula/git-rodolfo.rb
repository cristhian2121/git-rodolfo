# Homebrew formula template for git-rodolfo.
#
# This file is a template, not the published formula: the "VERSION" and
# "REPLACE_WITH_SHA256_OF_..." tokens below are placeholders, filled in
# automatically by scripts/publish-homebrew-tap.sh. Tagging a release
# (e.g. `git tag v0.3.0 && git push --tags`) triggers
# .github/workflows/release.yml, which runs `make release`, publishes the
# GitHub release, then runs that script to render this template with the
# real version and checksums and push it to the tap repository
# (cristhian2121/homebrew-tap) — no manual editing needed (RF-43).
class GitRodolfo < Formula
  desc "Manage multiple Git/GitHub identities on one computer"
  homepage "https://github.com/cristhian2121/git-rodolfo"
  version "VERSION"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/cristhian2121/git-rodolfo/releases/download/vVERSION/git-rodolfo-vVERSION-darwin-arm64.tar.gz"
      sha256 "REPLACE_WITH_SHA256_OF_darwin_arm64_TARBALL"
    else
      url "https://github.com/cristhian2121/git-rodolfo/releases/download/vVERSION/git-rodolfo-vVERSION-darwin-amd64.tar.gz"
      sha256 "REPLACE_WITH_SHA256_OF_darwin_amd64_TARBALL"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/cristhian2121/git-rodolfo/releases/download/vVERSION/git-rodolfo-vVERSION-linux-arm64.tar.gz"
      sha256 "REPLACE_WITH_SHA256_OF_linux_arm64_TARBALL"
    else
      url "https://github.com/cristhian2121/git-rodolfo/releases/download/vVERSION/git-rodolfo-vVERSION-linux-amd64.tar.gz"
      sha256 "REPLACE_WITH_SHA256_OF_linux_amd64_TARBALL"
    end
  end

  depends_on "git"

  def install
    bin.install "git-rodolfo"
  end

  test do
    assert_match "git-rodolfo version", shell_output("#{bin}/git-rodolfo --version")
  end
end
