; OffGrid LLM NSIS Installer Script
; Creates Windows installer with service integration

!define PRODUCT_NAME "OffGrid LLM"
!define PRODUCT_VERSION "0.1.0"
!define PRODUCT_PUBLISHER "OffGrid LLM Team"
!define PRODUCT_WEB_SITE "https://github.com/takuphilchan/offgrid-llm"
!define PRODUCT_DIR_REGKEY "Software\Microsoft\Windows\CurrentVersion\App Paths\offgrid.exe"
!define PRODUCT_UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}"

; MUI Settings
!include "MUI2.nsh"
!include "FileFunc.nsh"

Name "${PRODUCT_NAME} ${PRODUCT_VERSION}"
OutFile "OffGridSetup-${PRODUCT_VERSION}.exe"
InstallDir "$PROGRAMFILES64\OffGrid"
InstallDirRegKey HKLM "${PRODUCT_DIR_REGKEY}" ""
ShowInstDetails show
ShowUnInstDetails show

; Request admin privileges
RequestExecutionLevel admin

; MUI Interface Configuration
!define MUI_ABORTWARNING
!define MUI_ICON "${NSISDIR}\Contrib\Graphics\Icons\modern-install.ico"
!define MUI_UNICON "${NSISDIR}\Contrib\Graphics\Icons\modern-uninstall.ico"

; Pages
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "..\..\LICENSE"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES

!define MUI_FINISHPAGE_RUN
!define MUI_FINISHPAGE_RUN_TEXT "Open Command Prompt and run 'offgrid --help'"
!define MUI_FINISHPAGE_RUN_FUNCTION "LaunchCommandPrompt"
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

; Languages
!insertmacro MUI_LANGUAGE "English"

; Installer Sections
Section "Main Application" SEC01
  SetOutPath "$INSTDIR"
  SetOverwrite on
  
  ; Copy binaries
  File "..\..\dist\offgrid-windows-amd64.exe"
  File "..\..\dist\llama-server.exe"
  
  ; Rename to standard names
  Rename "$INSTDIR\offgrid-windows-amd64.exe" "$INSTDIR\offgrid.exe"
  
  ; Copy documentation
  File "..\..\README.md"
  File "..\..\LICENSE"
  
  ; Create config directory
  CreateDirectory "$APPDATA\OffGrid"
  
  ; Add to PATH
  EnVar::SetHKLM
  EnVar::AddValue "PATH" "$INSTDIR"
  
  ; Create uninstaller
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  
  ; Registry entries
  WriteRegStr HKLM "${PRODUCT_DIR_REGKEY}" "" "$INSTDIR\offgrid.exe"
  WriteRegStr HKLM "${PRODUCT_UNINST_KEY}" "DisplayName" "${PRODUCT_NAME}"
  WriteRegStr HKLM "${PRODUCT_UNINST_KEY}" "UninstallString" "$INSTDIR\Uninstall.exe"
  WriteRegStr HKLM "${PRODUCT_UNINST_KEY}" "DisplayIcon" "$INSTDIR\offgrid.exe"
  WriteRegStr HKLM "${PRODUCT_UNINST_KEY}" "DisplayVersion" "${PRODUCT_VERSION}"
  WriteRegStr HKLM "${PRODUCT_UNINST_KEY}" "Publisher" "${PRODUCT_PUBLISHER}"
  WriteRegStr HKLM "${PRODUCT_UNINST_KEY}" "URLInfoAbout" "${PRODUCT_WEB_SITE}"
  
  ; Calculate install size
  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  IntFmt $0 "0x%08X" $0
  WriteRegDWORD HKLM "${PRODUCT_UNINST_KEY}" "EstimatedSize" "$0"
  
SectionEnd

Section "Start Menu Shortcuts" SEC02
  CreateDirectory "$SMPROGRAMS\OffGrid LLM"
  CreateShortCut "$SMPROGRAMS\OffGrid LLM\OffGrid Command Prompt.lnk" \
    "$WINDIR\System32\cmd.exe" "/K offgrid --help" \
    "$INSTDIR\offgrid.exe"
  CreateShortCut "$SMPROGRAMS\OffGrid LLM\Uninstall.lnk" "$INSTDIR\Uninstall.exe"
  CreateShortCut "$SMPROGRAMS\OffGrid LLM\README.lnk" "$INSTDIR\README.md"
SectionEnd

Section "Desktop Shortcut" SEC03
  CreateShortCut "$DESKTOP\OffGrid LLM.lnk" "$WINDIR\System32\cmd.exe" \
    "/K offgrid --help" "$INSTDIR\offgrid.exe"
SectionEnd

; Section descriptions
!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
  !insertmacro MUI_DESCRIPTION_TEXT ${SEC01} "OffGrid LLM core application and llama.cpp inference engine"
  !insertmacro MUI_DESCRIPTION_TEXT ${SEC02} "Create Start Menu shortcuts"
  !insertmacro MUI_DESCRIPTION_TEXT ${SEC03} "Create Desktop shortcut"
!insertmacro MUI_FUNCTION_DESCRIPTION_END

; Uninstaller
Section "Uninstall"
  ; Remove files
  Delete "$INSTDIR\offgrid.exe"
  Delete "$INSTDIR\llama-server.exe"
  Delete "$INSTDIR\README.md"
  Delete "$INSTDIR\LICENSE"
  Delete "$INSTDIR\Uninstall.exe"
  
  ; Remove directories
  RMDir "$INSTDIR"
  
  ; Remove shortcuts
  Delete "$SMPROGRAMS\OffGrid LLM\*.*"
  RMDir "$SMPROGRAMS\OffGrid LLM"
  Delete "$DESKTOP\OffGrid LLM.lnk"
  
  ; Remove from PATH
  EnVar::SetHKLM
  EnVar::DeleteValue "PATH" "$INSTDIR"
  
  ; Remove registry entries
  DeleteRegKey HKLM "${PRODUCT_UNINST_KEY}"
  DeleteRegKey HKLM "${PRODUCT_DIR_REGKEY}"
  
  ; Optional: Remove config directory
  MessageBox MB_YESNO "Remove configuration files in $APPDATA\OffGrid?" IDNO +2
  RMDir /r "$APPDATA\OffGrid"
  
  SetAutoClose true
SectionEnd

; Launch Command Prompt function
Function LaunchCommandPrompt
  Exec '"$WINDIR\System32\cmd.exe" /K "cd $INSTDIR && offgrid --help"'
FunctionEnd

; Installer initialization
Function .onInit
  ; Check if already installed
  ReadRegStr $R0 HKLM "${PRODUCT_UNINST_KEY}" "UninstallString"
  StrCmp $R0 "" done
  
  MessageBox MB_OKCANCEL|MB_ICONEXCLAMATION \
    "${PRODUCT_NAME} is already installed.$\n$\nClick OK to uninstall the previous version, or Cancel to cancel this installation." \
    IDOK uninst
  Abort
  
uninst:
  ExecWait '$R0 /S _?=$INSTDIR'
  Delete "$INSTDIR\Uninstall.exe"
  RMDir $INSTDIR
  
done:
FunctionEnd
