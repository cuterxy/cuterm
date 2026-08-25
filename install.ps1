# Install cuterm / cuterm-hub on Windows.
# Usage:
#   irm https://raw.githubusercontent.com/cuterxy/cuterm/main/install.ps1 | iex
#   .\install.ps1 -App cuterm-hub     # install cuterm-hub instead of cuterm
#   .\install.ps1 -Version v1.0.0     # install a specific version
[CmdletBinding()]
param(
    [string]$App = "cuterm",
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"
$Repo = "cuterxy/cuterm"

if ($App -notin @("cuterm", "cuterm-hub")) {
    Write-Error "unknown app: $App (expected cuterm or cuterm-hub)"
}

$arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
if ($arch -ne "amd64") {
    Write-Error "only windows/amd64 builds are published for now (amd64 build also runs under ARM64 emulation; adjust this script to request it)"
}

if (-not $Version) {
    $release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
    $Version = $release.tag_name
}

$name = "$App-$Version-windows-$arch"
$url = "https://github.com/$Repo/releases/download/$Version/$name.zip"

$tmp = Join-Path $env:TEMP ([IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    Write-Host "-> downloading $url"
    Invoke-WebRequest -Uri $url -OutFile (Join-Path $tmp "$App.zip")
    Expand-Archive -Path (Join-Path $tmp "$App.zip") -DestinationPath $tmp

    $dest = Join-Path $env:LOCALAPPDATA "Programs\$App"
    New-Item -ItemType Directory -Force -Path $dest | Out-Null
    Move-Item -Force -Path (Join-Path $tmp "$name.exe") -Destination (Join-Path $dest "$App.exe")

    # Add $dest to the user PATH if missing.
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if (($userPath -split ";") -notcontains $dest) {
        [Environment]::SetEnvironmentVariable("Path", "$userPath;$dest", "User")
        $env:Path = "$env:Path;$dest"
        Write-Host "-> added $dest to your user PATH"
    }

    $verOut = & (Join-Path $dest "$App.exe") -version
    Write-Host "-> installed: $dest\$App.exe ($verOut)"
    $port = if ($App -eq "cuterm-hub") { 7682 } else { 7681 }
    Write-Host "run '$App' to start, then open http://localhost:$port"
    if ($App -eq "cuterm") {
        Write-Host "requires Windows 10 1809 or newer (ConPTY)"
    }
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
