; NSIS installer for cuterm-hub (per-user install, no admin required).
; Build (from repo root):
;   makensis /DVERSION=0.2.3 packaging\installer-hub.nsi
; Expects dist\cuterm-hub-v<VERSION>-windows-amd64.exe to exist (build.sh output).
; Output: dist\cuterm-hub-v<VERSION>-windows-amd64-setup.exe

!ifndef VERSION
  !define VERSION "0.0.0"
!endif

!define APPNAME "cuterm-hub"
!define PUBLISHER "cuterxy"
!define UNINSTKEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\cuterm-hub"

; HWND_BROADCAST / WM_SETTINGCHANGE come from WinMessages.nsh (via MUI2.nsh)
!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "WordFunc.nsh"

Name "${APPNAME}"
OutFile "..\dist\cuterm-hub-v${VERSION}-windows-amd64-setup.exe"
InstallDir "$LOCALAPPDATA\Programs\cuterm-hub"
RequestExecutionLevel user
Icon "..\cmd\cuterm-hub\assets\icon-tray.ico"
UninstallIcon "..\cmd\cuterm-hub\assets\icon-tray.ico"

VIProductVersion "${VERSION}.0"
VIAddVersionKey "ProductName" "${APPNAME}"
VIAddVersionKey "CompanyName" "${PUBLISHER}"
VIAddVersionKey "FileDescription" "cuterm-hub installer"
VIAddVersionKey "FileVersion" "${VERSION}"

!define MUI_FINISHPAGE_RUN "$INSTDIR\cuterm-hub.exe"
!define MUI_FINISHPAGE_RUN_TEXT "Launch cuterm-hub"
!insertmacro MUI_PAGE_LICENSE "..\LICENSE"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "SimpChinese"

; A running cuterm-hub daemon would lock cuterm-hub.exe and, after an upgrade,
; keep serving the old embedded web UI — stop it before install/uninstall.
!macro StopRunningCutermHub
  nsExec::ExecToLog 'taskkill /F /IM cuterm-hub.exe'
!macroend

Function .onInit
  !insertmacro StopRunningCutermHub
FunctionEnd

Function un.onInit
  !insertmacro StopRunningCutermHub
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
  File "..\dist\cuterm-hub-v${VERSION}-windows-amd64.exe"
  Rename "$INSTDIR\cuterm-hub-v${VERSION}-windows-amd64.exe" "$INSTDIR\cuterm-hub.exe"
  WriteUninstaller "$INSTDIR\uninstall.exe"

  CreateDirectory "$SMPROGRAMS\cuterm-hub"
  CreateShortcut "$SMPROGRAMS\cuterm-hub\cuterm-hub.lnk" "$INSTDIR\cuterm-hub.exe"
  CreateShortcut "$SMPROGRAMS\cuterm-hub\Uninstall cuterm-hub.lnk" "$INSTDIR\uninstall.exe"

  WriteRegStr HKCU "${UNINSTKEY}" "DisplayName" "${APPNAME}"
  WriteRegStr HKCU "${UNINSTKEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${UNINSTKEY}" "Publisher" "${PUBLISHER}"
  WriteRegStr HKCU "${UNINSTKEY}" "DisplayIcon" "$INSTDIR\cuterm-hub.exe"
  WriteRegStr HKCU "${UNINSTKEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTKEY}" "UninstallString" "$INSTDIR\uninstall.exe"
  WriteRegDWORD HKCU "${UNINSTKEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINSTKEY}" "NoRepair" 1

  Call AddToPath
  !insertmacro BroadcastEnvChange
SectionEnd

Section "Uninstall"
  Delete "$INSTDIR\cuterm-hub.exe"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"

  Delete "$SMPROGRAMS\cuterm-hub\cuterm-hub.lnk"
  Delete "$SMPROGRAMS\cuterm-hub\Uninstall cuterm-hub.lnk"
  RMDir "$SMPROGRAMS\cuterm-hub"

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
