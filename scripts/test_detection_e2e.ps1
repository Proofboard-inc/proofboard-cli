# Reproduce auto project detection on Windows the way a developer experiences
# it, across the installation methods that actually ship and the shells people
# actually use.
#
# Why this is parameterised rather than testing one path: the MSI and the Inno
# setup.exe do not run `proofboard install`. Both run `agent enable` as their
# post-install action (packaging/windows/proofboard.wxs, ProofboardCareerAgent.iss),
# so whether a Windows user who installed through either of those ever gets a
# working cd hook depends on a completely different code path from the one
# `proofboard install` exercises. And Windows PowerShell 5.1 reads a different
# profile from PowerShell 7 (Documents\WindowsPowerShell vs Documents\PowerShell),
# so a hook written for one says nothing about the other.
#
#   -Method install       what `proofboard install` does
#   -Method agent-enable  what the MSI and setup.exe do
#   -Shell  pwsh          PowerShell 7
#   -Shell  powershell    Windows PowerShell 5.1
param(
    [Parameter(Mandatory = $true)][string]$Binary,
    [ValidateSet('install', 'agent-enable')][string]$Method = 'install',
    [ValidateSet('pwsh', 'powershell')][string]$Shell = 'pwsh'
)

$ErrorActionPreference = 'Stop'
$pass = 0
$fail = 0
function Report-Pass($m) { Write-Output "  ok   [$Method/$Shell] $m"; $script:pass++ }
function Report-Fail($m) { Write-Output "  FAIL [$Method/$Shell] $m"; $script:fail++ }

if (-not (Get-Command $Shell -ErrorAction SilentlyContinue)) {
    Write-Output "  skip [$Method/$Shell] shell not installed"
    exit 0
}

$work = Join-Path ([IO.Path]::GetTempPath()) ("pbdetect-" + [Guid]::NewGuid().ToString('N'))
$homeDir = Join-Path $work 'home'
$repoDir = Join-Path $work 'repo'
New-Item -ItemType Directory -Force -Path $homeDir, $repoDir | Out-Null

try {
    git -C $repoDir init -q
    git -C $repoDir config user.email detection@proofboard.io
    git -C $repoDir config user.name "Detection Test"
    git -C $repoDir remote add origin https://github.com/acme/detection-probe.git
    Set-Content -Path (Join-Path $repoDir 'probe.txt') -Value 'probe'
    git -C $repoDir add probe.txt
    git -C $repoDir commit -qm probe

    # HOME and USERPROFILE because Go reads USERPROFILE here and the shell reads
    # HOME; LOCALAPPDATA because the installer writes the executable under it,
    # and leaving it at the real profile installs outside the scratch directory.
    $env:HOME = $homeDir
    $env:USERPROFILE = $homeDir
    $env:LOCALAPPDATA = Join-Path $homeDir 'AppData\Local'
    New-Item -ItemType Directory -Force -Path $env:LOCALAPPDATA | Out-Null
    $env:PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS = '1'

    if ($Method -eq 'install') {
        & $Binary install *> (Join-Path $work 'install.log')
        $installOk = ($LASTEXITCODE -eq 0)
        $binDir = Join-Path $env:LOCALAPPDATA 'Programs\Proofboard'
    } else {
        # Stand in for the MSI / setup.exe: they place the executable somewhere
        # of their own choosing and then run `agent enable`. Nothing about that
        # path goes through `proofboard install`.
        $binDir = Join-Path $work 'Program Files\Proofboard'
        New-Item -ItemType Directory -Force -Path $binDir | Out-Null
        Copy-Item $Binary (Join-Path $binDir 'proofboard.exe')
        $env:PATH = "$binDir;$env:PATH"
        & (Join-Path $binDir 'proofboard.exe') agent enable *> (Join-Path $work 'install.log')
        $installOk = ($LASTEXITCODE -eq 0)
    }

    if (-not $installOk) {
        Report-Fail "$Method failed"
        Get-Content (Join-Path $work 'install.log') -ErrorAction SilentlyContinue |
            Select-Object -First 15 | ForEach-Object { Write-Output "      $_" }
    }

    $exe = Join-Path $binDir 'proofboard.exe'
    if (Test-Path $exe) {
        Report-Pass "executable present in $binDir"
    } else {
        Report-Fail "no proofboard.exe in $binDir"
    }
    $env:PATH = "$binDir;$env:PATH"

    # The profile this shell will actually read. 5.1 and 7 differ, and a hook
    # in the other one does this shell no good at all.
    $profileDir = if ($Shell -eq 'pwsh') { 'PowerShell' } else { 'WindowsPowerShell' }
    $profilePath = Join-Path $homeDir "Documents\$profileDir\Microsoft.PowerShell_profile.ps1"

    if ((Test-Path $profilePath) -and
        ((Get-Content $profilePath -Raw) -match '_proofboardLastRoot')) {
        Report-Pass "directory-change hook written to the $profileDir profile"
    } else {
        Report-Fail "no directory-change hook in $profilePath"
    }

    # Commands on stdin so the shell draws a prompt between each, which is what
    # runs the wrapped prompt function the hook installs.
    $inputFile = Join-Path $work 'input.txt'
    @("cd '$repoDir'", "cd '$work'", "cd '$repoDir'", 'exit') -join "`n" |
        Set-Content -Path $inputFile -Encoding ascii

    $outFile = Join-Path $work 'out.txt'
    Get-Content $inputFile | & $Shell -NoLogo *> $outFile
    $output = if (Test-Path $outFile) { Get-Content $outFile -Raw } else { '' }

    if ($output -match 'New repository detected') {
        Report-Pass 'cd into an unconnected repository prompts'
    } else {
        Report-Fail 'cd produced no detection prompt'
        Write-Output '      --- shell output ---'
        ($output -split "`n" | Select-Object -First 15) | ForEach-Object { Write-Output "      $_" }
    }
} finally {
    Remove-Item -Recurse -Force $work -ErrorAction SilentlyContinue
}

Write-Output "  $pass passed, $fail failed"
if ($fail -gt 0) { exit 1 }
