#!/bin/sh
# Installs project-local tooling for development use in internal/tools/bin/. Can be used in docker 
# or locally as long as the docker image mounts its own internal/tools/bin/ folder; the host's 
# binary format is typically incompatible with the container.
set -ex
# gex (installed globally) manages our Go tool-only dependencies in internal/tools/
cd ./internal/tools
which gex || GO111MODULE=off go get github.com/izumin5210/gex/cmd/gex

# swap OS binaries (if we have a copy for our architecture) to avoid local vs docker binary format issues
binarch="bin-`go env GOOS`-`go env GOARCH`"
mkdir -p $binarch bin
rsync -a --delete $binarch/ bin/

# install missing binaries & back up for this architecture
gex --build
rsync -a --delete bin/ $binarch/