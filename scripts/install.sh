#!/bin/sh
set -e

# Proofboard Career Agent installation script
echo "Installing Proofboard Career Agent..."

OS="$(uname -s)"
ARCH="$(uname -m)"

if [ "$OS" = "Darwin" ]; then
    OS="darwin"
elif [ "$OS" = "Linux" ]; then
    OS="linux"
else
    echo "Unsupported OS: $OS"
    exit 1
fi

if [ "$ARCH" = "x86_64" ]; then
    ARCH="amd64"
elif [ "$ARCH" = "arm64" ] || [ "$ARCH" = "aarch64" ]; then
    ARCH="arm64"
else
    echo "Unsupported architecture: $ARCH"
    exit 1
fi

if [ "$OS" = "linux" ] && [ "$ARCH" = "arm64" ]; then
    echo "Linux arm64 is not available yet. Supported Linux architecture: amd64."
    exit 1
fi

BINARY_NAME="proofboard-${OS}-${ARCH}"

# Determine latest version
LATEST_RELEASE_URL="${PROOFBOARD_LATEST_RELEASE_URL:-https://releases.proofboard.io/latest.json}"
LATEST_JSON=$(curl -fsSL "$LATEST_RELEASE_URL" || echo "")
LATEST_VERSION=$(printf '%s' "$LATEST_JSON" | grep -o '"version": *"[^"]*"' | sed -E 's/.*"([^"]+)".*/\1/')
DOWNLOAD_BASE_URL=$(printf '%s' "$LATEST_JSON" | grep -o '"url": *"[^"]*"' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_VERSION" ]; then
    echo "Fallback: Using hardcoded latest version v1.8.11"
    LATEST_VERSION="v1.8.11"
fi
case "$LATEST_VERSION" in
    v*) RELEASE_TAG="$LATEST_VERSION" ;;
    *) RELEASE_TAG="v$LATEST_VERSION" ;;
esac
if [ -z "$DOWNLOAD_BASE_URL" ]; then
    DOWNLOAD_BASE_URL="${PROOFBOARD_DOWNLOAD_BASE_URL:-https://releases.proofboard.io/$RELEASE_TAG}"
fi

DOWNLOAD_URL="${DOWNLOAD_BASE_URL}/${BINARY_NAME}"
INSTALL_DIR="/usr/local/bin"
TEMP_DIR=$(mktemp -d)
TEMP_BINARY="${TEMP_DIR}/proofboard"
TEMP_SIGNATURE="${TEMP_DIR}/proofboard.sig"
TEMP_PUBLIC_KEY="${TEMP_DIR}/proofboard-release-public.pem"
trap 'rm -rf "$TEMP_DIR"' EXIT

echo "Downloading ${BINARY_NAME} ${LATEST_VERSION}..."
curl -fsSL -o "$TEMP_BINARY" "$DOWNLOAD_URL"
curl -fsSL -o "$TEMP_SIGNATURE" "${DOWNLOAD_URL}.sig"
printf '%s\n' \
    '-----BEGIN PUBLIC KEY-----' \
    'MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEdYPsxqaryQ9bQI3G3hQpsmyrTGs0' \
    'nKxvQXQC+nAK+EsNF6VEofCYuX42bTeooKLR1Ol+Eh3NhWErh4tfSkH1mA==' \
    '-----END PUBLIC KEY-----' > "$TEMP_PUBLIC_KEY"
if ! command -v openssl >/dev/null 2>&1; then
    echo "OpenSSL is required to verify the Proofboard release signature." >&2
    exit 1
fi
openssl dgst -sha256 -verify "$TEMP_PUBLIC_KEY" -signature "$TEMP_SIGNATURE" "$TEMP_BINARY" >/dev/null
if [ "${PROOFBOARD_INSTALL_VERIFY_ONLY:-0}" = "1" ]; then
    echo "Proofboard Career Agent ${LATEST_VERSION} signature verified."
    exit 0
fi

echo "Installing to ${INSTALL_DIR}/proofboard..."
chmod +x "$TEMP_BINARY"
sudo install -m 0755 "$TEMP_BINARY" "${INSTALL_DIR}/proofboard"
"${INSTALL_DIR}/proofboard" agent enable

echo "Proofboard Career Agent installed and running. Keep building software; Proofboard will handle the rest."
