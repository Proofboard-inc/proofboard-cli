# Reproduce auto project detection on Windows the way a developer experiences
# it: install the CLI, start a real interactive PowerShell, cd into a
# repository that is not connected, and check that the prompt appears.
#
# PowerShell has no chpwd event, so the hook wraps the prompt function. That
# means the same constraint as bash: the shell has to actually draw a prompt,
# which it only does when commands arrive on stdin rather than through -Command.
param(
    [Parameter(Mandatory = $true)][string]$Binary
)

$ErrorActionPreference = 'Stop'
$pass = 0
$fail = 0

function Report-Pass($m) { Write-Output "  ok   $m"; $script:pass++ }
function Report-Fail($m) { Write-Output "  FAIL $m"; $script:fail++ }

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

    # Both are set because Go reads USERPROFILE here and the shell reads HOME.
    $env:HOME = $homeDir
    $env:USERPROFILE = $homeDir
    $env:PROOFBOARD_DISABLE_DESKTOP_NOTIFICATIONS = '1'

    & $Binary install *> (Join-Path $work 'install.log')
    if ($LASTEXITCODE -ne 0) {
        Report-Fail 'proofboard install failed'
        Get-Content (Join-Path $work 'install.log') | ForEach-Object { Write-Output "      $_" }
    } else {
        $hookFound = $false
        Get-ChildItem -Path $homeDir -Recurse -File -ErrorAction SilentlyContinue | ForEach-Object {
            if ((Get-Content $_.FullName -Raw -ErrorAction SilentlyContinue) -match '_proofboardLastRoot|_proofboard_chpwd') {
                $hookFound = $true
            }
        }
        if ($hookFound) {
            Report-Pass 'directory-change hook installed'
        } else {
            Report-Fail 'no directory-change hook written to the PowerShell profile'
        }

        # Commands on stdin so the shell draws a prompt between each one, which
        # is what runs the wrapped prompt function the hook installs.
        $inputFile = Join-Path $work 'input.txt'
        @(
            "cd '$repoDir'"
            "cd '$($work)'"
            "cd '$repoDir'"
            'exit'
        ) -join "`n" | Set-Content -Path $inputFile -Encoding ascii

        $outFile = Join-Path $work 'out.txt'
        $binDir = Join-Path $homeDir '.local\bin'
        $env:PATH = "$binDir;$env:PATH"
        Get-Content $inputFile | & pwsh -NoLogo -NoExit *> $outFile
        $output = if (Test-Path $outFile) { Get-Content $outFile -Raw } else { '' }

        if ($output -match 'New repository detected') {
            Report-Pass 'cd into an unconnected repository prompts'
        } else {
            Report-Fail 'cd produced no detection prompt'
            Write-Output '      --- shell output ---'
            ($output -split "`n" | Select-Object -First 20) | ForEach-Object { Write-Output "      $_" }
        }
    }
} finally {
    Remove-Item -Recurse -Force $work -ErrorAction SilentlyContinue
}

Write-Output "  $pass passed, $fail failed"
if ($fail -gt 0) { exit 1 }
