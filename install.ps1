# Install cuterm on Windows.
# Usage:
#   irm https://raw.githubusercontent.com/cuterxy/cuterm/main/install.ps1 | iex
#   .\install.ps1 -Version v1.0.0   # install a specific version
[CmdletBinding()]
param(
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"
$Repo = "cuterxy/cuterm"

$arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
if ($arch -ne "amd64") {
    Write-Error "only windows/amd64 builds are published for now (amd64 build also runs under ARM64 emulation; adjust this script to request it)"
}

if (-not $Version) {
    $release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
    $Version = $release.tag_name
}

$name = "cuterm-$Version-windows-$arch"
$url = "https://github.com/$Repo/releases/download/$Version/$name.zip"

$tmp = Join-Path $env:TEMP ([IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    Write-Host "-> downloading $url"
    Invoke-WebRequest -Uri $url -OutFile (Join-Path $tmp "cuterm.zip")
    Expand-Archive -Path (Join-Path $tmp "cuterm.zip") -DestinationPath $tmp

    $dest = Join-Path $env:LOCALAPPDATA "Programs\cuterm"
    New-Item -ItemType Directory -Force -Path $dest | Out-Null
    Move-Item -Force -Path (Join-Path $tmp "$name.exe") -Destination (Join-Path $dest "cuterm.exe")

    # Add $dest to the user PATH if missing.
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if (($userPath -split ";") -notcontains $dest) {
        [Environment]::SetEnvironmentVariable("Path", "$userPath;$dest", "User")
        $env:Path = "$env:Path;$dest"
        Write-Host "-> added $dest to your user PATH"
    }

    $verOut = & (Join-Path $dest "cuterm.exe") -version
    Write-Host "-> installed: $dest\cuterm.exe ($verOut)"
    Write-Host "run 'cuterm' to start, then open http://localhost:7681"
    Write-Host "requires Windows 10 1809 or newer (ConPTY)"
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
