#!/bin/sh
set -e

# Proofboard Career Agent installation script for Linux and macOS.
#
# Usage:
#   curl -fsSL https://proofboard.io/install.sh | sh
#
# macOS users: proofboard.io will present `brew install` as the primary
# install path there (avoids the Gatekeeper "unidentified developer" warning
# this script's unnotarized binary can trigger). This script remains the
# fallback / power-user path.
#
# The script resolves the latest published release, verifies the release
# signature, and then hands over to the Career Agent's own installer so the
# executable and the background agent are registered the same way as a manual
# `proofboard install`. The install goes into the current account, so no
# administrator access is required.
#
# Releases are read from proofboard.io first. If that distribution origin is
# unavailable, the script falls back to the latest release published directly
# on GitHub. Private-repository fallback can use PROOFBOARD_GITHUB_TOKEN
# (GH_TOKEN and GITHUB_TOKEN are also honoured), or an authenticated GitHub CLI.
#
# Environment overrides (used by release verification and by pinned installs):
#   PROOFBOARD_VERSION              install a specific tag instead of the latest
#   PROOFBOARD_GITHUB_TOKEN         token used to read releases from the repository
#   PROOFBOARD_LATEST_RELEASE_URL   release manifest URL
#   PROOFBOARD_DOWNLOAD_BASE_URL    directory URL holding the release artifacts
#   PROOFBOARD_INSTALL_VERIFY_ONLY  download and verify, then stop
#   PROOFBOARD_SYSTEM_INSTALL       install for every account (needs sudo)

REPO="Proofboard-inc/proofboard-cli"
PINNED_VERSION="v1.16.4"
PUBLIC_DOWNLOAD_HOST="https://proofboard.io"

log() {
    printf '%s\n' "$*"
}

fail() {
    printf '%s\n' "$*" >&2
    exit 1
}

read_json_string() {
    printf '%s' "$1" | grep -o "\"$2\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" | head -n 1 | sed -E 's/.*"([^"]*)".*/\1/'
}

resolve_repository_token() {
    for candidate in "${PROOFBOARD_GITHUB_TOKEN:-}" "${GH_TOKEN:-}" "${GITHUB_TOKEN:-}"; do
        if [ -n "$candidate" ]; then
            printf '%s' "$candidate"
            return 0
        fi
    done
    if command -v gh >/dev/null 2>&1; then
        gh auth token 2>/dev/null || true
    fi
}

repository_api_get() {
    if [ -n "$REPOSITORY_TOKEN" ]; then
        curl -fsSL -H "Authorization: Bearer ${REPOSITORY_TOKEN}" -H "Accept: application/vnd.github+json" \
            -H "User-Agent: proofboard-installer" "$1" 2>/dev/null || true
    else
        curl -fsSL -H "Accept: application/vnd.github+json" -H "User-Agent: proofboard-installer" "$1" 2>/dev/null || true
    fi
}

# Reads the numeric asset identifier for an asset name out of a release
# payload. Splitting on braces keeps each asset object on its own line, so the
# identifier and the name are matched within the same object.
release_asset_id() {
    printf '%s' "$RELEASE_JSON" | tr '{' '\n' |
        grep "\"name\"[[:space:]]*:[[:space:]]*\"$1\"" |
        grep -o "\"id\"[[:space:]]*:[[:space:]]*[0-9]*" | head -n 1 | grep -o '[0-9]*$'
}

download_asset() {
    asset_name="$1"
    destination="$2"
    if [ "$RELEASE_SOURCE" != "repository" ] &&
        curl -fsSL -o "$destination" "${DOWNLOAD_BASE_URL}/${asset_name}"; then
        return 0
    fi

    # The root-domain distribution is primary. Resolve and download the same
    # asset directly from the matching GitHub release when it is unavailable.
    if [ -z "$RELEASE_JSON" ]; then
        if [ -n "$RELEASE_TAG" ]; then
            RELEASE_JSON=$(repository_api_get "https://api.github.com/repos/${REPO}/releases/tags/${RELEASE_TAG}")
        else
            RELEASE_JSON=$(repository_api_get "https://api.github.com/repos/${REPO}/releases/latest")
        fi
    fi
    resolved_repository_tag=$(read_json_string "$RELEASE_JSON" tag_name)
    if [ -n "$resolved_repository_tag" ]; then
        RELEASE_TAG="$resolved_repository_tag"
    fi
    asset_id=$(release_asset_id "$asset_name")
    [ -n "$asset_id" ] ||
        fail "Neither proofboard.io nor the GitHub release ${RELEASE_TAG} contains ${asset_name}."
    if [ -n "$REPOSITORY_TOKEN" ]; then
        curl -fsSL -o "$destination" -H "Authorization: Bearer ${REPOSITORY_TOKEN}" \
            -H "Accept: application/octet-stream" -H "User-Agent: proofboard-installer" \
            "https://api.github.com/repos/${REPO}/releases/assets/${asset_id}" ||
            fail "Could not download ${asset_name} from proofboard.io or GitHub."
    else
        curl -fsSL -o "$destination" -H "Accept: application/octet-stream" \
            -H "User-Agent: proofboard-installer" \
            "https://api.github.com/repos/${REPO}/releases/assets/${asset_id}" ||
            fail "Could not download ${asset_name} from proofboard.io or GitHub."
    fi
}

log "Installing Proofboard Career Agent..."

command -v curl >/dev/null 2>&1 || fail "curl is required to install the Proofboard Career Agent."
command -v openssl >/dev/null 2>&1 || fail "OpenSSL is required to verify the Proofboard release signature."

OS="$(uname -s)"
ARCH="$(uname -m)"

if [ "$OS" = "Darwin" ]; then
    OS="darwin"
elif [ "$OS" = "Linux" ]; then
    OS="linux"
else
    fail "Unsupported OS: $OS"
fi

if [ "$ARCH" = "x86_64" ]; then
    ARCH="amd64"
elif [ "$ARCH" = "arm64" ] || [ "$ARCH" = "aarch64" ]; then
    ARCH="arm64"
else
    fail "Unsupported architecture: $ARCH"
fi

# Linux and Windows on ARM are built and published, so no architecture is
# refused here any more. A single-board machine or an ARM laptop was
# previously told the platform did not exist, which was true of the release
# rather than of the tool.

# The release carries the executable under both names. The product name
# matches every installer package on the release page; the lowercase name is
# what versions up to 1.13.2 look for and is kept so those can still update.
# Preferred first, legacy second.
BINARY_NAME="Proofboard-Career-Agent-${OS}-${ARCH}"
LEGACY_BINARY_NAME="proofboard-${OS}-${ARCH}"

# Resolve the release to install. An explicit download base short-circuits
# every remote lookup so pinned and offline installs stay deterministic.
RELEASE_TAG="${PROOFBOARD_VERSION:-}"
RELEASE_JSON=""
RELEASE_SOURCE="download-host"
DOWNLOAD_BASE_URL="${PROOFBOARD_DOWNLOAD_BASE_URL:-}"
REPOSITORY_TOKEN=""

if [ -z "$DOWNLOAD_BASE_URL" ]; then
    REPOSITORY_TOKEN=$(resolve_repository_token)

    # Primary source: the root-domain release manifest.
    if [ -z "$RELEASE_TAG" ]; then
        LATEST_JSON=$(curl -fsSL "${PROOFBOARD_LATEST_RELEASE_URL:-${PUBLIC_DOWNLOAD_HOST}/latest.json}" 2>/dev/null || true)
        RELEASE_TAG=$(read_json_string "$LATEST_JSON" version)
        DOWNLOAD_BASE_URL=$(read_json_string "$LATEST_JSON" url)
    fi

    if [ -n "$RELEASE_TAG" ]; then
        RELEASE_SOURCE="download-host"
    else
        # Fallback source: the release published directly on GitHub.
        RELEASE_JSON=$(repository_api_get "https://api.github.com/repos/${REPO}/releases/latest")
        RELEASE_TAG=$(read_json_string "$RELEASE_JSON" tag_name)
    fi
    if [ -n "$RELEASE_JSON" ]; then
        RELEASE_SOURCE="repository"
    fi
fi

if [ -z "$RELEASE_TAG" ]; then
    log "Could not resolve the latest release; falling back to ${PINNED_VERSION}."
    RELEASE_TAG="$PINNED_VERSION"
fi

case "$RELEASE_TAG" in
    v*) ;;
    *) RELEASE_TAG="v${RELEASE_TAG}" ;;
esac

if [ "$RELEASE_SOURCE" != "repository" ] && [ -z "$DOWNLOAD_BASE_URL" ]; then
    DOWNLOAD_BASE_URL="${PUBLIC_DOWNLOAD_HOST}/${RELEASE_TAG}"
fi

TEMP_DIR=$(mktemp -d)
TEMP_BINARY="${TEMP_DIR}/proofboard"
TEMP_SIGNATURE="${TEMP_DIR}/proofboard.sig"
TEMP_PUBLIC_KEY="${TEMP_DIR}/proofboard-release-public.pem"
trap 'rm -rf "$TEMP_DIR"' EXIT

log "Downloading ${BINARY_NAME} ${RELEASE_TAG}..."
# Whichever name resolves, the signature has to come from the same one, or
# verification compares the binary against a different file's signature.
if ! download_asset "$BINARY_NAME" "$TEMP_BINARY" 2>/dev/null; then
    BINARY_NAME="$LEGACY_BINARY_NAME"
    download_asset "$BINARY_NAME" "$TEMP_BINARY"
fi
download_asset "${BINARY_NAME}.sig" "$TEMP_SIGNATURE"

printf '%s\n' \
    '-----BEGIN PUBLIC KEY-----' \
    'MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEdYPsxqaryQ9bQI3G3hQpsmyrTGs0' \
    'nKxvQXQC+nAK+EsNF6VEofCYuX42bTeooKLR1Ol+Eh3NhWErh4tfSkH1mA==' \
    '-----END PUBLIC KEY-----' > "$TEMP_PUBLIC_KEY"
openssl dgst -sha256 -verify "$TEMP_PUBLIC_KEY" -signature "$TEMP_SIGNATURE" "$TEMP_BINARY" >/dev/null ||
    fail "Proofboard release signature verification failed."

if [ "${PROOFBOARD_INSTALL_VERIFY_ONLY:-0}" = "1" ]; then
    log "Proofboard Career Agent ${RELEASE_TAG} signature verified."
    exit 0
fi

chmod +x "$TEMP_BINARY"

# Install (or replace) the executable and register the background agent. This
# installs into the current account and needs no administrator access, which
# keeps the Career Agent installable on managed machines where sudo is not
# granted. A machine-wide install stays available for anyone who wants it.
if [ "${PROOFBOARD_SYSTEM_INSTALL:-0}" = "1" ]; then
    command -v sudo >/dev/null 2>&1 ||
        fail "A machine-wide install needs administrator access, but sudo is unavailable."
    # When the script is piped into a shell, stdin is the script itself.
    # Reattach the terminal so the password prompt reaches the person running
    # the install. Opening the terminal can fail when there is no controlling
    # terminal at all, so the attempt is made in a subshell first.
    if [ ! -t 0 ] && (exec </dev/tty) 2>/dev/null; then
        exec </dev/tty
    fi
    log "Installing for every account on this machine..."
    sudo "$TEMP_BINARY" install --system ||
        fail "Machine-wide install failed. Unset PROOFBOARD_SYSTEM_INSTALL to install into your own account instead."
    # The background agent belongs to the signed-in user, not to root.
    INSTALL_PATH="/usr/local/bin/proofboard"
    "$INSTALL_PATH" agent enable ||
        log "The Career Agent could not be started automatically. Run: proofboard agent enable"
else
    "$TEMP_BINARY" install
    INSTALL_PATH="${PROOFBOARD_INSTALL_DIR:-${HOME}/.local/bin}/proofboard"
    if [ ! -x "$INSTALL_PATH" ] && [ -x "/usr/local/bin/proofboard" ]; then
        # An existing writable machine-wide install is upgraded in place.
        INSTALL_PATH="/usr/local/bin/proofboard"
    fi
fi

# Install (or refresh) shell completions for the signed-in user. This is a
# convenience step, so a failure here must not fail the installation.
if [ -x "$INSTALL_PATH" ]; then
    printf 'y\n' | "$INSTALL_PATH" completion ||
        log "Shell completions could not be installed automatically. Run: proofboard completion"
fi

# Connect the Career Agent straight away, so opening a project is all that is
# left to do. An existing connection is kept, which is what makes re-running
# this script an update rather than a fresh sign-in.
if [ -x "$INSTALL_PATH" ]; then
    if [ -f "${HOME}/.proofboard/credentials.json" ]; then
        log "Career Agent is already connected."
    elif [ ! -t 0 ] && ! (exec </dev/tty) 2>/dev/null; then
        log "Run 'proofboard auth' to connect your Career Agent."
    else
        [ -t 0 ] || exec </dev/tty
        "$INSTALL_PATH" auth || log "Run 'proofboard auth' when you are ready to connect your Career Agent."
    fi

    # Pick up the project this was installed from, so a workspace that is
    # already open is detected without waiting for the next shell.
    "$INSTALL_PATH" detect >/dev/null 2>&1 || true
fi

log "Proofboard Career Agent installed and running. Keep building software; Proofboard will handle the rest."
