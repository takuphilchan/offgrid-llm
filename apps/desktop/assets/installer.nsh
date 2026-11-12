; Use supported electron-builder hooks; retain native keyboard, high-DPI,
; per-user/elevated installation, upgrade, and uninstallation behavior.
!include LogicLib.nsh
!include Util.nsh
!include "${__FILEDIR__}\installer-process-guard.nsh"
!define MUI_TEXTCOLOR "202020"
!define MUI_BGCOLOR "FFFFFF"
!define MUI_INSTFILESPAGE_COLORS "202020 FFFFFF"
!define MUI_CUSTOMFUNCTION_GUIINIT OffGridStyleWindow
!define MUI_CUSTOMFUNCTION_UNGUIINIT un.OffGridStyleWindow
!define MUI_DIRECTORYPAGE_TEXT_TOP "Choose where to install OffGrid. Your models and saved workspace are stored separately and will be kept when updating or reinstalling."
!macro customHeader
  ; NSIS supports system DPI awareness. Do not claim per-monitor rescaling
  ; until the installer engine implements and qualifies that behavior.
  ManifestDPIAware true
  SetFont "Segoe UI" 9
  BrandingText " "
  ShowInstDetails hide
  ShowUninstDetails hide
  !ifdef BUILD_UNINSTALLER
    !insertmacro OffGridFullPage "un." "Welcome"
    !insertmacro OffGridFullPage "un." "Finish"
  !else
    !insertmacro OffGridFullPage "" "Welcome"
    !insertmacro OffGridFullPage "" "Finish"
  !endif
!macroend

!macro customWelcomePage
  !define MUI_PAGE_CUSTOMFUNCTION_SHOW OffGridWelcomeLayout
  !define MUI_WELCOMEPAGE_TITLE "Install OffGrid"
  !define MUI_WELCOMEPAGE_TEXT "A local workspace for conversations, documents and agents.$\r$\n$\r$\nYour models and saved work are kept."
  !insertmacro MUI_PAGE_WELCOME
!macroend

!macro customUnWelcomePage
  !define MUI_PAGE_CUSTOMFUNCTION_SHOW un.OffGridWelcomeLayout
  !define MUI_WELCOMEPAGE_TITLE "Remove OffGrid?"
  !define MUI_WELCOMEPAGE_TEXT "This removes the application and its shortcuts.$\r$\n$\r$\nYour models and saved work are kept. Separately managed services are not removed."
  !insertmacro MUI_UNPAGE_WELCOME
!macroend

!macro customUninstallPage
  !define MUI_PAGE_CUSTOMFUNCTION_SHOW un.OffGridFinishLayout
  ; MUI_UNPAGE_FINISH consumes the same page-local definitions as install
  ; Finish. Set them in this hook immediately before that page is inserted.
  !define MUI_FINISHPAGE_TITLE "OffGrid removed"
  !define MUI_FINISHPAGE_TEXT "The application has been removed.$\r$\n$\r$\nYour models and saved work are still on this computer. Reinstall OffGrid to use them again."
!macroend

; Keep the native page lifecycle, but replace the stock bitmap-column layout.
; All geometry is in dialog units, mapped by Windows using the current font/DPI.
; Do not use screen pixels: these pages also run at enlarged display scales.
!macro OffGridPlace CONTROL X Y W H
  System::Call '*(&i4 ${X}, &i4 ${Y}, &i4 ${W}, &i4 ${H}) p .r0'
  System::Call 'user32::MapDialogRect(p $HWNDPARENT, p r0)'
  System::Call '*$0(&i4 .r1, &i4 .r2, &i4 .r3, &i4 .r4)'
  System::Free $0
  System::Call 'user32::MoveWindow(p ${CONTROL}, i r1, i r2, i r3, i r4, i 1)'
!macroend

!macro OffGridFullPage PREFIX PAGE
Function ${PREFIX}OffGrid${PAGE}Layout
  Push $0
  Push $1
  Push $2
  Push $3
  Push $4
  ; A small layer mark replaces the full-height decorative sidebar.
  !insertmacro OffGridPlace $mui.${PAGE}Page.Image 24 20 22 22
  ${NSD_FreeImage} $mui.${PAGE}Page.Image.Bitmap
  ${NSD_SetStretchedImage} $mui.${PAGE}Page.Image "$PLUGINSDIR\modern-wizard.bmp" $mui.${PAGE}Page.Image.Bitmap
  !insertmacro OffGridPlace $mui.${PAGE}Page.Title 24 50 280 28
  StrCpy $0 $mui.${PAGE}Page.Title.Font
  CreateFont $mui.${PAGE}Page.Title.Font "Segoe UI" 18 600
  SendMessage $mui.${PAGE}Page.Title ${WM_SETFONT} $mui.${PAGE}Page.Title.Font 1
  System::Call 'gdi32::DeleteObject(p r0)'
  !insertmacro OffGridPlace $mui.${PAGE}Page.Text 24 84 280 52
  !if "${PAGE}" == "Finish"
    !if "${PREFIX}" == ""
      !ifndef HIDE_RUN_AFTER_FINISH
        !insertmacro OffGridPlace $mui.FinishPage.Run 24 148 280 14
      !endif
    !endif
  !endif
  Pop $4
  Pop $3
  Pop $2
  Pop $1
  Pop $0
FunctionEnd
!macroend

!macro OffGridStyleWindow PREFIX
Function ${PREFIX}OffGridStyleWindow
  ; Keep system contrast colors when Windows high-contrast mode is active.
  ${IfNot} ${IsHighContrastModeActive}
    SetCtlColors $HWNDPARENT "202020" "FFFFFF"
    GetDlgItem $0 $HWNDPARENT 1028
    SetCtlColors $0 "757575" "FFFFFF"
  ${EndIf}
FunctionEnd
!macroend
!insertmacro OffGridStyleWindow ""
!insertmacro OffGridStyleWindow "un."

!macro customPageAfterChangeDir
  !define MUI_PAGE_CUSTOMFUNCTION_SHOW OffGridStyleProgress
!macroend
Function OffGridStyleProgress
  ${IfNot} ${IsHighContrastModeActive}
    GetDlgItem $0 $HWNDPARENT 1016
    SetCtlColors $0 "202020" "FFFFFF"
    FindWindow $1 "#32770" "" $HWNDPARENT
    SetCtlColors $1 "202020" "FFFFFF"
    ; The current-file label sits directly above the progress control. NSIS
    ; gives it the progress-track background by default, which reads as a
    ; second empty bar at high DPI. Keep the label on the page background.
    GetDlgItem $0 $1 1006
    SetCtlColors $0 "202020" "FFFFFF"
    GetDlgItem $0 $1 1004
    ; Only the progress bar opts out of visual themes, to honor monochrome.
    System::Call 'uxtheme::SetWindowTheme(p r0, w"", w"")'
    SendMessage $0 0x409 0 0x202020
    SendMessage $0 0x2001 0 0xEEEEEE
  ${EndIf}
FunctionEnd

; Keep the Finish callback trivial. Shell activation/virus scanning must not
; block the wizard's UI thread while its Finish button is still on screen.
!macro customFinishPage
  !define MUI_PAGE_CUSTOMFUNCTION_SHOW OffGridFinishLayout
  !define MUI_FINISHPAGE_TITLE "OffGrid installed"
  !define MUI_FINISHPAGE_TEXT "Your models and saved work are kept.$\r$\n$\r$\nOpen OffGrid to continue. Separately managed services are unchanged."
  !ifndef HIDE_RUN_AFTER_FINISH
    Var OffGridLaunchRequested
    !define MUI_FINISHPAGE_RUN
    !define MUI_FINISHPAGE_RUN_FUNCTION OffGridRequestLaunch
    Function OffGridRequestLaunch
      StrCpy $OffGridLaunchRequested "yes"
    FunctionEnd
    Function .onGUIEnd
      ${If} $OffGridLaunchRequested == "yes"
        SetOutPath $INSTDIR
        StrCpy $4 ""
        !if "${APP_ID}" == "com.offgrid.llm.desktop.installtest"
          ; Test-only arguments preserve isolation across Explorer/UAC launch.
          ReadEnvStr $2 "OFFGRID_DESKTOP_HOME"
          ReadEnvStr $3 "OFFGRID_PORT"
          StrCpy $4 '--offgrid-test-profile="$2" --offgrid-test-port=$3'
        !endif
        ${IfNot} ${UAC_IsAdmin}
          ; The default per-user path needs neither Explorer/COM nor a link.
          Exec '"$INSTDIR\${APP_EXECUTABLE_FILENAME}" $4'
        ${Else}
          ; Never launch the product with the installer's elevated token.
          ${StdUtils.ExecShellAsUser} $0 "$INSTDIR\${APP_EXECUTABLE_FILENAME}" "open" "$4"
        ${EndIf}
      ${EndIf}
    FunctionEnd
  !endif
  !insertmacro MUI_PAGE_FINISH
!macroend

; Native, bounded running-app detection replaces repeated PowerShell/WMI
; startup. Never force-kill an app, its model worker, or another user's service.
; Older releases that do not support the quit request must be quit manually.
!macro customCheckAppRunning
  ; electron-builder disables detail output for interactive installs. Restore
  ; the log without forcing it open or flooding the main status label.
  SetDetailsPrint listonly
  DetailPrint "Checking for a running OffGrid application..."
  IfFileExists "$INSTDIR\${APP_EXECUTABLE_FILENAME}" 0 offgrid_not_running
  ${nsProcess::FindProcess} "${APP_EXECUTABLE_FILENAME}" $R0
  !insertmacro OffGridRequireProcessInspection "${APP_EXECUTABLE_FILENAME}"
  ${If} $R0 == 603
    Goto offgrid_not_running
  ${EndIf}
  ${If} ${Silent}
    DetailPrint "OffGrid is running. Quit it before installing silently; no processes were stopped."
    SetErrorLevel 2
    Quit
  ${EndIf}
  MessageBox MB_OKCANCEL|MB_ICONINFORMATION "Setup needs to close OffGrid before changing application files. Finish any active tasks first. Your models and saved workspace will be kept.$\r$\n$\r$\nClick OK to close OffGrid and continue, or Cancel to leave it running." /SD IDCANCEL IDCANCEL offgrid_cancel_install
  ${If} $R0 == 0
    StrCpy $4 ""
    !if "${APP_ID}" == "com.offgrid.llm.desktop.installtest"
      ; The isolated test app uses a separate Electron single-instance scope.
      ; Pass the same profile so the quit request reaches that running app.
      ReadEnvStr $2 "OFFGRID_DESKTOP_HOME"
      ReadEnvStr $3 "OFFGRID_PORT"
      StrCpy $4 '--offgrid-test-profile="$2" --offgrid-test-port=$3'
    !endif
    Exec '"$INSTDIR\${APP_EXECUTABLE_FILENAME}" --offgrid-quit-for-install $4'
  ${EndIf}
  StrCpy $R1 0
  offgrid_wait_for_exit:
    Sleep 200
    ${nsProcess::FindProcess} "${APP_EXECUTABLE_FILENAME}" $R0
    !insertmacro OffGridRequireProcessInspection "${APP_EXECUTABLE_FILENAME}"
    ${If} $R0 == 603
      Goto offgrid_not_running
    ${EndIf}
    IntOp $R1 $R1 + 1
    ${If} $R1 < 75
      Goto offgrid_wait_for_exit
    ${EndIf}
    MessageBox MB_RETRYCANCEL|MB_ICONINFORMATION "OffGrid is still running. Older versions may need File > Quit or Quit OffGrid in the system tray. If another signed-in user is running OffGrid, ask them to quit it too. Setup has not forced any process to stop.$\r$\n$\r$\nAfter quitting, click Retry." /SD IDCANCEL IDRETRY offgrid_retry_exit
  offgrid_cancel_install:
    SetErrorLevel 2
    Quit
  offgrid_retry_exit:
    StrCpy $R1 0
    Goto offgrid_wait_for_exit
  offgrid_not_running:
  !insertmacro OffGridRequireProcessClosed "${APP_EXECUTABLE_FILENAME}"
  !insertmacro OffGridRequireProcessClosed "offgrid.exe"
  !insertmacro OffGridRequireProcessClosed "llama-server.exe"
  DetailPrint "Application is closed. Updating application files; saved workspace and models are kept."
!macroend

!macro customInstall
  SetDetailsPrint listonly
  DetailPrint "Application files and installation registration are ready."
  DetailPrint "Setup does not download models. Choose a model after opening OffGrid."
!macroend

Function .onInstFailed
  SetDetailsPrint listonly
  DetailPrint "Installation did not complete. Close applications using the install folder, check free disk space and permissions, then retry. Saved work is stored separately."
FunctionEnd
