#!/bin/bash

set -e

PROJECT_NAME="proz"

OUTPUT_DIR="./bin"

mkdir -p "${OUTPUT_DIR}"

echo "Building ${PROJECT_NAME} for multiple platforms..."

echo "Building for macOS (AMD64)..."
env GOOS=darwin GOARCH=amd64 go build -o "${OUTPUT_DIR}/${PROJECT_NAME}-darwin-amd64" .
echo "Building for macOS (ARM64 - Apple Silicon)..."
env GOOS=darwin GOARCH=arm64 go build -o "${OUTPUT_DIR}/${PROJECT_NAME}-darwin-arm64" .

echo "Building for Windows (AMD64)..."
env GOOS=windows GOARCH=amd64 go build -o "${OUTPUT_DIR}/${PROJECT_NAME}-windows-amd64.exe" .
echo "Building for Windows (386 - 32-bit)..."
env GOOS=windows GOARCH=386 go build -o "${OUTPUT_DIR}/${PROJECT_NAME}-windows-386.exe" .

echo "Building for Linux (AMD64)..."
env GOOS=linux GOARCH=amd64 go build -o "${OUTPUT_DIR}/${PROJECT_NAME}-linux-amd64" .
echo "Building for Linux (ARM64)..."
env GOOS=linux GOARCH=arm64 go build -o "${OUTPUT_DIR}/${PROJECT_NAME}-linux-arm64" .
echo "Building for Linux (ARMv7 - commonly used on Raspberry Pi)..."
env GOOS=linux GOARCH=arm GOARM=7 go build -o "${OUTPUT_DIR}/${PROJECT_NAME}-linux-armv7" .


echo "All builds complete! Binaries are in the '${OUTPUT_DIR}' directory."
ls -l "${OUTPUT_DIR}"
