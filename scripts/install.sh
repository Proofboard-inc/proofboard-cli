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
LATEST_JSON=$(curl -sSL https://releases.proofboard.io/latest.json || echo "")
LATEST_VERSION=$(echo "$LATEST_JSON" | grep -o '"version": *"[^"]*"' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_VERSION" ]; then
    echo "Fallback: Using hardcoded latest version v1.8.10"
    LATEST_VERSION="v1.8.10"
fi

DOWNLOAD_URL="https://releases.proofboard.io/${LATEST_VERSION}/${BINARY_NAME}"
INSTALL_DIR="/usr/local/bin"
TEMP_DIR=$(mktemp -d)
TEMP_BINARY="${TEMP_DIR}/proofboard"
trap 'rm -rf "$TEMP_DIR"' EXIT

echo "Downloading ${BINARY_NAME} ${LATEST_VERSION}..."
curl -fsSL -o "$TEMP_BINARY" "$DOWNLOAD_URL" || {
    echo "Download failed from releases.proofboard.io. Falling back to GitHub releases..."
    curl -fsSL -o "$TEMP_BINARY" "https://github.com/Proofboard-inc/proofboard-cli/releases/download/${LATEST_VERSION}/${BINARY_NAME}"
}

echo "Installing to ${INSTALL_DIR}/proofboard..."
chmod +x "$TEMP_BINARY"
sudo install -m 0755 "$TEMP_BINARY" "${INSTALL_DIR}/proofboard"
"${INSTALL_DIR}/proofboard" agent enable

echo "Proofboard Career Agent installed and running. Keep building software; Proofboard will handle the rest."
