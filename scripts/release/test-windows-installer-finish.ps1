$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if (-not ('OffGridWizard' -as [type])) { . (Join-Path $PSScriptRoot 'windows-installer-ui.ps1') }

# Pure timing delegates: no windows, capture, installers or user input.
$clock = @{ now = 0L }
$prompt = [OffGridWizard]::ObserveFinish({ $clock.now -lt 100 }, { $clock.now }, { $clock.now += 10 })
if (-not $prompt.Hidden -or $prompt.ClosedMs -ne 100 -or $prompt.MaxProbeGapMs -ne 10) { throw 'Prompt closure timing lost.' }
# Slow diagnostic work after the observation must not age its receipt.
$clock.now += 3345
if ($prompt.ClosedMs -ne 100) { throw 'Diagnostics changed observed closure.' }
$clock.now = 0L
$blocked = [OffGridWizard]::ObserveFinish({ $true }, { $clock.now }, { $clock.now += 10 })
if ($blocked.Hidden -or $blocked.ClosedMs -ne 2010) { throw 'Visible Finish deadline weakened.' }
$clock.now = 0L
$gap = [OffGridWizard]::ObserveFinish({ $clock.now -eq 0 }, { $clock.now }, { $clock.now += 3345 })
if (-not $gap.Hidden -or $gap.ClosedMs -ne 3345 -or $gap.MaxProbeGapMs -ne 3345) { throw 'Late observation must retain failure evidence.' }
$clock.now = 0L
$slowProbe = [OffGridWizard]::ObserveFinish({ $clock.now += 2001; $false }, { $clock.now }, { $clock.now += 10 })
if ($slowProbe.ClosedMs -ne 2001) { throw 'Visibility probe time excluded from deadline.' }
$clock.now = 0L
$boundary = [OffGridWizard]::ObserveFinish({ $clock.now -lt 2000 }, { $clock.now }, { $clock.now += 10 })
if (-not $boundary.Hidden -or $boundary.ClosedMs -ne 2000) { throw 'Exact visibility deadline changed.' }
Write-Output 'PASS: Finish observation excludes diagnostics, preserves the 2000ms gate and reports monitoring gaps.'
