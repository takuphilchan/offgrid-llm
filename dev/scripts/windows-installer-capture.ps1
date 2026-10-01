param(
    [Parameter(Mandatory = $true)][long]$Window,
    [Parameter(Mandatory = $true)][uint32]$OwnerId,
    [Parameter(Mandatory = $true)][string]$OutputPath
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'windows-installer-ui.ps1')

$handle = [IntPtr]::new($Window)
if (-not [OffGridWizard]::IsWindow($handle) -or [OffGridWizard]::Owner($handle) -ne $OwnerId -or
    -not [OffGridWizard]::Text($handle).StartsWith('OffGrid Desktop Install Test', [StringComparison]::Ordinal)) {
    throw 'Capture target is not the original isolated installer window.'
}
# This call may block indefinitely. The parent owns and bounds this helper's
# lifetime while continuing its independent responsiveness probes.
[OffGridWizard]::Capture($handle, $OutputPath)
