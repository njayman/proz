#Requires -Version 5.1
param(
    [string]$InstallDir = "$env:USERPROFILE\.local\bin"
)

$Repo = "njayman/proz"
$Arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { throw "32-bit not supported" }
$Url = "https://github.com/$Repo/releases/latest/download/proz-windows-$Arch.zip"
$Bin = "$InstallDir\proz.exe"

New-Object -TypeName System.IO.DirectoryInfo -ArgumentList $InstallDir | ForEach-Object {
    if (-not $_.Exists) { $_.Create() }
}

Write-Host "Downloading proz for windows-$Arch..."
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
Invoke-WebRequest -Uri $Url -OutFile "$env:TEMP\proz.zip"

Expand-Archive -Path "$env:TEMP\proz.zip" -DestinationPath "$env:TEMP\proz_install" -Force
Copy-Item "$env:TEMP\proz_install\proz.exe" $Bin -Force
Remove-Item "$env:TEMP\proz.zip" -Force -ErrorAction SilentlyContinue
Remove-Item "$env:TEMP\proz_install" -Recurse -Force -ErrorAction SilentlyContinue

Write-Host "Installed proz to $Bin"

$CurrentPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($CurrentPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$CurrentPath;$InstallDir", "User")
    Write-Host "Added $InstallDir to user PATH"
    Write-Host "  Restart your shell for the change to take effect"
}

Write-Host ""
Write-Host "Done. Run 'proz' to list projects."
