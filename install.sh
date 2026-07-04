#!/bin/sh
set -eu

REPO="njayman/proz"
BIN_DIR="${HOME}/.local/bin"
PROZ_BIN="${BIN_DIR}/proz"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
  linux) ;;
  darwin) ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

URL="https://github.com/$REPO/releases/latest/download/proz-${OS}-${ARCH}.tar.gz"

mkdir -p "$BIN_DIR"

echo "Downloading proz for $OS-$ARCH..."
if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$URL" -o /tmp/proz.tar.gz
elif command -v wget >/dev/null 2>&1; then
  wget -q "$URL" -O /tmp/proz.tar.gz
else
  echo "Need curl or wget"
  exit 1
fi

tar -xzf /tmp/proz.tar.gz -C /tmp/
cp /tmp/proz "$PROZ_BIN"
chmod +x "$PROZ_BIN"
rm -f /tmp/proz /tmp/proz.tar.gz

echo "Installed proz to $PROZ_BIN"
echo ""
echo "Add shell completions (one line in your rc file):"
echo '  source <(proz completion)'
