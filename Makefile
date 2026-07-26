.PHONY: all build test lint release clean

all: test build

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o proofboard ./cmd/proofboard

test:
	./scripts/test_no_system_pollution.sh go test -count=1 ./...

lint:
	go vet ./...

release:
	./scripts/build-release.sh

clean:
	rm -rf dist proofboard coverage.out
