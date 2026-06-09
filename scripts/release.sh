#!/usr/bin/env sh
set -eu

goreleaser release --clean --config build/goreleaser.yaml
