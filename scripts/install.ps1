$ErrorActionPreference = 'Stop'

Write-Host "Installing Proofboard Career Agent..." -ForegroundColor Cyan

$Arch = (Get-WmiObject Win32_OperatingSystem).OSArchitecture
if ($Arch -ne "64-bit") {
    Write-Error "Unsupported architecture: $Arch. Only 64-bit is supported."
    exit 1
}

$BinaryName = "proofboard-windows-amd64.exe"
$InstallDir = "$env:ProgramFiles\Proofboard"
$ExePath = "$InstallDir\proofboard.exe"

# Determine latest version
$LatestReleaseUrl = "https://releases.proofboard.io/latest.json"
$LatestVersion = ""

try {
    $ReleaseData = Invoke-RestMethod -Uri $LatestReleaseUrl -ErrorAction Stop
    $LatestVersion = $ReleaseData.version
} catch {
    Write-Warning "Failed to fetch from releases.proofboard.io, using fallback version."
    $LatestVersion = "v1.8.10"
}

$DownloadUrl = "https://releases.proofboard.io/$LatestVersion/$BinaryName"
$GithubFallbackUrl = "https://github.com/Proofboard-inc/proofboard-cli/releases/download/$LatestVersion/$BinaryName"

# Create installation directory if it doesn't exist
if (-not (Test-Path -Path $InstallDir)) {
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
}

Write-Host "Downloading $BinaryName $LatestVersion..." -ForegroundColor Cyan
try {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $ExePath -ErrorAction Stop
} catch {
    Write-Warning "Download from releases.proofboard.io failed, falling back to GitHub..."
    Invoke-WebRequest -Uri $GithubFallbackUrl -OutFile $ExePath
}

# Add to PATH if not already present
$CurrentPath = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::Machine)
if ($CurrentPath -notmatch [regex]::Escape($InstallDir)) {
    Write-Host "Adding $InstallDir to system PATH..." -ForegroundColor Cyan
    $NewPath = $CurrentPath + ";$InstallDir"
    [Environment]::SetEnvironmentVariable("Path", $NewPath, [EnvironmentVariableTarget]::Machine)
}

& $ExePath agent enable
if ($LASTEXITCODE -ne 0) {
    throw "Proofboard Career Agent could not be registered."
}

Write-Host "Proofboard Career Agent installed and running. Keep building software; Proofboard will handle the rest." -ForegroundColor Green
