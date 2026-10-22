# Builds the desktop build (launcher, tray and CLI; cgo) for Windows amd64
# and packs it with Inno Setup 6 into dist\proz-setup-windows-amd64.exe.
# Run it on Windows with Go, a C compiler (MinGW gcc) and Inno Setup.
#
#   pwsh scripts/package-windows.ps1 [-Version 1.2.3]
param([string]$Version = '0.0.0-snapshot')
$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

$env:CGO_ENABLED = '1'
go build -trimpath -tags gui,tray,accessibility -ldflags "-s -w -X github.com/njayman/proz/cmd.version=$Version" -o dist\windows\proz.exe .
if ($LASTEXITCODE) { exit $LASTEXITCODE }

$iscc = Join-Path ${env:ProgramFiles(x86)} 'Inno Setup 6\ISCC.exe'
if (-not (Test-Path $iscc)) { choco install innosetup -y --no-progress; if ($LASTEXITCODE) { exit $LASTEXITCODE } }
& $iscc "/DVersion=$Version" packaging\windows\proz.iss
if ($LASTEXITCODE) { exit $LASTEXITCODE }
