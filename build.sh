#!/bin/bash

# Build script for lab-update-esxi-cert with version injection
# This script demonstrates how to build the application with version information
# injected at build time using Go's -ldflags parameter

set -e

# Get version information from Git
VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "development")}
COMMIT=${COMMIT:-$(git rev-parse HEAD 2>/dev/null || echo "unknown")}
BUILD_DATE=${BUILD_DATE:-$(date -u '+%Y-%m-%dT%H:%M:%SZ')}
GIT_TAG=${GIT_TAG:-$(git describe --tags --exact-match 2>/dev/null || echo "")}

# Package path for version variables
VERSION_PKG="lab-update-esxi-cert/internal/version"

# Build flags
LDFLAGS="-X '${VERSION_PKG}.Version=${VERSION}' \
         -X '${VERSION_PKG}.GitCommit=${COMMIT}' \
         -X '${VERSION_PKG}.BuildDate=${BUILD_DATE}' \
         -X '${VERSION_PKG}.GitTag=${GIT_TAG}'"

# Output binary name
OUTPUT=${OUTPUT:-"lab-update-esxi-cert"}

echo "Building ${OUTPUT} with version information:"
echo "  Version:    ${VERSION}"
echo "  Git Commit: ${COMMIT}"
echo "  Git Tag:    ${GIT_TAG}"
echo "  Build Date: ${BUILD_DATE}"
echo ""

# Build the application
go build -ldflags "${LDFLAGS}" -o "${OUTPUT}"

echo "Build completed successfully: ${OUTPUT}"
echo ""
echo "You can verify the version with:"
echo "  ./${OUTPUT} --version"