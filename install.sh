#!/usr/bin/env bash
# install.sh — curl|sh installer for stagehand (PRD §21.3).
#
# Detects OS/arch, fetches the matching archive + checksums.txt from the
# latest GitHub Release, verifies the SHA-256, extracts the `stagehand` binary,
# and installs it to $INSTALL_DIR (default /usr/local/bin, sudo'd if needed,
# or $HOME/.local/bin when /usr/local/bin is not user-writable).
#
#   curl -fsSL https://github.com/dustin/stagehand/releases/latest/download/install.sh | bash
#
# Override the destination:   INSTALL_DIR=~/bin bash install.sh
# Pin a version (tag):        VERSION=v1.0.0 bash install.sh
# Use a fork:                 OWNER=myorg REPO=stagehand bash install.sh
set -euo pipefail

# --- configuration (overridable via env) -----------------------------------
OWNER="${OWNER:-dustin}"
REPO="${REPO:-stagehand}"
BIN_NAME="${BIN_NAME:-stagehand}"
VERSION="${VERSION:-}"   # empty ⇒ resolve latest from GitHub
if [[ -n "${INSTALL_DIR:-}" ]]; then
  : # respect explicit override
elif [[ -w "/usr/local/bin" ]]; then
  INSTALL_DIR="/usr/local/bin"
else
  INSTALL_DIR="${HOME}/.local/bin"
fi

# --- helpers ---------------------------------------------------------------
info()  { printf '\033[1;34m==>\033[0m %s\n' "$*" >&2; }
fatal() { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }
need_cmd() {
  command -v "$1" >/dev/null 2>&1 || fatal "'$1' is required but not found in PATH."
}

# --- dependency checks -----------------------------------------------------
need_cmd curl
need_cmd tar
# shasum/sha256sum checked after checksum download (one of them is required)

# --- detect OS / arch (goreleaser matrix: linux/darwin/windows × amd64/arm64)
case "$(uname -s)" in
  Linux*)  OS="linux" ;;
  Darwin*) OS="macos" ;;   # archive name_template maps darwin→macos
  MINGW*|MSYS*|CYGWIN*) OS="windows" ;;
  *) fatal "unsupported OS: $(uname -s). On Windows use: scoop install ${OWNER}/${REPO}" ;;
esac
case "$(uname -m)" in
  x86_64|amd64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) fatal "unsupported architecture: $(uname -m)" ;;
esac

# --- resolve latest version tag from GitHub --------------------------------
if [[ -z "${VERSION}" ]]; then
  info "checking latest ${OWNER}/${REPO} release..."
  VERSION="$(curl -fsSL "https://api.github.com/repos/${OWNER}/${REPO}/releases/latest" \
    | grep '"tag_name"' | head -n1 | sed -E 's/.*"([^"]+)".*/\1/')" \
    || fatal "could not determine latest release of ${OWNER}/${REPO}"
  [[ -n "${VERSION}" ]] || fatal "latest release tag was empty"
fi

# goreleaser download path keeps the tag verbatim; the archive filename uses
# the version WITHOUT the leading 'v' ({{.Version}} strips it).
DOWNLOAD_TAG="${VERSION}"
VERSION_NUM="${VERSION#v}"
ARCHIVE="${REPO}_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${OWNER}/${REPO}/releases/download/${DOWNLOAD_TAG}/${ARCHIVE}"
CHECKSUM_URL="https://github.com/${OWNER}/${REPO}/releases/download/${DOWNLOAD_TAG}/checksums.txt"

info "installing ${BIN_NAME} ${VERSION} for ${OS}/${ARCH} into ${INSTALL_DIR}"

# --- working area ----------------------------------------------------------
TMPDIR="$(mktemp -d)"
trap 'rm -rf "${TMPDIR}"' EXIT

# --- download archive + checksums ------------------------------------------
info "downloading ${ARCHIVE}..."
curl -fsSL -o "${TMPDIR}/${ARCHIVE}" "${DOWNLOAD_URL}" \
  || fatal "download failed: ${DOWNLOAD_URL}"
info "downloading checksums.txt..."
curl -fsSL -o "${TMPDIR}/checksums.txt" "${CHECKSUM_URL}" \
  || fatal "checksum download failed: ${CHECKSUM_URL}"

# --- verify SHA-256 --------------------------------------------------------
EXPECTED="$(grep -E " ${ARCHIVE}$" "${TMPDIR}/checksums.txt" | awk '{print $1}')"
[[ -n "${EXPECTED}" ]] || fatal "no checksum entry for ${ARCHIVE} in checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL="$(sha256sum "${TMPDIR}/${ARCHIVE}" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL="$(shasum -a 256 "${TMPDIR}/${ARCHIVE}" | awk '{print $1}')"
else
  fatal "neither sha256sum nor shasum is available; cannot verify checksum"
fi
[[ "${ACTUAL}" = "${EXPECTED}" ]] \
  || fatal "checksum mismatch for ${ARCHIVE} (expected ${EXPECTED}, got ${ACTUAL})"
info "checksum verified: ${ACTUAL}"

# --- extract + install -----------------------------------------------------
tar -xzf "${TMPDIR}/${ARCHIVE}" -C "${TMPDIR}"
[[ -f "${TMPDIR}/${BIN_NAME}" ]] \
  || fatal "${BIN_NAME} binary not found in archive"
mkdir -p "${INSTALL_DIR}"
if [[ -w "${INSTALL_DIR}" ]]; then
  install -m 0755 "${TMPDIR}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"
else
  info "sudo required to write to ${INSTALL_DIR}"
  sudo install -m 0755 "${TMPDIR}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"
fi
INSTALLED="${INSTALL_DIR}/${BIN_NAME}"
info "installed ${INSTALLED} ($("${INSTALLED}" --version 2>/dev/null || echo "${VERSION}"))"

# --- PATH hint -------------------------------------------------------------
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    path_hint=$(printf "export PATH=\"%s:\$PATH\"" "${INSTALL_DIR}")
    printf '\n\033[1;33mnote:\033[0m %s is not on your PATH. Add this to your shell rc:\n    %s\n\n' \
      "${INSTALL_DIR}" "${path_hint}" >&2
    ;;
esac
