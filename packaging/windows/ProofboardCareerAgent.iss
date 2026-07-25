#ifndef BinaryPath
  #error BinaryPath must point to proofboard-windows-amd64.exe
#endif

#ifndef MyAppVersion
  #define MyAppVersion "1.8.18"
#endif

#ifndef InstallerOutputDir
  #define InstallerOutputDir "."
#endif

[Setup]
AppId={{1B2D761B-189B-4BD6-82F7-BB339CF0EA2D}
AppName=Proofboard Career Agent
AppVersion={#MyAppVersion}
AppPublisher=Proofboard
AppPublisherURL=https://proofboard.io
AppSupportURL=https://proofboard.io
DefaultDirName={autopf}\Proofboard
DefaultGroupName=Proofboard Career Agent
DisableProgramGroupPage=yes
OutputDir={#InstallerOutputDir}
OutputBaseFilename=Proofboard-Career-Agent-windows-amd64-setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
UninstallDisplayName=Proofboard Career Agent
VersionInfoVersion={#MyAppVersion}
VersionInfoDescription=Proofboard Career Agent Installer
CloseApplications=yes

[Files]
Source: "{#BinaryPath}"; DestDir: "{app}"; DestName: "proofboard.exe"; Flags: ignoreversion

[Icons]
Name: "{group}\Proofboard Career Agent Status"; Filename: "{app}\proofboard.exe"; Parameters: "status"

[Run]
Filename: "{app}\proofboard.exe"; Parameters: "agent enable"; Description: "Start Proofboard Career Agent"; Flags: runhidden waituntilterminated runasoriginaluser

[UninstallRun]
Filename: "{app}\proofboard.exe"; Parameters: "agent disable"; RunOnceId: "DisableCareerAgent"; Flags: runhidden waituntilterminated skipifdoesntexist

[Code]
const
  EnvironmentKey = 'SYSTEM\CurrentControlSet\Control\Session Manager\Environment';

procedure SetMachinePathEntry(AddEntry: Boolean);
var
  CurrentPath: string;
  PaddedPath: string;
  AppPath: string;
begin
  AppPath := ExpandConstant('{app}');
  if not RegQueryStringValue(HKLM, EnvironmentKey, 'Path', CurrentPath) then
    CurrentPath := '';

  PaddedPath := ';' + CurrentPath + ';';
  if AddEntry then
  begin
    if Pos(';' + AppPath + ';', PaddedPath) = 0 then
    begin
      if CurrentPath <> '' then
      begin
        if CurrentPath[Length(CurrentPath)] <> ';' then
          CurrentPath := CurrentPath + ';';
      end;
      CurrentPath := CurrentPath + AppPath;
      RegWriteExpandStringValue(HKLM, EnvironmentKey, 'Path', CurrentPath);
    end;
  end
  else
  begin
    StringChangeEx(PaddedPath, ';' + AppPath + ';', ';', True);
    while Pos(';;', PaddedPath) > 0 do
      StringChangeEx(PaddedPath, ';;', ';', True);
    if Length(PaddedPath) >= 2 then
      CurrentPath := Copy(PaddedPath, 2, Length(PaddedPath) - 2)
    else
      CurrentPath := '';
    RegWriteExpandStringValue(HKLM, EnvironmentKey, 'Path', CurrentPath);
  end;
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
    SetMachinePathEntry(True);
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usUninstall then
    SetMachinePathEntry(False);
end;
