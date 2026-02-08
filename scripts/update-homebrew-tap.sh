#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: $0 <version-tag> <tap-path>"
  echo "example: $0 v0.0.6 ../homebrew-tap"
  exit 1
fi

VERSION_TAG="$1"
TAP_PATH="$2"
VERSION="${VERSION_TAG#v}"

TMP_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

download() {
  local name="$1"
  local url="https://github.com/kplane-dev/kplane/releases/download/${VERSION_TAG}/kplane-${VERSION_TAG}-${name}"
  curl -fsSL -o "${TMP_DIR}/kplane-${name}" "${url}"
  shasum -a 256 "${TMP_DIR}/kplane-${name}" | awk '{print $1}'
}

SHA_DARWIN_AMD64="$(download darwin-amd64)"
SHA_DARWIN_ARM64="$(download darwin-arm64)"
SHA_LINUX_AMD64="$(download linux-amd64)"
SHA_LINUX_ARM64="$(download linux-arm64)"

mkdir -p "${TAP_PATH}/Formula"

cat > "${TAP_PATH}/Formula/kplane.rb" <<EOF
class Kplane < Formula
  desc "Local virtual control planes for Kubernetes"
  homepage "https://github.com/kplane-dev/kplane"
  version "${VERSION}"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/kplane-dev/kplane/releases/download/${VERSION_TAG}/kplane-${VERSION_TAG}-darwin-arm64"
      sha256 "${SHA_DARWIN_ARM64}"
    else
      url "https://github.com/kplane-dev/kplane/releases/download/${VERSION_TAG}/kplane-${VERSION_TAG}-darwin-amd64"
      sha256 "${SHA_DARWIN_AMD64}"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/kplane-dev/kplane/releases/download/${VERSION_TAG}/kplane-${VERSION_TAG}-linux-arm64"
      sha256 "${SHA_LINUX_ARM64}"
    else
      url "https://github.com/kplane-dev/kplane/releases/download/${VERSION_TAG}/kplane-${VERSION_TAG}-linux-amd64"
      sha256 "${SHA_LINUX_AMD64}"
    end
  end

  def install
    bin.install "kplane"
  end
end
EOF

echo "Updated ${TAP_PATH}/Formula/kplane.rb for ${VERSION_TAG}"
