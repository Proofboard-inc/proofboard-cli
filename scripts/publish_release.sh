#!/bin/bash
set -euo pipefail
shopt -s nullglob

REPO="Proofboard-inc/proofboard-cli"
VERSION=$(sed -n 's/^const Version = "\(.*\)"/\1/p' internal/version/version.go)
TAG="v${VERSION}"

REQUIRED_FILES="
dist/proofboard-linux-amd64
dist/proofboard-linux-amd64.sig
dist/proofboard-darwin-amd64
dist/proofboard-darwin-amd64.sig
dist/proofboard-darwin-arm64
dist/proofboard-darwin-arm64.sig
dist/proofboard-windows-amd64.exe
dist/proofboard-windows-amd64.exe.sig
dist/Proofboard-Career-Agent-linux-amd64.deb
dist/Proofboard-Career-Agent-darwin-amd64.pkg
dist/Proofboard-Career-Agent-darwin-arm64.pkg
dist/Proofboard-Career-Agent-windows-amd64-setup.exe
dist/install.sh
dist/install.ps1
dist/install.cmd
dist/latest.json
dist/checksums.txt
"

for FILE in $REQUIRED_FILES; do
  if [ ! -f "$FILE" ]; then
    echo "Missing required release artifact: $FILE" >&2
    exit 1
  fi
done

NPM_PACKAGES=(dist/proofboard-cli-*.tgz)
if [ "${#NPM_PACKAGES[@]}" -ne 1 ]; then
  echo "Expected exactly one npm release package, found ${#NPM_PACKAGES[@]}." >&2
  exit 1
fi

echo "Ensuring release exists for tag $TAG..."
RELEASE_ID=$(curl -sS -H "Authorization: token $GITHUB_TOKEN" "https://api.github.com/repos/$REPO/releases/tags/$TAG" | jq -r '.id // empty')
if [ -z "$RELEASE_ID" ] || [ "$RELEASE_ID" = "null" ]; then
  CREATE_RESP=$(curl -sS -X POST -H "Authorization: token $GITHUB_TOKEN" -H "Content-Type: application/json" \
    -d '{"tag_name":"'"$TAG"'","name":"Proofboard Career Agent '"$TAG"'","body":"Proofboard Career Agent release '"$TAG"'."}' \
    "https://api.github.com/repos/$REPO/releases")
  RELEASE_ID=$(echo "$CREATE_RESP" | jq -r '.id')
  UPLOAD_URL=$(echo "$CREATE_RESP" | jq -r '.upload_url' | sed -e 's/{?name,label}//')
else
  UPLOAD_URL=$(curl -sS -H "Authorization: token $GITHUB_TOKEN" "https://api.github.com/repos/$REPO/releases/$RELEASE_ID" | jq -r '.upload_url' | sed -e 's/{?name,label}//')
fi

echo "Upload URL: $UPLOAD_URL"

for FILE in $REQUIRED_FILES "${NPM_PACKAGES[@]}"; do
  FILENAME=$(basename "$FILE")
  echo "Uploading $FILENAME..."
  curl -s -X POST -H "Authorization: token $GITHUB_TOKEN" -H "Content-Type: application/octet-stream" \
    --data-binary "@$FILE" "$UPLOAD_URL?name=$FILENAME"
done

echo "Done!"
