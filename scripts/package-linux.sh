#!/bin/sh
# Builds the desktop build (launcher, tray and CLI; cgo) for this machine's
# architecture and packages it as .deb, .rpm and Arch packages into ./dist.
# It builds natively: the launcher needs cgo and the OpenGL and X11 headers
# (the package list is in README.md, under Install). Needs nfpm.
#
#   scripts/package-linux.sh [VERSION]    # VERSION defaults to 0.0.0-snapshot
set -eu

cd "$(dirname "$0")/.."
VERSION="${1:-0.0.0-snapshot}"
GOARCH="$(go env GOARCH)"
export VERSION GOARCH

mkdir -p dist/linux
CGO_ENABLED=1 go build -trimpath -tags gui,tray,accessibility \
  -ldflags="-s -w -X github.com/njayman/proz/cmd.version=$VERSION" \
  -o dist/linux/proz .

for packager in deb rpm archlinux; do
  nfpm package --config packaging/linux/nfpm.yaml --packager "$packager" --target dist/
done
ls -l dist/proz*
