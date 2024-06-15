#!/usr/bin/env bash

# This script is used to build the OpenFero project locally. It performs the following steps:
# 1. Determines the operating system and architecture.
# 2. Checks if Docker Desktop or Lima is installed (required for macOS).
# 3. Increments the version and makes a local release.
# 4. Builds the project using goreleaser.
# 5. Copies the binary to the project root directory.
# 6. Builds a Docker image using either Lima or Docker Desktop.
#
# Usage: ./local-build.sh

OS_ARCH=$(go env GOOS)_$(go env GOARCH)
GITROOT=$(git rev-parse --show-toplevel)
LIMA_INSTALLED=false

# trap to always cleanup the goreleaser build
trap cleanup EXIT

cleanup() {
	# Removes the goreleaser build
	rm -rf "${GITROOT}/dist"
	rm -f "${GITROOT}/openfero"
}

copy_binary() {
	# Copies the binary to the project root directory
	cp "${GITROOT}/dist/openfero_${OS_ARCH}/openfero" "${GITROOT}/"
}

if [ "$(uname)" == "Darwin" ]; then
	if ! command -v docker &>/dev/null; then
		if ! command -v lima &>/dev/null; then
			echo "Please install Docker Desktop or Lima"
			exit 1
		else
			LIMA_INSTALLED=true
		fi
	fi
fi

# make a local release by incrementing the version
"${GITROOT}"/scripts/release.sh patch local

# build the project using goreleaser
goreleaser build --snapshot --clean

# when lima is installed set OS_ARCH not to darwin set it to linux
if [ "$LIMA_INSTALLED" == "true" ]; then
	OS_ARCH=linux_$(go env GOARCH)
	copy_binary
	lima nerdctl build -f goreleaser.dockerfile -t openfero:latest "${GITROOT}/"
else
	copy_binary
	docker build -f goreleaser.dockerfile -t openfero:latest "${GITROOT}/"
fi
