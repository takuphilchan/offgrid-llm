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
  public OffGridProbeFixture(bool visible) {
    thread = new Thread(() => {
      using (var form = new Form()) {
        form.Text = "OffGrid isolated responsiveness fixture";
        form.ShowInTaskbar = false;
        Window = form.Handle;
        if (visible) form.Show();
        ready.Set();
        // Managed STA waits may pump COM/window messages; use a native wait.
        WaitForSingleObject(release.SafeWaitHandle.DangerousGetHandle(), 45000);
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
$hiddenWindow = [OffGridProbeFixture]::new($false)
$visibleWindow = [OffGridProbeFixture]::new($true)
try {
    [IO.File]::WriteAllText((Join-Path $FixtureDirectory 'handles.tmp'), (@{
        hidden = $hiddenWindow.Window.ToInt64(); visible = $visibleWindow.Window.ToInt64()
    } | ConvertTo-Json))
    [IO.File]::Move((Join-Path $FixtureDirectory 'handles.tmp'), (Join-Path $FixtureDirectory 'handles.json'))
    $deadline = [DateTime]::UtcNow.AddSeconds(40)
    while (-not (Test-Path (Join-Path $FixtureDirectory 'stop')) -and [DateTime]::UtcNow -lt $deadline) { Start-Sleep -Milliseconds 100 }
} finally {
    $hiddenWindow.Dispose()
    $visibleWindow.Dispose()
}
exit
}

. (Join-Path $PSScriptRoot 'windows-installer-ui.ps1')
$directory = (New-Item -ItemType Directory -Path (Join-Path ([IO.Path]::GetTempPath()) ('offgrid-wizard-probe-' + [Guid]::NewGuid().ToString('N')))).FullName
$fixture = Start-Process powershell.exe -ArgumentList "-NoProfile -File `"$PSCommandPath`" -FixtureDirectory `"$directory`"" -WindowStyle Hidden -PassThru
try {
    $deadline = [DateTime]::UtcNow.AddSeconds(10)
    $handlesPath = Join-Path $directory 'handles.json'
    while (-not (Test-Path $handlesPath) -and [DateTime]::UtcNow -lt $deadline) { Start-Sleep -Milliseconds 100 }
    $handles = Get-Content $handlesPath -Raw | ConvertFrom-Json
    $hidden = [IntPtr]::new([long]$handles.hidden)
    $visible = [IntPtr]::new([long]$handles.visible)
    $clock = [Diagnostics.Stopwatch]::StartNew()
    $failures = @{}
    $failures[$hidden.ToInt64().ToString()] = -6000L
    Assert-WizardResponsive $hidden $failures $clock 'probe failed'
    if ($failures.Count -ne 0) { throw 'Hidden maintenance window retained a failure.' }
    $failures[$visible.ToInt64().ToString()] = -6000L
    $rejected = $false
    try { Assert-WizardResponsive $visible $failures $clock 'probe failed' }
    catch {
        if ($_.Exception.Message -notlike 'probe failed*visible=True*') { throw }
        $rejected = $true
    }
    if (-not $rejected) { throw 'Visible frozen UI was incorrectly accepted.' }
    Write-Output 'PASS: hidden maintenance window ignored; visible frozen UI rejected.'
} finally {
    [IO.File]::WriteAllText((Join-Path $directory 'stop'), '')
    if (-not $fixture.WaitForExit(5000)) { throw "Fixture did not exit; inspect $directory" }
    if ($fixture.ExitCode -ne 0) { throw "Fixture exited with $($fixture.ExitCode)" }
}
