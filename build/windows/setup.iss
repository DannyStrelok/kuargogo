; Inno Setup Script for kuargogo
; To be compiled with ISCC.exe

#define MyAppName "Kuargogo CLI"
#define MyAppVersion "1.0.0" 
; NOTE: The version above is a placeholder. It will be overridden by the /DMyAppVersion=... command line argument during build.
#define MyAppPublisher "DannyStrelok"
#define MyAppURL "https://github.com/DannyStrelok/kuargogo"
#define MyAppExeName "kgg.exe"

[Setup]
; NOTE: The value of AppId uniquely identifies this application. Do not use the same AppId value in installers for other applications.
; (To generate a new GUID, click Tools | Generate GUID inside the IDE.)
AppId={{D3280B2F-B660-4F19-B1FF-3EFEF611F1D3}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
DefaultDirName={autopf}\{#MyAppName}
DisableProgramGroupPage=yes
SetupIconFile=app.ico
UninstallDisplayIcon={app}\{#MyAppExeName}
; Remove the following line to run in administrative install mode (install for all users.)
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
OutputBaseFilename=kgg-setup
Compression=lzma
SolidCompression=yes
WizardStyle=modern
ChangesEnvironment=yes

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked
Name: "addtopath"; Description: "Add to PATH environment variable"; GroupDescription: "Additional tasks"

[Files]
; We assume the source binary is passed via command line or located in a standard dist location relative to this script
; For the GitHub Action, we will place the binary in "dist/kgg.exe" before running ISCC.
Source: "..\..\dist\kgg.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent

[Code]
const
  EnvironmentKey = 'Environment';

procedure EnvAddPath(Path: string);
var
  Paths: string;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER, EnvironmentKey, 'Path', Paths) then
    Paths := '';

  if Pos(';' + Path + ';', ';' + Paths + ';') = 0 then
  begin
    Paths := Paths + ';' + Path;
    RegWriteStringValue(HKEY_CURRENT_USER, EnvironmentKey, 'Path', Paths);
  end;
end;

procedure EnvRemovePath(Path: string);
var
  Paths: string;
  P: Integer;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER, EnvironmentKey, 'Path', Paths) then
    Exit;

  P := Pos(';' + Path + ';', ';' + Paths + ';');
  if P = 0 then
  begin
    if Pos(Path + ';', Paths) = 1 then
      P := 1
    else if Pos(';' + Path, Paths) = Length(Paths) - Length(Path) then
      P := Length(Paths) - Length(Path);
  end;

  if P > 0 then
  begin
    Delete(Paths, P - 1, Length(Path) + 1);
    RegWriteStringValue(HKEY_CURRENT_USER, EnvironmentKey, 'Path', Paths);
  end;
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if (CurStep = ssPostInstall) and IsTaskSelected('addtopath') then
  begin
    EnvAddPath(ExpandConstant('{app}'));
  end;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usPostUninstall then
  begin
    EnvRemovePath(ExpandConstant('{app}'));
  end;
end;
