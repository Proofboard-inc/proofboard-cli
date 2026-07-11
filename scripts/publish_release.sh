#!/bin/bash
set -e

REPO="Proofboard-inc/proofboard-cli"
TAG="v1.8.4"

echo "Deleting existing release for tag $TAG..."
RELEASE_ID=$(curl -s -H "Authorization: token $GITHUB_TOKEN" "https://api.github.com/repos/$REPO/releases/tags/$TAG" | jq -r '.id // empty')
if [ -n "$RELEASE_ID" ] && [ "$RELEASE_ID" != "null" ]; then
  curl -s -X DELETE -H "Authorization: token $GITHUB_TOKEN" "https://api.github.com/repos/$REPO/releases/$RELEASE_ID"
fi

echo "Creating new release for tag $TAG..."
CREATE_RESP=$(curl -s -X POST -H "Authorization: token $GITHUB_TOKEN" -H "Content-Type: application/json" \
  -d '{"tag_name":"'"$TAG"'","name":"Proofboard CLI v1.8.4","body":"Added secure release signing and verification using ECDSA keys. Removed Phase 6 Handshake. Added local fraud detection. Finalized notification architecture and typography."}' \
  "https://api.github.com/repos/$REPO/releases")

UPLOAD_URL=$(echo "$CREATE_RESP" | jq -r '.upload_url' | sed -e 's/{?name,label}//')
echo "Upload URL: $UPLOAD_URL"

for FILE in dist/proofboard-*; do
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
