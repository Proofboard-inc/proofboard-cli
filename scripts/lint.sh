#!/usr/bin/env sh
set -eu

gofmt -w ./cmd ./internal
go vet ./...
