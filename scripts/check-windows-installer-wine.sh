#!/bin/sh
# Builds the Windows installer on Linux and runs it under wine: install,
# upgrade to a newer version, uninstall, with the same checks as
# scripts/check-windows-installer.ps1 where wine can answer them.
#
# ponytail: wine only approximates Windows, and this packs the plain
# cgo-free build (the launcher needs cgo for Windows), so `proz gui`, the
# Start menu as Explorer shows it, Settings and the Windows PATH are only
# truly checked by the Windows job in .github/workflows/package.yml.
#
#   scripts/check-windows-installer-wine.sh
#
# Needs docker (Inno Setup 6 comes from the amake/innosetup image), a 64-bit
# wine and xvfb-run. It uses a throwaway wine prefix, never ~/.wine, and a
# virtual display, never the desktop.
set -eu

# Inno Setup needs a display even when silent.
[ -n "${PROZ_IN_XVFB:-}" ] || exec env PROZ_IN_XVFB=1 xvfb-run -a "$0" "$@"

cd "$(dirname "$0")/.."
mkdir -p dist/windows
# Two versions, so the check can upgrade from one to the other.
for version in 0.0.1-wine 0.0.2-wine; do
  CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -tags tray \
    -ldflags="-s -w -X github.com/njayman/proz/cmd.version=$version" -o dist/windows/proz.exe .
  docker run --rm -v "$PWD:/work" amake/innosetup@sha256:e003376ba818547275fe10c95e2a29be0f2d12d45e9eb8f205b6672dc5685bb1 \
    /DVersion=$version /Fproz-setup-$version packaging/windows/proz.iss >/dev/null
done

WINEPREFIX=$(mktemp -d)
export WINEPREFIX WINEDEBUG=-all
unset WAYLAND_DISPLAY # keep wine on the virtual display
trap 'wineserver -k 2>/dev/null; rm -rf "$WINEPREFIX"' EXIT
wineboot -i >/dev/null 2>&1

fail() { echo "FAIL: $*"; exit 1; }
win() { wine "$@" 2>/dev/null; }
install() { win "dist/proz-setup-$1.exe" /VERYSILENT /SUPPRESSMSGBOXES /NORESTART; wineserver -w; }
home=$(winepath -u "$(win cmd /c echo %USERPROFILE% | tr -d '\r')")
app="$home/AppData/Local/Programs/proz"
lnk="$home/AppData/Roaming/Microsoft/Windows/Start Menu/Programs/proz.lnk"
db="$home/AppData/Roaming/proz/proz.db"
key='HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\{6F4A2B1E-8C3D-4E5F-9A70-2B1C3D4E5F60}_is1'
userpath() { win reg query 'HKCU\Environment' /v Path | tr -d '\r' | sed -n 's/.*REG_EXPAND_SZ *//p'; }
onpath() { userpath | tr ';' '\n' | grep -cixF "$(winepath -w "$app")" || true; }

install 0.0.1-wine
[ -f "$app/proz.exe" ] || fail "no $app/proz.exe"
win "$app/proz.exe" --version | grep -q 0.0.1-wine || fail "proz --version"
[ -f "$lnk" ] || fail "no Start menu entry"
tr -d '\0' <"$lnk" | grep -aq gui || fail "the Start menu entry does not pass gui"
[ "$(onpath)" = 1 ] || fail "proz is not on the user PATH once: $(userpath)"
win reg query "$key" /v DisplayName | grep -q proz || fail "no uninstaller entry"
win "$app/proz.exe" list --json >/dev/null
[ -f "$db" ] || fail "proz did not create its config folder"

install 0.0.2-wine
win "$app/proz.exe" --version | grep -q 0.0.2-wine || fail "upgrading did not replace proz"
[ "$(onpath)" = 1 ] || fail "upgrading changed the PATH entry: $(userpath)"

win "$app/unins000.exe" /VERYSILENT /SUPPRESSMSGBOXES /NORESTART; wineserver -w
[ ! -e "$app" ] || fail "uninstalling left $app"
[ ! -e "$lnk" ] || fail "uninstalling left the Start menu entry"
if win reg query "$key" >/dev/null; then fail "uninstalling left the uninstaller entry"; fi
[ "$(onpath)" = 0 ] || fail "uninstalling left proz on PATH: $(userpath)"
[ -f "$db" ] || fail "uninstalling deleted the user config folder"
echo ok
