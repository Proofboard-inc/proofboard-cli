#!/bin/sh
set -eu

PROJECT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
BUILD_DIR=$(mktemp -d)
trap 'rm -rf "$BUILD_DIR"' EXIT

env CGO_ENABLED=0 go build -trimpath -o "$BUILD_DIR/proofboard" "$PROJECT_DIR/cmd/proofboard"
env CGO_ENABLED=0 go build -trimpath -o "$BUILD_DIR/mock-backend" "$PROJECT_DIR/scripts/mock_backend"

PB="$BUILD_DIR/proofboard" \
MOCK_BIN="$BUILD_DIR/mock-backend" \
	"$PROJECT_DIR/scripts/test_no_system_pollution.sh" \
	bash "$PROJECT_DIR/scripts/e2e_journey.sh"
