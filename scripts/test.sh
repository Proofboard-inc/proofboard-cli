#!/usr/bin/env sh
set -eu

PROJECT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

"$PROJECT_DIR/scripts/test_no_system_pollution.sh" go test -count=1 ./...
"$PROJECT_DIR/scripts/test_e2e.sh"
