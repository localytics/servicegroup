#!/bin/bash
# Run in a container for CI: docker-compose run --rm --entrypoint=ci/checks.sh go
set -ex

./install_tools.sh

# Run tests, with race detector on.
CGO_ENABLED=1 go test -v -race -coverprofile=coverage.txt -covermode=atomic -count=1 ./...

internal/tools/bin/golangci-lint run
