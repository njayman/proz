#!/bin/bash

set -e

PROJECT_NAME="proz"

OUTPUT_DIR="./bin"
LDFLAGS="-s -w"
# The tray builds without cgo on Linux and Windows; macOS needs cgo, so a
# macOS tray build only works on a Mac. Elsewhere darwin builds skip the tray.
DARWIN_TAGS=""
DARWIN_CGO=0
if [ "$(uname)" = "Darwin" ]; then DARWIN_TAGS="tray"; DARWIN_CGO=1; fi

mkdir -p "${OUTPUT_DIR}"

echo "Building ${PROJECT_NAME} for multiple platforms..."

echo "Building for macOS (AMD64)..."
env CGO_ENABLED=${DARWIN_CGO} GOOS=darwin GOARCH=amd64 go build -trimpath -tags "${DARWIN_TAGS}" -ldflags="${LDFLAGS}" -o "${OUTPUT_DIR}/${PROJECT_NAME}-darwin-amd64" .
echo "Building for macOS (ARM64 - Apple Silicon)..."
env CGO_ENABLED=${DARWIN_CGO} GOOS=darwin GOARCH=arm64 go build -trimpath -tags "${DARWIN_TAGS}" -ldflags="${LDFLAGS}" -o "${OUTPUT_DIR}/${PROJECT_NAME}-darwin-arm64" .

echo "Building for Windows (AMD64)..."
env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -tags tray -ldflags="${LDFLAGS}" -o "${OUTPUT_DIR}/${PROJECT_NAME}-windows-amd64.exe" .
echo "Building for Windows (386 - 32-bit)..."
env CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -trimpath -tags tray -ldflags="${LDFLAGS}" -o "${OUTPUT_DIR}/${PROJECT_NAME}-windows-386.exe" .

echo "Building for Linux (AMD64)..."
env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -tags tray -ldflags="${LDFLAGS}" -o "${OUTPUT_DIR}/${PROJECT_NAME}-linux-amd64" .
echo "Building for Linux (ARM64)..."
env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -tags tray -ldflags="${LDFLAGS}" -o "${OUTPUT_DIR}/${PROJECT_NAME}-linux-arm64" .
echo "Building for Linux (ARMv7 - commonly used on Raspberry Pi)..."
env CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -tags tray -ldflags="${LDFLAGS}" -o "${OUTPUT_DIR}/${PROJECT_NAME}-linux-armv7" .


echo "All builds complete! Binaries are in the '${OUTPUT_DIR}' directory."
ls -l "${OUTPUT_DIR}"
