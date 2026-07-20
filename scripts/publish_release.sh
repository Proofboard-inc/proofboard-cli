#!/bin/bash
set -euo pipefail

REPO="Proofboard-inc/proofboard-cli"
VERSION=$(sed -n 's/^const Version = "\(.*\)"/\1/p' internal/version/version.go)
TAG="v${VERSION}"

echo "Ensuring release exists for tag $TAG..."
RELEASE_ID=$(curl -sS -H "Authorization: token $GITHUB_TOKEN" "https://api.github.com/repos/$REPO/releases/tags/$TAG" | jq -r '.id // empty')
if [ -z "$RELEASE_ID" ] || [ "$RELEASE_ID" = "null" ]; then
  CREATE_RESP=$(curl -sS -X POST -H "Authorization: token $GITHUB_TOKEN" -H "Content-Type: application/json" \
    -d '{"tag_name":"'"$TAG"'","name":"Proofboard CLI '"$TAG"'","body":"Proofboard CLI release '"$TAG"'."}' \
    "https://api.github.com/repos/$REPO/releases")
  RELEASE_ID=$(echo "$CREATE_RESP" | jq -r '.id')
  UPLOAD_URL=$(echo "$CREATE_RESP" | jq -r '.upload_url' | sed -e 's/{?name,label}//')
else
  UPLOAD_URL=$(curl -sS -H "Authorization: token $GITHUB_TOKEN" "https://api.github.com/repos/$REPO/releases/$RELEASE_ID" | jq -r '.upload_url' | sed -e 's/{?name,label}//')
fi

echo "Upload URL: $UPLOAD_URL"

for FILE in dist/proofboard dist/proofboard-* dist/checksums.txt; do
  [ -e "$FILE" ] || continue
  FILENAME=$(basename "$FILE")
  echo "Uploading $FILENAME..."
  curl -s -X POST -H "Authorization: token $GITHUB_TOKEN" -H "Content-Type: application/octet-stream" \
    --data-binary "@$FILE" "$UPLOAD_URL?name=$FILENAME"
done

for FILE in dist/*.sig; do
  FILENAME=$(basename "$FILE")
  echo "Uploading $FILENAME..."
  curl -s -X POST -H "Authorization: token $GITHUB_TOKEN" -H "Content-Type: application/octet-stream" \
    --data-binary "@$FILE" "$UPLOAD_URL?name=$FILENAME"
done

echo "Done!"
