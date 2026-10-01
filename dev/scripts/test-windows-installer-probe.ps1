param([string]$FixtureDirectory)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

# Real windows on separate deliberately blocked UI threads: a hidden maintenance
# window must not fail qualification, but the same visible hang must still fail.
if ($FixtureDirectory) {
Add-Type -ReferencedAssemblies System.Windows.Forms -TypeDefinition @'
using System;
using System.Threading;
using System.Windows.Forms;
using System.Runtime.InteropServices;
public sealed class OffGridProbeFixture : IDisposable {
  [DllImport("kernel32.dll")] static extern uint WaitForSingleObject(IntPtr handle, uint timeout);
  readonly ManualResetEvent ready = new ManualResetEvent(false);
  readonly ManualResetEvent release = new ManualResetEvent(false);
  readonly Thread thread;
  public IntPtr Window;
  public OffGridProbeFixture(bool visible, bool blocked) {
    thread = new Thread(() => {
      using (var form = new Form()) {
        form.Text = "OffGrid Desktop Install Test responsiveness fixture";
        form.ShowInTaskbar = false;
        Window = form.Handle;
        if (visible) form.Show();
        ready.Set();
        // Managed STA waits may pump COM/window messages; use a native wait.
        if (blocked) WaitForSingleObject(release.SafeWaitHandle.DangerousGetHandle(), 90000);
        else while (!release.WaitOne(10)) Application.DoEvents();
      }
    });
    thread.IsBackground = true;
    thread.SetApartmentState(ApartmentState.STA);
    thread.Start();
    if (!ready.WaitOne(5000)) throw new Exception("Fixture did not start");
  }
  public void Dispose() { release.Set(); thread.Join(5000); }
}
'@
$hiddenWindow = [OffGridProbeFixture]::new($false, $true)
$visibleWindow = [OffGridProbeFixture]::new($true, $true)
$healthyWindow = [OffGridProbeFixture]::new($true, $false)
try {
    [IO.File]::WriteAllText((Join-Path $FixtureDirectory 'handles.tmp'), (@{
        hidden = $hiddenWindow.Window.ToInt64(); visible = $visibleWindow.Window.ToInt64(); healthy = $healthyWindow.Window.ToInt64()
    } | ConvertTo-Json))
    [IO.File]::Move((Join-Path $FixtureDirectory 'handles.tmp'), (Join-Path $FixtureDirectory 'handles.json'))
    $deadline = [DateTime]::UtcNow.AddSeconds(80)
    while (-not (Test-Path (Join-Path $FixtureDirectory 'stop')) -and [DateTime]::UtcNow -lt $deadline) { Start-Sleep -Milliseconds 100 }
} finally {
    $hiddenWindow.Dispose()
    $visibleWindow.Dispose()
    $healthyWindow.Dispose()
}
exit
}

. (Join-Path $PSScriptRoot 'windows-installer-ui.ps1')

# Deterministic time-series checks cover gaps, recovery and hidden/closed targets
# without waiting on the scheduler or pre-aging a real failure timestamp.
$timing = @{}
for ($sample = 0; $sample -lt 7; $sample++) {
    $start = $sample * 800
    $frozen = Update-WizardProbeState 'continuous' $timing $start ($start + 750) $true $false
    if ($frozen -ne ($sample -eq 6)) { throw "Continuous-hang timing failed at sample $sample" }
}
if (Update-WizardProbeState 'gap' $timing 0 750 $true $false) { throw 'One failed probe is not a sustained hang.' }
if (Update-WizardProbeState 'gap' $timing 6000 6750 $true $false) { throw 'Unsampled time was counted as a continuous hang.' }
if ($timing.gap.samples -ne 1 -or $timing.gap.sinceMs -ne 6000) { throw 'Monitoring gap did not reset the failure streak.' }
if (Update-WizardProbeState 'gap' $timing 6800 6801 $true $true) { throw 'Responsive target was rejected.' }
if ($timing.ContainsKey('gap')) { throw 'A successful probe did not clear failures.' }
[void](Update-WizardProbeState 'gap' $timing 6900 7650 $true $false)
if ($timing.gap.samples -ne 1) { throw 'A transient miss reused the prior failure streak.' }
[void](Update-WizardProbeState 'gap' $timing 7700 7701 $false $false)
if ($timing.ContainsKey('gap')) { throw 'Hidden/closed target retained a failure.' }
Write-Output 'PASS: continuous hang, transient miss, recovery, monitoring gap, hidden/closed target timing.'

$directory = (New-Item -ItemType Directory -Path (Join-Path ([IO.Path]::GetTempPath()) ('offgrid-wizard-probe-' + [Guid]::NewGuid().ToString('N')))).FullName
$diagnosticPath = Join-Path $directory 'events.jsonl'
$fixture = Start-Process powershell.exe -ArgumentList "-NoProfile -File `"$PSCommandPath`" -FixtureDirectory `"$directory`"" -WindowStyle Hidden -PassThru
$capture = $null
try {
    $deadline = [DateTime]::UtcNow.AddSeconds(10)
    $handlesPath = Join-Path $directory 'handles.json'
    while (-not (Test-Path $handlesPath) -and [DateTime]::UtcNow -lt $deadline) { Start-Sleep -Milliseconds 100 }
    $handles = Get-Content $handlesPath -Raw | ConvertFrom-Json
    $hidden = [IntPtr]::new([long]$handles.hidden)
    $visible = [IntPtr]::new([long]$handles.visible)
    $healthy = [IntPtr]::new([long]$handles.healthy)
    $clock = [Diagnostics.Stopwatch]::StartNew()
    $failures = @{}
    $hiddenKey = "$($hidden.ToInt64())/$([OffGridWizard]::Owner($hidden))"
    $failures[$hiddenKey] = @{ sinceMs = -6000L; lastCompletedMs = 0L; samples = 10 }
    Assert-WizardResponsive $hidden $failures $clock 'probe failed' $diagnosticPath
    if ($failures.Count -ne 0) { throw 'Hidden maintenance window retained a failure.' }
    $rejected = $false
    while ($clock.ElapsedMilliseconds -lt 15000 -and -not $rejected) {
        try { Assert-WizardResponsive $visible $failures $clock 'probe failed' $diagnosticPath }
        catch {
            if ($_.Exception.Message -notlike 'probe failed*visible=True*') { throw }
            $rejected = $true
        }
        Start-Sleep -Milliseconds 50
    }
    if (-not $rejected) { throw 'Visible frozen UI was incorrectly accepted.' }
    if ($clock.ElapsedMilliseconds -lt 5000) { throw 'Visible hang rejected before sustained failure was measured.' }
    Write-Output 'PASS: hidden maintenance window ignored; real sustained visible hang rejected.'

    # A blocked window also blocks PrintWindow. Prove that its owned capture
    # helper is bounded while a healthy window can still be monitored.
    $capture = Start-WizardCapture $visible (Join-Path $directory 'blocked.png') $diagnosticPath
    $probesDuringCapture = 0
    while (-not $capture.done) {
        Assert-WizardResponsive $healthy $failures $clock 'Healthy fixture became unresponsive' $diagnosticPath
        $probesDuringCapture++
        Update-WizardCapture $capture
        Start-Sleep -Milliseconds 50
    }
    if ($capture.status -ne 'timed-out') { throw "Blocked capture did not hit its deadline: $($capture.status)" }
    if ($probesDuringCapture -lt 2) { throw 'Capture blocked independent monitoring.' }
    $capture = Start-WizardCapture $healthy (Join-Path $directory 'healthy.png') $diagnosticPath
    while (-not $capture.done) { Update-WizardCapture $capture; Start-Sleep -Milliseconds 50 }
    if ($capture.status -ne 'captured') { throw "Healthy window capture failed: $($capture.status); evidence: $directory" }
    $image = [Drawing.Image]::FromFile((Join-Path $directory 'healthy.png'))
    try { if ($image.Width -lt 1 -or $image.Height -lt 1) { throw 'Empty fixture capture.' } } finally { $image.Dispose() }
    $capture = Start-WizardCapture ([IntPtr]::Zero) (Join-Path $directory 'invalid.png') $diagnosticPath
    while (-not $capture.done) { Update-WizardCapture $capture; Start-Sleep -Milliseconds 50 }
    if ($capture.status -ne 'failed' -or (Test-Path -LiteralPath $capture.output)) { throw 'Invalid capture target was not rejected.' }
    Write-Output "PASS: blocked capture bounded; $probesDuringCapture independent probes; healthy capture saved."
    Write-Output 'PASS: invalid capture target refused without saving an image.'
} finally {
    if ($capture -and -not $capture.done) { Update-WizardCapture $capture -Stop }
    [IO.File]::WriteAllText((Join-Path $directory 'stop'), '')
    if (-not $fixture.WaitForExit(5000)) { throw "Fixture did not exit; inspect $directory" }
    if ($fixture.ExitCode -ne 0) { throw "Fixture exited with $($fixture.ExitCode)" }
}
