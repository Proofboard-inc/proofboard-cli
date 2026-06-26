#!/bin/bash
set -e

mkdir -p dist

echo "Building Linux amd64..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o dist/proofboard-linux-amd64 ./cmd/proofboard

echo "Building macOS amd64..."
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o dist/proofboard-darwin-amd64 ./cmd/proofboard

echo "Building macOS arm64..."
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o dist/proofboard-darwin-arm64 ./cmd/proofboard

echo "Building Windows amd64..."
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o dist/proofboard-windows-amd64.exe ./cmd/proofboard

echo "Done!"
