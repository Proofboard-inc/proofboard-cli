@echo off
setlocal enabledelayedexpansion

rem Proofboard Career Agent installation launcher for Windows.
rem
rem Double-click this file, or run it from a command prompt. It runs the
rem PowerShell installer sitting next to it when present, and otherwise
rem downloads that installer from proofboard.io, falling back to the latest
rem release published directly on GitHub. The PowerShell installer requests
rem administrator access through UAC when it needs to write into Program Files.
rem
rem Private-repository fallback can use PROOFBOARD_GITHUB_TOKEN (GH_TOKEN and
rem GITHUB_TOKEN are also honoured).

set "PROOFBOARD_REPO=Proofboard-inc/proofboard-cli"
set "PROOFBOARD_INSTALLER=%~dp0install.ps1"
set "DOWNLOADED_INSTALLER="
set "PROOFBOARD_EXIT=0"

rem Pause before closing when the window was opened by double-clicking.
set "PROOFBOARD_PAUSE=0"
echo %cmdcmdline% | find /i "%~nx0" >nul 2>&1 && set "PROOFBOARD_PAUSE=1"

where powershell >nul 2>&1
if errorlevel 1 (
    echo Windows PowerShell is required to install the Proofboard Career Agent.
    set "PROOFBOARD_EXIT=1"
    goto :finish
)

if exist "%PROOFBOARD_INSTALLER%" goto :run

echo Downloading the Proofboard Career Agent installer...
set "DOWNLOADED_INSTALLER=%TEMP%\proofboard-install-%RANDOM%%RANDOM%.ps1"
set "PROOFBOARD_INSTALLER=%DOWNLOADED_INSTALLER%"

rem Resolve the installer the same way install.ps1 resolves the executable: an
rem explicit download base wins, then the public root domain, then the latest
rem GitHub release.
powershell -NoProfile -ExecutionPolicy Bypass -Command "$ErrorActionPreference='Stop'; [Net.ServicePointManager]::SecurityProtocol=[Net.SecurityProtocolType]::Tls12; $headers=@{'User-Agent'='proofboard-installer'}; $token=$env:PROOFBOARD_GITHUB_TOKEN; if(-not $token){$token=$env:GH_TOKEN}; if(-not $token){$token=$env:GITHUB_TOKEN}; if($token){$headers['Authorization']='Bearer '+$token}; if($env:PROOFBOARD_DOWNLOAD_BASE_URL){Invoke-WebRequest -Uri ($env:PROOFBOARD_DOWNLOAD_BASE_URL.TrimEnd('/')+'/install.ps1') -OutFile $env:PROOFBOARD_INSTALLER -UseBasicParsing; exit 0}; $tag=$env:PROOFBOARD_VERSION; if($tag -and -not $tag.StartsWith('v')){$tag='v'+$tag}; $primary=if($tag){'https://proofboard.io/'+$tag+'/install.ps1'}else{'https://proofboard.io/install.ps1'}; try{Invoke-WebRequest -Uri $primary -OutFile $env:PROOFBOARD_INSTALLER -UseBasicParsing -ErrorAction Stop}catch{$fallback=if($tag){'https://github.com/'+$env:PROOFBOARD_REPO+'/releases/download/'+$tag+'/install.ps1'}else{'https://github.com/'+$env:PROOFBOARD_REPO+'/releases/latest/download/install.ps1'}; Invoke-WebRequest -Uri $fallback -OutFile $env:PROOFBOARD_INSTALLER -UseBasicParsing -Headers $headers -ErrorAction Stop}"
if errorlevel 1 (
    echo Could not download the Proofboard Career Agent installer.
    set "PROOFBOARD_EXIT=1"
    goto :finish
)

:run
powershell -NoProfile -ExecutionPolicy Bypass -File "%PROOFBOARD_INSTALLER%"
set "PROOFBOARD_EXIT=%errorlevel%"

:finish
if defined DOWNLOADED_INSTALLER del /f /q "%DOWNLOADED_INSTALLER%" >nul 2>&1
if "%PROOFBOARD_PAUSE%"=="1" (
    echo.
    pause
)
exit /b %PROOFBOARD_EXIT%
