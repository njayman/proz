; The Windows installer of the desktop build, made by Inno Setup 6:
;   iscc /DVersion=1.2.3 packaging\windows\proz.iss
; It installs for the current user only (no admin): proz.exe, a Start menu
; entry that runs `proz gui`, proz on the user's PATH, and an uninstaller in
; Settings. Installing and uninstalling never touch the user's config folder.

#ifndef Version
  #define Version "0.0.0-snapshot"
#endif

[Setup]
; AppId ties upgrades and the uninstaller together; never change it.
AppId={{6F4A2B1E-8C3D-4E5F-9A70-2B1C3D4E5F60}
AppName=proz
AppVersion={#Version}
AppPublisher=proz
AppPublisherURL=https://github.com/njayman/proz
AppSupportURL=https://github.com/njayman/proz/issues
PrivilegesRequired=lowest
; With lowest privileges {autopf} is %LOCALAPPDATA%\Programs.
DefaultDirName={autopf}\proz
DisableDirPage=yes
DisableProgramGroupPage=yes
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
ChangesEnvironment=yes
UninstallDisplayIcon={app}\proz.exe
UninstallDisplayName=proz
OutputDir=..\..\dist
OutputBaseFilename=proz-setup-windows-amd64
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
; An upgrade closes a running proz (the tray) so it can replace proz.exe.
CloseApplications=yes

[Files]
Source: "..\..\dist\windows\proz.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
; ponytail: proz.exe is a console program (the CLI needs that), so Windows
; opens a console window next to the launcher until `proz gui` hides its own.
Name: "{autoprograms}\proz"; Filename: "{app}\proz.exe"; Parameters: "gui"; Comment: "Open your projects"

[Registry]
Root: HKCU; Subkey: "Environment"; ValueType: expandsz; ValueName: "Path"; ValueData: "{olddata};{app}"; Check: NotOnPath(ExpandConstant('{app}'))

[Code]
function PathWithout(Path, Dir: string): string;
begin
  Result := ';' + Path + ';';
  StringChangeEx(Result, ';' + Dir + ';', ';', True);
  Result := Copy(Result, 2, Length(Result) - 2);
end;

function NotOnPath(Dir: string): Boolean;
var
  Path: string;
begin
  if not RegQueryStringValue(HKCU, 'Environment', 'Path', Path) then
    Path := '';
  Result := Pos(';' + Uppercase(Dir) + ';', ';' + Uppercase(Path) + ';') = 0;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  Path: string;
begin
  if (CurUninstallStep = usPostUninstall) and
     RegQueryStringValue(HKCU, 'Environment', 'Path', Path) then
    RegWriteExpandStringValue(HKCU, 'Environment', 'Path',
      PathWithout(Path, ExpandConstant('{app}')));
end;
