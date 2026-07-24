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
$LatestReleaseUrl = if ($env:PROOFBOARD_LATEST_RELEASE_URL) { $env:PROOFBOARD_LATEST_RELEASE_URL } else { "https://releases.proofboard.io/latest.json" }
$LatestVersion = ""
$DownloadBaseUrl = ""

try {
    $ReleaseData = Invoke-RestMethod -Uri $LatestReleaseUrl -ErrorAction Stop
    $LatestVersion = $ReleaseData.version
    $DownloadBaseUrl = $ReleaseData.url
} catch {
    Write-Warning "Failed to fetch the latest release manifest, using fallback version."
    $LatestVersion = "v1.8.14"
}

if ($LatestVersion.StartsWith("v")) {
    $ReleaseTag = $LatestVersion
} else {
    $ReleaseTag = "v$LatestVersion"
}
if ([string]::IsNullOrWhiteSpace($DownloadBaseUrl)) {
    $DownloadBaseUrl = if ($env:PROOFBOARD_DOWNLOAD_BASE_URL) { $env:PROOFBOARD_DOWNLOAD_BASE_URL } else { "https://releases.proofboard.io/$ReleaseTag" }
}
$DownloadUrl = "$DownloadBaseUrl/$BinaryName"
$ChecksumsUrl = "$DownloadBaseUrl/checksums.txt"

Write-Host "Downloading $BinaryName $LatestVersion..." -ForegroundColor Cyan
$TempBinary = Join-Path ([System.IO.Path]::GetTempPath()) "proofboard-$([guid]::NewGuid()).exe"
$TempChecksums = Join-Path ([System.IO.Path]::GetTempPath()) "proofboard-$([guid]::NewGuid()).checksums"
try {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $TempBinary -ErrorAction Stop
    Invoke-WebRequest -Uri $ChecksumsUrl -OutFile $TempChecksums -ErrorAction Stop
    $ExpectedHashLine = Get-Content $TempChecksums | Where-Object { $_ -match "[ *]$([regex]::Escape($BinaryName))$" } | Select-Object -First 1
    if (-not $ExpectedHashLine) {
        throw "Release checksums do not contain $BinaryName."
    }
    $ExpectedHash = ($ExpectedHashLine -split '\s+')[0].ToLowerInvariant()
    $ActualHash = (Get-FileHash -Algorithm SHA256 $TempBinary).Hash.ToLowerInvariant()
    if ($ActualHash -ne $ExpectedHash) {
        throw "Proofboard release checksum verification failed."
    }
    if ($env:PROOFBOARD_INSTALL_VERIFY_ONLY -eq "1") {
        Write-Host "Proofboard Career Agent $LatestVersion checksum verified." -ForegroundColor Green
        return
    }
    if (-not (Test-Path -Path $InstallDir)) {
        New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    }
    Move-Item -Force $TempBinary $ExePath
} finally {
    Remove-Item -Force -ErrorAction SilentlyContinue $TempBinary
    Remove-Item -Force -ErrorAction SilentlyContinue $TempChecksums
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
