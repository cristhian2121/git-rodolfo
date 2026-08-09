# Homebrew formula template for git-rodolfo.
#
# This is a template, not a published formula: the url/sha256 pairs below
# are placeholders. To cut a real release:
#   1. Tag a version and push it (e.g. `git tag v0.1.0 && git push --tags`)
#      and let CI (or `make release`) build and publish the four
#      platform tarballs as GitHub release assets.
#   2. Replace VERSION and each REPLACE_WITH_SHA256_OF_* below with the
#      real tag and the checksums from that release's checksums.txt.
#   3. Publish this file in a tap repository (e.g. lean-tech/homebrew-tap
#      as Formula/git-rodolfo.rb) so `brew install lean-tech/tap/git-rodolfo`
#      works — Homebrew does not accept formulas from arbitrary repos.
class GitRodolfo < Formula
  desc "Manage multiple Git/GitHub identities on one computer"
  homepage "https://github.com/lean-tech/git-rodolfo"
  version "VERSION"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/lean-tech/git-rodolfo/releases/download/vVERSION/git-rodolfo-vVERSION-darwin-arm64.tar.gz"
      sha256 "REPLACE_WITH_SHA256_OF_darwin_arm64_TARBALL"
    else
      url "https://github.com/lean-tech/git-rodolfo/releases/download/vVERSION/git-rodolfo-vVERSION-darwin-amd64.tar.gz"
      sha256 "REPLACE_WITH_SHA256_OF_darwin_amd64_TARBALL"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/lean-tech/git-rodolfo/releases/download/vVERSION/git-rodolfo-vVERSION-linux-arm64.tar.gz"
      sha256 "REPLACE_WITH_SHA256_OF_linux_arm64_TARBALL"
    else
      url "https://github.com/lean-tech/git-rodolfo/releases/download/vVERSION/git-rodolfo-vVERSION-linux-amd64.tar.gz"
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
