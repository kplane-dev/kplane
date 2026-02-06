#!/usr/bin/env sh
set -eu

OWNER="kplane-dev"
REPO="kplane"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "${OS}" in
  darwin) OS="darwin" ;;
  linux) OS="linux" ;;
  *)
    echo "unsupported OS: ${OS}"
    exit 1
    ;;
esac

case "${ARCH}" in
  arm64|aarch64) ARCH="arm64" ;;
  amd64|x86_64) ARCH="amd64" ;;
  *)
    echo "unsupported architecture: ${ARCH}"
    exit 1
    ;;
esac

BIN_NAME="kplane-${OS}-${ARCH}"
BASE_URL="https://github.com/${OWNER}/${REPO}/releases/latest/download"
URL="${BASE_URL}/${BIN_NAME}"

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
TARGET="${INSTALL_DIR}/kplane"

if [ ! -d "${INSTALL_DIR}" ]; then
  echo "install dir does not exist: ${INSTALL_DIR}"
  exit 1
fi

TMP="$(mktemp -t kplane.XXXXXX)"
trap 'rm -f "${TMP}"' EXIT

echo "downloading ${URL}"
curl -fsSL "${URL}" -o "${TMP}"
chmod +x "${TMP}"

if [ ! -w "${INSTALL_DIR}" ]; then
  echo "installing to ${TARGET} (sudo required)"
  sudo mv "${TMP}" "${TARGET}"
else
  mv "${TMP}" "${TARGET}"
fi

if [ "${OS}" = "darwin" ] && command -v xattr >/dev/null 2>&1; then
  xattr -d com.apple.quarantine "${TARGET}" 2>/dev/null || true
fi

add_alias() {
  rc_file="$1"
  if [ -f "${rc_file}" ] && ! grep -q '^alias kp="kplane"$' "${rc_file}"; then
    printf '\n# kplane\nalias kp="kplane"\n' >> "${rc_file}"
    echo "added alias to ${rc_file}"
    echo "reload your shell to use kp (e.g. 'source ${rc_file}')"
  fi
}

if [ -n "${SHELL:-}" ]; then
  case "${SHELL}" in
    */zsh) add_alias "${HOME}/.zshrc" ;;
    */bash) add_alias "${HOME}/.bashrc" ;;
  esac
fi

echo "installed ${TARGET}"
