#!/bin/sh
# Installs each Linux package in a clean container of its distro and checks
# what a user would see: proz on PATH, `proz --version`, the desktop build's
# `proz gui`, a valid app menu entry and icon, and that removing the package
# leaves nothing behind but the user's config folder.
#
#   scripts/check-linux-package.sh dist/proz_*.deb dist/proz_*.rpm dist/proz_*.pkg.tar.zst
#
# Needs docker. Installs nothing on the host.
set -eu

[ $# -gt 0 ] || { echo "usage: $0 PACKAGE..." >&2; exit 2; }

# The checks that run inside the container, as root. $1 is the package.
inner='
set -eu
pkg="$1"
case "$pkg" in
  *.deb)
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq >/dev/null
    apt-get install -y -qq desktop-file-utils >/dev/null
    apt-get install -y -qq "$pkg" >/dev/null
    files=$(dpkg -L proz); remove="apt-get purge -y -qq proz" ;;
  *.rpm)
    dnf install -y -q desktop-file-utils "$pkg" >/dev/null
    files=$(rpm -ql proz); remove="dnf remove -y -q proz" ;;
  *.pkg.tar.zst)
    pacman -Syu --noconfirm --needed desktop-file-utils >/dev/null
    pacman -U --noconfirm "$pkg" >/dev/null
    files=$(pacman -Qlq proz); remove="pacman -R --noconfirm proz" ;;
esac
fail() { echo "FAIL: $*"; exit 1; }

[ -x /usr/bin/proz ] || fail "no /usr/bin/proz"
command -v proz >/dev/null || fail "proz is not on PATH"
proz --version >/dev/null || fail "proz --version"
proz help gui | grep -q "proz gui" || fail "this proz has no gui command"
if proz help gui | grep -q "desktop builds only"; then fail "proz gui is the plain build, not the desktop build"; fi
entry=/usr/share/applications/proz.desktop
[ -f "$entry" ] || fail "no app menu entry at $entry"
desktop-file-validate "$entry" || fail "desktop-file-validate $entry"
grep -qx "Exec=proz gui" "$entry" || fail "the app menu entry does not run proz gui"
[ -f /usr/share/icons/hicolor/scalable/apps/proz.svg ] || fail "no icon"

# Leave the user something to keep: proz creates its database on first use.
proz list --json >/dev/null
[ -f "$HOME/.config/proz/proz.db" ] || fail "proz did not create its config folder"

$remove >/dev/null || fail "removing the package"
for f in $files; do
  if [ -e "$f" ] && [ ! -d "$f" ]; then fail "removing the package left $f"; fi
done
[ -f "$HOME/.config/proz/proz.db" ] || fail "removing the package deleted the user config folder"
echo "ok"
'

for pkg in "$@"; do
  case "$pkg" in
    *.deb) images="debian:13 ubuntu:24.04" ;;
    *.rpm) images="fedora:43" ;;
    *.pkg.tar.zst)
      # ponytail: Arch publishes amd64 images only, so arm64 Arch packages
      # are built but not install-checked until an arm64 Arch image is trusted.
      case "$pkg" in *aarch64*) echo "SKIP $pkg: no official arm64 Arch image"; continue ;; esac
      images="archlinux:base" ;;
    *) echo "unknown package type: $pkg" >&2; exit 2 ;;
  esac
  [ -f "$pkg" ] || { echo "FAIL: $pkg does not exist" >&2; exit 1; }
  dir=$(cd "$(dirname "$pkg")" && pwd)
  name=$(basename "$pkg")
  for image in $images; do
    echo "== $name on $image"
    docker run --rm -v "$dir:/pkg:ro" "$image" sh -c "$inner" check "/pkg/$name"
  done
done
