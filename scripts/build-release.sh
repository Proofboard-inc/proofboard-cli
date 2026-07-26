#!/bin/bash
set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
if [ -f "$SCRIPT_DIR/go.mod" ]; then
  PROJECT_DIR="$SCRIPT_DIR"
else
  PROJECT_DIR=$(dirname "$SCRIPT_DIR")
fi
cd "$PROJECT_DIR"

VERSION=$(sed -n 's/^const Version = "\(.*\)"/\1/p' internal/version/version.go)
echo "Building Proofboard CLI release v${VERSION}..."

mkdir -p dist
rm -rf dist/*

echo "[1/7] Cross-compiling static Go binaries..."
env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/proofboard-linux-amd64 ./cmd/proofboard
env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/proofboard-darwin-amd64 ./cmd/proofboard
env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o dist/proofboard-darwin-arm64 ./cmd/proofboard
env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/proofboard-windows-amd64.exe ./cmd/proofboard

echo "[2/7] Generating detached signatures..."
KEY_FILE="keys/proofboard_private.pem"
if [ ! -f "$KEY_FILE" ]; then
  echo "Error: Private key file $KEY_FILE not found." >&2
  exit 1
fi

go run scripts/sign.go "$KEY_FILE" dist/proofboard-linux-amd64
go run scripts/sign.go "$KEY_FILE" dist/proofboard-darwin-amd64
go run scripts/sign.go "$KEY_FILE" dist/proofboard-darwin-arm64
go run scripts/sign.go "$KEY_FILE" dist/proofboard-windows-amd64.exe

echo "[3/7] Verifying binary signatures..."
go run ./scripts/verify_signature dist/proofboard-linux-amd64 dist/proofboard-linux-amd64.sig
go run ./scripts/verify_signature dist/proofboard-darwin-amd64 dist/proofboard-darwin-amd64.sig
go run ./scripts/verify_signature dist/proofboard-darwin-arm64 dist/proofboard-darwin-arm64.sig
go run ./scripts/verify_signature dist/proofboard-windows-amd64.exe dist/proofboard-windows-amd64.exe.sig

echo "[4/7] Packaging native installers..."
chmod +x dist/proofboard-linux-amd64 dist/proofboard-darwin-amd64 dist/proofboard-darwin-arm64
scripts/package_linux_deb.sh "$VERSION" dist/proofboard-linux-amd64 dist/Proofboard-Career-Agent-linux-amd64.deb
scripts/package_macos_pkg.sh "$VERSION" dist/proofboard-darwin-amd64 dist/Proofboard-Career-Agent-darwin-amd64.pkg
scripts/package_macos_pkg.sh "$VERSION" dist/proofboard-darwin-arm64 dist/Proofboard-Career-Agent-darwin-arm64.pkg
scripts/package_windows_exe.sh "$VERSION" dist/proofboard-windows-amd64.exe dist/Proofboard-Career-Agent-windows-amd64-setup.exe

echo "[5/7] Staging installation scripts..."
cp scripts/install.sh dist/install.sh
cp scripts/install.ps1 dist/install.ps1
cp scripts/install.cmd dist/install.cmd

echo "[6/7] Building npm package tarball..."
NPM_STAGE=$(mktemp -d)
trap 'rm -rf "$NPM_STAGE"' EXIT
cp -R npm-package/. "$NPM_STAGE/"
mkdir -p "$NPM_STAGE/vendor"
cp \
  dist/proofboard-linux-amd64 \
  dist/proofboard-linux-amd64.sig \
  dist/proofboard-darwin-amd64 \
  dist/proofboard-darwin-amd64.sig \
  dist/proofboard-darwin-arm64 \
  dist/proofboard-darwin-arm64.sig \
  dist/proofboard-windows-amd64.exe \
  dist/proofboard-windows-amd64.exe.sig \
  "$NPM_STAGE/vendor/"
npm pack "$NPM_STAGE" --pack-destination dist

echo "[7/7] Generating metadata and checksums..."
printf '{"version":"%s","url":"https://proofboard.io/v%s"}\n' "$VERSION" "$VERSION" > dist/latest.json

(
  cd dist
  sha256sum \
    proofboard-linux-amd64 \
    proofboard-linux-amd64.sig \
    proofboard-darwin-amd64 \
    proofboard-darwin-amd64.sig \
    proofboard-darwin-arm64 \
    proofboard-darwin-arm64.sig \
    proofboard-windows-amd64.exe \
    proofboard-windows-amd64.exe.sig \
    Proofboard-Career-Agent-linux-amd64.deb \
    Proofboard-Career-Agent-darwin-amd64.pkg \
    Proofboard-Career-Agent-darwin-arm64.pkg \
    Proofboard-Career-Agent-windows-amd64-setup.exe \
    install.sh \
    install.ps1 \
    install.cmd \
    proofboard-cli-*.tgz \
    latest.json > checksums.txt
)

echo "Verifying complete artifact set in dist/..."
REQUIRED_FILES=(
  "dist/proofboard-linux-amd64"
  "dist/proofboard-linux-amd64.sig"
  "dist/proofboard-darwin-amd64"
  "dist/proofboard-darwin-amd64.sig"
  "dist/proofboard-darwin-arm64"
  "dist/proofboard-darwin-arm64.sig"
  "dist/proofboard-windows-amd64.exe"
  "dist/proofboard-windows-amd64.exe.sig"
  "dist/Proofboard-Career-Agent-linux-amd64.deb"
  "dist/Proofboard-Career-Agent-darwin-amd64.pkg"
  "dist/Proofboard-Career-Agent-darwin-arm64.pkg"
  "dist/Proofboard-Career-Agent-windows-amd64-setup.exe"
  "dist/install.sh"
  "dist/install.ps1"
  "dist/install.cmd"
  "dist/latest.json"
  "dist/checksums.txt"
)

for file in "${REQUIRED_FILES[@]}"; do
  if [ ! -f "$file" ]; then
    echo "Error: Missing required release artifact: $file" >&2
    exit 1
  fi
done

NPM_TARBALLS=(dist/proofboard-cli-*.tgz)
if [ "${#NPM_TARBALLS[@]}" -lt 1 ] || [ ! -f "${NPM_TARBALLS[0]}" ]; then
  echo "Error: Missing npm tarball in dist/" >&2
  exit 1
fi

echo "Successfully built and verified all release artifacts in dist/!"
