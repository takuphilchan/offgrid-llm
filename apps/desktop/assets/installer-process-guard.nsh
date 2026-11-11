!ifndef OFFGRID_PROCESS_GUARD_INCLUDED
!define OFFGRID_PROCESS_GUARD_INCLUDED
!include LogicLib.nsh

; nsProcess returns 0 for a match and 603 only when no matching process exists.
; Every other result is an inspection failure, never permission to replace files.
; The caller must include nsProcess.nsh before expanding this macro.
!macro OffGridRequireProcessInspection PROCESS_NAME
  ${If} $R0 != 0
  ${AndIf} $R0 != 603
    DetailPrint "Process inspection failed for ${PROCESS_NAME} (code $R0)."
    MessageBox MB_OK|MB_ICONSTOP "Setup could not check whether ${PROCESS_NAME} is running. No application files were replaced. Check that Windows process inspection is available, then try again." /SD IDOK
    SetErrorLevel 3
    Quit
  ${EndIf}
!macroend

!macro OffGridRequireProcessClosed PROCESS_NAME
  ${nsProcess::FindProcess} "${PROCESS_NAME}" $R0
  !insertmacro OffGridRequireProcessInspection "${PROCESS_NAME}"
  ${If} $R0 == 0
    DetailPrint "${PROCESS_NAME} is running; no processes were stopped."
    MessageBox MB_OK|MB_ICONEXCLAMATION "OffGrid cannot change application files while ${PROCESS_NAME} is running. Close OffGrid and stop any separately managed OffGrid service, then try again. Your models and saved work are kept." /SD IDOK
    SetErrorLevel 2
    Quit
  ${EndIf}
!macroend
!endif
