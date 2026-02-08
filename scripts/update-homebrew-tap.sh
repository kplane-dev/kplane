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
  local versioned="https://github.com/kplane-dev/kplane/releases/download/${VERSION_TAG}/kplane-${VERSION_TAG}-${name}"
  local unversioned="https://github.com/kplane-dev/kplane/releases/download/${VERSION_TAG}/kplane-${name}"
  local out="${TMP_DIR}/kplane-${name}"
  local attempt

  for url in "${versioned}" "${unversioned}"; do
    for attempt in {1..10}; do
      if curl -fsSL -o "${out}" "${url}"; then
        shasum -a 256 "${out}" | awk '{print $1}'
        return 0
      fi
      sleep 3
    done
  done

  echo "failed to download ${name} from ${VERSION_TAG} after retries" >&2
  return 1
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
    if OS.mac?
      if Hardware::CPU.arm?
        bin.install "kplane-v#{version}-darwin-arm64" => "kplane"
      else
        bin.install "kplane-v#{version}-darwin-amd64" => "kplane"
      end
    elsif OS.linux?
      if Hardware::CPU.arm?
        bin.install "kplane-v#{version}-linux-arm64" => "kplane"
      else
        bin.install "kplane-v#{version}-linux-amd64" => "kplane"
      end
    end
  end
end
EOF

echo "Updated ${TAP_PATH}/Formula/kplane.rb for ${VERSION_TAG}"
