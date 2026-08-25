; NSIS installer for cuterm (per-user install, no admin required).
; Build (from repo root):
;   makensis /DVERSION=0.2.3 packaging\installer.nsi
; Expects dist\cuterm-v<VERSION>-windows-amd64.exe to exist (build.sh output).
; Output: dist\cuterm-v<VERSION>-windows-amd64-setup.exe

!ifndef VERSION
  !define VERSION "0.0.0"
!endif

!define APPNAME "cuterm"
!define PUBLISHER "cuterxy"
!define UNINSTKEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\cuterm"

; HWND_BROADCAST / WM_SETTINGCHANGE come from WinMessages.nsh (via MUI2.nsh)
!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "WordFunc.nsh"

Name "${APPNAME}"
OutFile "..\dist\cuterm-v${VERSION}-windows-amd64-setup.exe"
InstallDir "$LOCALAPPDATA\Programs\cuterm"
RequestExecutionLevel user
Icon "..\assets\icon-tray.ico"
UninstallIcon "..\assets\icon-tray.ico"

VIProductVersion "${VERSION}.0"
VIAddVersionKey "ProductName" "${APPNAME}"
VIAddVersionKey "CompanyName" "${PUBLISHER}"
VIAddVersionKey "FileDescription" "cuterm installer"
VIAddVersionKey "FileVersion" "${VERSION}"

!define MUI_FINISHPAGE_RUN "$INSTDIR\cuterm.exe"
!define MUI_FINISHPAGE_RUN_TEXT "Launch cuterm"
!insertmacro MUI_PAGE_LICENSE "..\LICENSE"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "SimpChinese"

; A running cuterm daemon would lock cuterm.exe and, after an upgrade, keep
; serving the old embedded web UI — stop it before install/uninstall.
!macro StopRunningCuterm
  nsExec::ExecToLog 'taskkill /F /IM cuterm.exe'
!macroend

Function .onInit
  !insertmacro StopRunningCuterm
FunctionEnd

Function un.onInit
  !insertmacro StopRunningCuterm
FunctionEnd

; Broadcast WM_SETTINGCHANGE so new consoles pick up the updated Path.
!macro BroadcastEnvChange
  SendMessage ${HWND_BROADCAST} ${WM_SETTINGCHANGE} 0 "STR:Environment" /TIMEOUT=5000
!macroend

; Append $INSTDIR to the user Path unless it is already there.
Function AddToPath
  ReadRegStr $0 HKCU "Environment" "Path"
  ${If} $0 == ""
    WriteRegExpandStr HKCU "Environment" "Path" "$INSTDIR"
  ${Else}
    ${WordFind} "$0" ";" "#" $1
    ${For} $2 1 $1
      ${WordFind} "$0" ";" "+$2" $3
      ${If} $3 == $INSTDIR
        Return
      ${EndIf}
    ${Next}
    StrCpy $4 $0 1 -1
    ${If} $4 == ";"
      WriteRegExpandStr HKCU "Environment" "Path" "$0$INSTDIR"
    ${Else}
      WriteRegExpandStr HKCU "Environment" "Path" "$0;$INSTDIR"
    ${EndIf}
  ${EndIf}
FunctionEnd

Section "Install"
  SetOutPath $INSTDIR
  ; Install directly as cuterm.exe — File overwrites any old version
  ; (Rename would silently fail when the target already exists).
  ; Also drop versioned binaries left behind by older installers.
  Delete "$INSTDIR\cuterm-v*-windows-amd64.exe"
  File /oname=cuterm.exe "..\dist\cuterm-v${VERSION}-windows-amd64.exe"
  WriteUninstaller "$INSTDIR\uninstall.exe"

  CreateDirectory "$SMPROGRAMS\cuterm"
  CreateShortcut "$SMPROGRAMS\cuterm\cuterm.lnk" "$INSTDIR\cuterm.exe"
  CreateShortcut "$SMPROGRAMS\cuterm\Uninstall cuterm.lnk" "$INSTDIR\uninstall.exe"

  WriteRegStr HKCU "${UNINSTKEY}" "DisplayName" "${APPNAME}"
  WriteRegStr HKCU "${UNINSTKEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${UNINSTKEY}" "Publisher" "${PUBLISHER}"
  WriteRegStr HKCU "${UNINSTKEY}" "DisplayIcon" "$INSTDIR\cuterm.exe"
  WriteRegStr HKCU "${UNINSTKEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTKEY}" "UninstallString" "$INSTDIR\uninstall.exe"
  WriteRegDWORD HKCU "${UNINSTKEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINSTKEY}" "NoRepair" 1

  Call AddToPath
  !insertmacro BroadcastEnvChange
SectionEnd

Section "Uninstall"
  Delete "$INSTDIR\cuterm.exe"
  Delete "$INSTDIR\uninstall.exe"
  ; Clean up versioned binaries left behind by older installers.
  Delete "$INSTDIR\cuterm-v*-windows-amd64.exe"
  RMDir "$INSTDIR"

  Delete "$SMPROGRAMS\cuterm\cuterm.lnk"
  Delete "$SMPROGRAMS\cuterm\Uninstall cuterm.lnk"
  RMDir "$SMPROGRAMS\cuterm"

  DeleteRegKey HKCU "${UNINSTKEY}"

  ; Remove $INSTDIR from the user Path, rebuilding it entry by entry.
  ReadRegStr $0 HKCU "Environment" "Path"
  ${If} $0 != ""
    ${WordFind} "$0" ";" "#" $1
    StrCpy $5 ""
    ${For} $2 1 $1
      ${WordFind} "$0" ";" "+$2" $3
      ${If} $3 != $INSTDIR
        ${If} $5 == ""
          StrCpy $5 $3
        ${Else}
          StrCpy $5 "$5;$3"
        ${EndIf}
      ${EndIf}
    ${Next}
    ${If} $5 == ""
      DeleteRegValue HKCU "Environment" "Path"
    ${Else}
      WriteRegExpandStr HKCU "Environment" "Path" "$5"
    ${EndIf}
  ${EndIf}
  !insertmacro BroadcastEnvChange
SectionEnd
