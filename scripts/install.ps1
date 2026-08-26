#Requires -Version 5.1
<#
    Proofboard Career Agent installation script for Windows.

    Usage:
        irm https://proofboard.io/install.ps1 | iex

    The script resolves the latest published release, verifies the release
    checksum, and then hands over to the Career Agent's own installer so the
    global executable and the background agent are registered the same way as a
    manual `proofboard install`. The install goes into the current account, so
    no administrator access is required. Set PROOFBOARD_SYSTEM_INSTALL=1 to
    install for every account instead, which prompts for administrator access
    through UAC.

    Releases are read from proofboard.io first. If that distribution origin is
    unavailable, the script falls back to the latest release published directly
    on GitHub. Private-repository fallback can use PROOFBOARD_GITHUB_TOKEN
    (GH_TOKEN and GITHUB_TOKEN are also honoured).

    Environment overrides (used by release verification and by pinned installs):
        PROOFBOARD_VERSION              install a specific tag instead of the latest
        PROOFBOARD_GITHUB_TOKEN         token used to read releases from the repository
        PROOFBOARD_LATEST_RELEASE_URL   release manifest URL
        PROOFBOARD_DOWNLOAD_BASE_URL    directory URL holding the release artifacts
        PROOFBOARD_INSTALL_VERIFY_ONLY  download and verify, then stop
        PROOFBOARD_SYSTEM_INSTALL       install for every account (needs UAC)
        PROOFBOARD_INSTALL_DIR          install into a specific directory
#>

$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$Repo = 'Proofboard-inc/proofboard-cli'
$PinnedVersion = 'v1.16.4'
$PublicDownloadHost = 'https://proofboard.io'
# ARM Windows machines exist — the Surface and the Snapdragon laptops — and
# were previously served the x64 build or refused outright. PROCESSOR_ARCHITECTURE
# reports the native architecture; a 32-bit shell on 64-bit Windows reports the
# emulated one in PROCESSOR_ARCHITEW6432, so both are consulted.
$nativeArch = $env:PROCESSOR_ARCHITEW6432
if (-not $nativeArch) { $nativeArch = $env:PROCESSOR_ARCHITECTURE }
if ($nativeArch -eq 'ARM64') {
    $BinaryName = 'Proofboard-Career-Agent-windows-arm64.exe'
} else {
    $BinaryName = 'Proofboard-Career-Agent-windows-amd64.exe'
}
$SystemInstall = $env:PROOFBOARD_SYSTEM_INSTALL -eq '1'
$InstallDir = if ($env:PROOFBOARD_INSTALL_DIR) {
    $env:PROOFBOARD_INSTALL_DIR
} elseif ($SystemInstall) {
    Join-Path $env:ProgramFiles 'Proofboard'
} else {
    Join-Path $env:LOCALAPPDATA 'Programs\Proofboard'
}
$ExePath = Join-Path $InstallDir 'proofboard.exe'
$CompletionMarker = '# Proofboard Career Agent completions'
$CompletionCommand = 'proofboard completion powershell | Out-String | Invoke-Expression'

function Test-Administrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    return ([Security.Principal.WindowsPrincipal]$identity).IsInRole(
        [Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Get-RepositoryToken {
    foreach ($candidate in @($env:PROOFBOARD_GITHUB_TOKEN, $env:GH_TOKEN, $env:GITHUB_TOKEN)) {
        if ($candidate) { return $candidate }
    }
    return $null
}

function Get-RequestHeaders {
    param([string]$Token, [string]$Accept = 'application/vnd.github+json')

    $headers = @{ 'User-Agent' = 'proofboard-installer'; 'Accept' = $Accept }
    if ($Token) { $headers['Authorization'] = "Bearer $Token" }
    return $headers
}

Write-Host 'Installing Proofboard Career Agent...' -ForegroundColor Cyan

if (-not [Environment]::Is64BitOperatingSystem) {
    throw 'Unsupported architecture. The Proofboard Career Agent requires 64-bit Windows.'
}

# Resolve the release to install. An explicit download base short-circuits every
# remote lookup so pinned and offline installs stay deterministic.
$ReleaseTag = $env:PROOFBOARD_VERSION
$Release = $null
$ReleaseSource = 'download-host'
$DownloadBaseUrl = $env:PROOFBOARD_DOWNLOAD_BASE_URL
$Token = $null

if (-not $DownloadBaseUrl) {
    $Token = Get-RepositoryToken

    # Primary source: the root-domain release manifest.
    if (-not $ReleaseTag) {
        $manifestUrl = if ($env:PROOFBOARD_LATEST_RELEASE_URL) {
            $env:PROOFBOARD_LATEST_RELEASE_URL
        } else {
            "$PublicDownloadHost/latest.json"
        }
        try {
            $manifest = Invoke-RestMethod -Uri $manifestUrl -ErrorAction Stop
            if ($manifest.version) { $ReleaseTag = $manifest.version }
            if ($manifest.url) { $DownloadBaseUrl = $manifest.url }
        } catch {
            Write-Verbose "Could not read the release manifest from proofboard.io: $_"
        }
    }

    if (-not $ReleaseTag) {
        # Fallback source: the latest release published directly on GitHub.
        try {
            $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" `
                -Headers (Get-RequestHeaders -Token $Token) -ErrorAction Stop
            if ($Release.tag_name) {
                $ReleaseTag = $Release.tag_name
                $ReleaseSource = 'repository'
            }
        } catch {
            Write-Verbose "Could not read the latest GitHub release: $_"
        }
    }
}

if (-not $ReleaseTag) {
    Write-Warning "Could not resolve the latest release; falling back to $PinnedVersion."
    $ReleaseTag = $PinnedVersion
}
if (-not $ReleaseTag.StartsWith('v')) {
    $ReleaseTag = "v$ReleaseTag"
}
if ($ReleaseSource -ne 'repository' -and -not $DownloadBaseUrl) {
    $DownloadBaseUrl = "$PublicDownloadHost/$ReleaseTag"
}

function Save-ReleaseAsset {
    param([string]$Name, [string]$Destination)

    if ($ReleaseSource -ne 'repository') {
        try {
            Invoke-WebRequest -Uri "$DownloadBaseUrl/$Name" -OutFile $Destination -UseBasicParsing -ErrorAction Stop
            return
        } catch {
            Write-Verbose "Could not download $Name from proofboard.io: $_"
        }
    }

    # Root-domain asset fallback: resolve the matching GitHub release and
    # download the asset directly from GitHub.
    if (-not $Release) {
        $releasePath = if ($ReleaseTag) { "tags/$ReleaseTag" } else { 'latest' }
        $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/$releasePath" `
            -Headers (Get-RequestHeaders -Token $Token) -ErrorAction Stop
    }
    $asset = $Release.assets | Where-Object { $_.name -eq $Name } | Select-Object -First 1
    if (-not $asset) {
        throw "Neither proofboard.io nor the GitHub release $ReleaseTag contains $Name."
    }
    Invoke-WebRequest -Uri "https://api.github.com/repos/$Repo/releases/assets/$($asset.id)" `
        -OutFile $Destination -UseBasicParsing `
        -Headers (Get-RequestHeaders -Token $Token -Accept 'application/octet-stream') -ErrorAction Stop
}

Write-Host "Downloading $BinaryName $ReleaseTag..." -ForegroundColor Cyan
$TempBinary = Join-Path ([System.IO.Path]::GetTempPath()) "proofboard-$([guid]::NewGuid()).exe"
$TempChecksums = Join-Path ([System.IO.Path]::GetTempPath()) "proofboard-$([guid]::NewGuid()).checksums"
$Installed = $false
try {
    Save-ReleaseAsset -Name $BinaryName -Destination $TempBinary
    Save-ReleaseAsset -Name 'checksums.txt' -Destination $TempChecksums

    $ExpectedHashLine = Get-Content $TempChecksums |
        Where-Object { $_ -match "[ *]$([regex]::Escape($BinaryName))$" } |
        Select-Object -First 1
    if (-not $ExpectedHashLine) {
        throw "Release checksums do not contain $BinaryName."
    }
    $ExpectedHash = ($ExpectedHashLine -split '\s+')[0].ToLowerInvariant()
    $ActualHash = (Get-FileHash -Algorithm SHA256 $TempBinary).Hash.ToLowerInvariant()
    if ($ActualHash -ne $ExpectedHash) {
        throw 'Proofboard release checksum verification failed.'
    }

    if ($env:PROOFBOARD_INSTALL_VERIFY_ONLY -eq '1') {
        Write-Host "Proofboard Career Agent $ReleaseTag checksum verified." -ForegroundColor Green
        return
    }

    # Install (or replace) the executable and register the background agent.
    # This installs into the current account and needs no administrator access,
    # which keeps the Career Agent installable on managed machines where
    # administrator details are not handed out. A machine-wide install stays
    # available for anyone who wants it.
    if ($SystemInstall -and -not (Test-Administrator)) {
        Write-Host "A machine-wide install needs administrator access." -ForegroundColor Yellow
        try {
            $process = Start-Process -FilePath $TempBinary -ArgumentList 'install', '--system' `
                -Verb RunAs -Wait -PassThru -ErrorAction Stop
        } catch {
            throw 'Administrator access was not granted. Unset PROOFBOARD_SYSTEM_INSTALL to install into your own account instead.'
        }
        if ($process.ExitCode -ne 0) {
            throw "Proofboard Career Agent installation failed with exit code $($process.ExitCode)."
        }
        # The background agent belongs to the signed-in user, not to the
        # administrator account that performed the machine-wide install.
        & $ExePath agent enable
    } else {
        $arguments = @('install')
        if ($SystemInstall) { $arguments += '--system' }
        & $TempBinary @arguments
        if ($LASTEXITCODE -ne 0) {
            throw "Proofboard Career Agent installation failed with exit code $LASTEXITCODE."
        }
    }
    $Installed = $true
} finally {
    Remove-Item -Force -ErrorAction SilentlyContinue $TempBinary
    Remove-Item -Force -ErrorAction SilentlyContinue $TempChecksums
}

if (-not $Installed) {
    return
}

# Make the executable resolvable in this session too, since the machine PATH
# entry added by the installer only reaches newly started processes.
if ($env:Path -notmatch [regex]::Escape($InstallDir)) {
    $env:Path = "$env:Path;$InstallDir"
}

# Install (or refresh) shell completions for the signed-in user. The completion
# script is regenerated on each session start, so it always matches the
# installed Career Agent. This is a convenience step: a failure here must not
# fail the installation.
try {
    & $ExePath completion powershell | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "completion generation exited with code $LASTEXITCODE"
    }

    $profilePath = $PROFILE.CurrentUserAllHosts
    $profileDir = Split-Path -Parent $profilePath
    if (-not (Test-Path $profileDir)) {
        New-Item -ItemType Directory -Force -Path $profileDir | Out-Null
    }

    $existing = if (Test-Path $profilePath) { Get-Content $profilePath -Raw } else { '' }
    if ($existing -notmatch [regex]::Escape($CompletionMarker)) {
        Add-Content -Path $profilePath -Value "`n$CompletionMarker`n$CompletionCommand`n"
        Write-Host "Shell completions installed to $profilePath." -ForegroundColor Green
    } else {
        Write-Host "Shell completions are already installed in $profilePath." -ForegroundColor Green
    }
} catch {
    Write-Warning "Shell completions could not be installed automatically: $_"
    Write-Warning 'Run: proofboard completion powershell'
}

# Connect the Career Agent straight away, so opening a project is all that is
# left to do. An existing connection is kept, which is what makes re-running
# this script an update rather than a fresh sign-in.
$credentials = Join-Path $env:USERPROFILE '.proofboard\credentials.json'
if (Test-Path $credentials) {
    Write-Host 'Career Agent is already connected.' -ForegroundColor Green
} else {
    & $ExePath auth
    if ($LASTEXITCODE -ne 0) {
        Write-Warning "Run 'proofboard auth' when you are ready to connect your Career Agent."
    }
}

# Pick up the project this was installed from, so a workspace that is already
# open is detected without waiting for the next terminal.
& $ExePath detect 2>&1 | Out-Null

Write-Host 'Proofboard Career Agent installed and running. Keep building software; Proofboard will handle the rest.' -ForegroundColor Green
