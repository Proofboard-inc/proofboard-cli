@echo off
setlocal enabledelayedexpansion

rem Proofboard Career Agent installation launcher for Windows.
rem
rem Double-click this file, or run it from a command prompt. It runs the
rem PowerShell installer sitting next to it when present, and otherwise
rem downloads that installer from the latest release published on the Git
rem repository. The PowerShell installer requests administrator access through
rem UAC when it needs to write into Program Files.
rem
rem While the repository is private a token is required to download the
rem installer: set PROOFBOARD_GITHUB_TOKEN (GH_TOKEN and GITHUB_TOKEN are also
rem honoured). Without a token the public download host is used instead.

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
rem explicit download base wins, then the release on the repository, then the
rem public download host.
powershell -NoProfile -ExecutionPolicy Bypass -Command "$ErrorActionPreference='Stop'; [Net.ServicePointManager]::SecurityProtocol=[Net.SecurityProtocolType]::Tls12; $headers=@{'User-Agent'='proofboard-installer'}; $token=$env:PROOFBOARD_GITHUB_TOKEN; if(-not $token){$token=$env:GH_TOKEN}; if(-not $token){$token=$env:GITHUB_TOKEN}; if($token){$headers['Authorization']='Bearer '+$token}; if($env:PROOFBOARD_DOWNLOAD_BASE_URL){Invoke-WebRequest -Uri ($env:PROOFBOARD_DOWNLOAD_BASE_URL+'/install.ps1') -OutFile $env:PROOFBOARD_INSTALLER -UseBasicParsing; exit 0}; $release=$null; $path=if($env:PROOFBOARD_VERSION){'tags/'+$env:PROOFBOARD_VERSION}else{'latest'}; try{$release=Invoke-RestMethod -Uri ('https://api.github.com/repos/'+$env:PROOFBOARD_REPO+'/releases/'+$path) -Headers $headers}catch{$release=$null}; $asset=$null; if($release){$asset=$release.assets ^| Where-Object {$_.name -eq 'install.ps1'} ^| Select-Object -First 1}; if($asset){$headers['Accept']='application/octet-stream'; Invoke-WebRequest -Uri ('https://api.github.com/repos/'+$env:PROOFBOARD_REPO+'/releases/assets/'+$asset.id) -OutFile $env:PROOFBOARD_INSTALLER -UseBasicParsing -Headers $headers}else{Invoke-WebRequest -Uri 'https://releases.proofboard.io/install.ps1' -OutFile $env:PROOFBOARD_INSTALLER -UseBasicParsing}"
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
