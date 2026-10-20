# Installs the Windows installer silently for the current user, checks what
# a user would see, installs it again over itself (an upgrade), uninstalls it
# silently and checks nothing is left but the user's config folder.
#
#   pwsh scripts/check-windows-installer.ps1 dist\proz-setup-windows-amd64.exe
#
# Run it on a clean Windows machine (CI): it installs proz for real.
param([Parameter(Mandatory)][string]$Installer)
$ErrorActionPreference = 'Stop'

function Fail($msg) { Write-Host "FAIL: $msg"; exit 1 }
function Install { Start-Process -Wait -FilePath $Installer -ArgumentList '/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART', '/CURRENTUSER' }
function UserPath { [Environment]::GetEnvironmentVariable('Path', 'User') }
function OnPath { @((UserPath) -split ';' | Where-Object { $_ -eq $app }).Count }

$app = Join-Path $env:LOCALAPPDATA 'Programs\proz'
$exe = Join-Path $app 'proz.exe'
$lnk = Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs\proz.lnk'
$uninstallKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\{6F4A2B1E-8C3D-4E5F-9A70-2B1C3D4E5F60}_is1'
$db = Join-Path $env:APPDATA 'proz\proz.db'

Install
if (-not (Test-Path $exe)) { Fail "no $exe" }
& $exe --version
if ($LASTEXITCODE -ne 0) { Fail 'proz --version' }
if (-not (& $exe help gui | Select-String -SimpleMatch 'proz gui')) { Fail 'this proz has no gui command' }
if (& $exe help gui | Select-String -SimpleMatch 'desktop builds only') { Fail 'proz gui is the plain build, not the desktop build' }
if (-not (Test-Path $lnk)) { Fail "no Start menu entry at $lnk" }
$shortcut = (New-Object -ComObject WScript.Shell).CreateShortcut($lnk)
if ($shortcut.TargetPath -ne $exe -or $shortcut.Arguments -ne 'gui') { Fail "the Start menu entry runs '$($shortcut.TargetPath) $($shortcut.Arguments)', not '$exe gui'" }
if ((OnPath) -ne 1) { Fail "$app is not on the user's PATH once: $(UserPath)" }
if (-not (Test-Path $uninstallKey)) { Fail 'no uninstaller in Settings' }
if ((Get-ItemProperty $uninstallKey).DisplayName -ne 'proz') { Fail 'the uninstaller is not named proz' }

# Leave the user something to keep: proz creates its database on first use.
& $exe list --json | Out-Null
if (-not (Test-Path $db)) { Fail 'proz did not create its config folder' }

Install # an upgrade, over the running install
& $exe --version | Out-Null
if ($LASTEXITCODE -ne 0) { Fail 'proz --version after upgrading' }
if ((OnPath) -ne 1) { Fail "upgrading changed the PATH entry: $(UserPath)" }

$uninstaller = (Get-ItemProperty $uninstallKey).UninstallString.Trim('"')
Start-Process -Wait -FilePath $uninstaller -ArgumentList '/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART'
# The uninstaller hands off to a copy of itself and returns at once.
for ($i = 0; $i -lt 60 -and (Test-Path $app); $i++) { Start-Sleep -Seconds 1 }
if (Test-Path $app) { Fail "uninstalling left $app" }
if (Test-Path $lnk) { Fail 'uninstalling left the Start menu entry' }
if (Test-Path $uninstallKey) { Fail 'uninstalling left the Settings entry' }
if ((OnPath) -ne 0) { Fail "uninstalling left $app on PATH" }
if (-not (Test-Path $db)) { Fail "uninstalling deleted the user's config folder" }
Write-Host 'ok'
