# Native wizard driver for the deliberately isolated test identity only.
# Post button clicks instead of blocking SendMessage calls: a stuck Finish
# callback must fail the test, not freeze the test runner along with Setup.
Add-Type -ReferencedAssemblies System.Drawing -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Text;
public static class OffGridWizard {
  [StructLayout(LayoutKind.Sequential)] public struct Rect { public int left, top, right, bottom; }
  [DllImport("user32.dll")] static extern bool GetWindowRect(IntPtr window, out Rect rect);
  [DllImport("user32.dll")] static extern bool PrintWindow(IntPtr window, IntPtr dc, uint flags);
  [DllImport("user32.dll")] static extern IntPtr GetWindowDpiAwarenessContext(IntPtr window);
  [DllImport("user32.dll")] static extern IntPtr SetThreadDpiAwarenessContext(IntPtr context);
  [DllImport("user32.dll")] static extern int GetAwarenessFromDpiAwarenessContext(IntPtr context);
  public static bool DpiAware(IntPtr window) { return GetAwarenessFromDpiAwarenessContext(GetWindowDpiAwarenessContext(window)) > 0; }
  public static void Capture(IntPtr window, string path) {
    var previousDpi = SetThreadDpiAwarenessContext(new IntPtr(-4));
    try {
    Rect r;
    if (!GetWindowRect(window, out r) || r.right <= r.left || r.bottom <= r.top) throw new InvalidOperationException("Capture window has no valid bounds.");
    using (var bitmap = new System.Drawing.Bitmap(r.right-r.left, r.bottom-r.top)) {
      using (var graphics = System.Drawing.Graphics.FromImage(bitmap)) {
        var dc = graphics.GetHdc();
        try { if (!PrintWindow(window, dc, 2)) throw new InvalidOperationException("Window capture failed."); }
        finally { graphics.ReleaseHdc(dc); }
      }
      bitmap.Save(path, System.Drawing.Imaging.ImageFormat.Png);
    }
    } finally { SetThreadDpiAwarenessContext(previousDpi); }
  }
  public delegate bool EnumProc(IntPtr window, IntPtr parameter);
  [DllImport("user32.dll")] static extern bool EnumWindows(EnumProc callback, IntPtr parameter);
  [DllImport("user32.dll")] static extern bool EnumChildWindows(IntPtr window, EnumProc callback, IntPtr parameter);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] static extern int GetWindowText(IntPtr window, StringBuilder text, int max);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] static extern int GetClassName(IntPtr window, StringBuilder text, int max);
  [DllImport("user32.dll")] public static extern IntPtr GetDlgItem(IntPtr window, int id);
  [DllImport("user32.dll")] public static extern bool IsWindow(IntPtr window);
  [DllImport("user32.dll")] public static extern bool IsWindowEnabled(IntPtr window);
  [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr window);
  [DllImport("user32.dll")] static extern uint GetWindowThreadProcessId(IntPtr window, out uint process);
  [DllImport("kernel32.dll")] static extern void SetLastError(uint error);
  [DllImport("user32.dll")] public static extern bool PostMessage(IntPtr window, uint message, IntPtr w, IntPtr l);
  [DllImport("user32.dll", SetLastError=true)] static extern IntPtr SendMessageTimeout(IntPtr window, uint message, IntPtr w, IntPtr l, uint flags, uint timeout, out IntPtr result);
  public static string Text(IntPtr window) { var text = new StringBuilder(2048); GetWindowText(window, text, text.Capacity); return text.ToString(); }
  public static string Class(IntPtr window) { var text = new StringBuilder(256); GetClassName(window, text, text.Capacity); return text.ToString(); }
  public static int Items(IntPtr window) { IntPtr result; SendMessageTimeout(window, 0x1004, IntPtr.Zero, IntPtr.Zero, 2, 200, out result); return result.ToInt32(); }
  public static IntPtr[] Windows(string prefix) { var found = new List<IntPtr>(); EnumWindows((w,p) => { if (Text(w).StartsWith(prefix, StringComparison.Ordinal)) found.Add(w); return true; }, IntPtr.Zero); return found.ToArray(); }
  public static IntPtr[] Children(IntPtr window) { var found = new List<IntPtr>(); EnumChildWindows(window, (w,p) => { found.Add(w); return true; }, IntPtr.Zero); return found.ToArray(); }
  public static uint Owner(IntPtr window) { uint process; GetWindowThreadProcessId(window, out process); return process; }
  public sealed class ProbeResult { public bool Responded; public int Error; }
  public static ProbeResult Probe(IntPtr window, uint timeout) {
    IntPtr result;
    SetLastError(0);
    var answered = SendMessageTimeout(window, 0, IntPtr.Zero, IntPtr.Zero, 0x22, timeout, out result) != IntPtr.Zero;
    return new ProbeResult { Responded = answered, Error = answered ? 0 : Marshal.GetLastWin32Error() };
  }
  public static void Check(IntPtr window, bool value) { IntPtr result; SendMessageTimeout(window, 0xf1, value ? new IntPtr(1) : IntPtr.Zero, IntPtr.Zero, 2, 200, out result); }
  public static void Click(IntPtr window) { if (!PostMessage(window, 0xf5, IntPtr.Zero, IntPtr.Zero)) throw new Exception("Could not click test wizard control"); }
}
'@

function Write-WizardDiagnostic([string]$Path, [string]$Event, [hashtable]$Detail) {
    if (-not $Path) { return }
    $record = @{ time = [DateTime]::UtcNow.ToString('o'); event = $Event; detail = $Detail } | ConvertTo-Json -Depth 5 -Compress
    [IO.File]::AppendAllText($Path, $record + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
}

# Pure timing policy, separate from Win32 and screenshot collection. Time spent
# outside the probe loop is not evidence of a continuously unresponsive target.
function Update-WizardProbeState([string]$Key, [hashtable]$Failures, [long]$StartedMs, [long]$CompletedMs, [bool]$Eligible, [bool]$Responded) {
    if (-not $Eligible -or $Responded) {
        [void]$Failures.Remove($Key)
        return $false
    }
    if (-not $Failures.ContainsKey($Key) -or $StartedMs - $Failures[$Key].lastCompletedMs -gt 1000) {
        $Failures[$Key] = @{ sinceMs = $StartedMs; lastCompletedMs = $CompletedMs; samples = 1 }
    } else {
        $Failures[$Key].lastCompletedMs = $CompletedMs
        $Failures[$Key].samples++
    }
    return ($Failures[$Key].samples -ge 2 -and $CompletedMs - $Failures[$Key].sinceMs -ge 5000)
}

function Assert-WizardResponsive([IntPtr]$Window, [hashtable]$Failures, [Diagnostics.Stopwatch]$Clock, [string]$Message, [string]$DiagnosticPath = '') {
    $ownerId = [OffGridWizard]::Owner($Window)
    $key = "$($Window.ToInt64())/$ownerId"
    # Hidden maintenance windows remain subject to the overall process deadline,
    # not a visible UI deadline. Recheck after probing: windows can close mid-call.
    if (-not [OffGridWizard]::IsWindow($Window) -or -not [OffGridWizard]::IsWindowVisible($Window)) {
        [void]$Failures.Remove($key)
        return
    }
    $started = $Clock.ElapsedMilliseconds
    $probe = [OffGridWizard]::Probe($Window, 750)
    $completed = $Clock.ElapsedMilliseconds
    $eligible = [OffGridWizard]::IsWindow($Window) -and [OffGridWizard]::IsWindowVisible($Window) -and [OffGridWizard]::Owner($Window) -eq $ownerId
    $gap = if ($Failures.ContainsKey($key)) { $started - $Failures[$key].lastCompletedMs } else { 0 }
    Write-WizardDiagnostic $DiagnosticPath 'probe' @{ window = $Window.ToInt64(); owner = $ownerId; startedMs = $started; completedMs = $completed; answered = $probe.Responded; error = $probe.Error; eligible = $eligible; gapMs = $gap }
    # ERROR_TIMEOUT is 1460; Windows can also fail without setting an error.
    # Access denial/other API failures must not be mislabeled as a frozen UI.
    if ($eligible -and -not $probe.Responded -and $probe.Error -notin @(0, 1460)) {
        throw "Installer responsiveness probe failed. Window=$key; Win32Error=$($probe.Error)"
    }
    if (Update-WizardProbeState $key $Failures $started $completed $eligible $probe.Responded) {
        throw "$Message Window=$key; title=$([OffGridWizard]::Text($Window)); class=$([OffGridWizard]::Class($Window)); visible=True; samples=$($Failures[$key].samples); Win32Error=$($probe.Error)"
    }
}

# PrintWindow is synchronous and cannot be cancelled in-process. Only an owned
# capture helper may block on it; the watchdog never waits for that helper.
function Start-WizardCapture([IntPtr]$Window, [string]$OutputPath, [string]$DiagnosticPath) {
    $worker = Join-Path $PSScriptRoot 'windows-installer-capture.ps1'
    $ownerId = [OffGridWizard]::Owner($Window)
    $process = Start-Process powershell.exe -ArgumentList "-NoProfile -File `"$worker`" -Window $($Window.ToInt64()) -OwnerId $ownerId -OutputPath `"$OutputPath`"" -WindowStyle Hidden -PassThru -RedirectStandardOutput "$OutputPath.stdout.log" -RedirectStandardError "$OutputPath.stderr.log"
    # Retain the handle before the child exits. Windows PowerShell's asynchronous
    # Start-Process object can otherwise lose the exit code when polling HasExited.
    [void]$process.Handle
    Write-WizardDiagnostic $DiagnosticPath 'capture-start' @{ window = $Window.ToInt64(); owner = $ownerId; helper = $process.Id; output = $OutputPath }
    return [PSCustomObject]@{ process = $process; clock = [Diagnostics.Stopwatch]::StartNew(); output = $OutputPath; diagnosticPath = $DiagnosticPath; done = $false; status = 'running' }
}

function Update-WizardCapture($Capture, [switch]$Stop) {
    if ($Capture.done) { return }
    if ($Capture.process.HasExited) {
        $Capture.status = if ($Capture.process.ExitCode -eq 0 -and (Test-Path -LiteralPath $Capture.output)) { 'captured' } else { 'failed' }
    } elseif ($Stop -or $Capture.clock.ElapsedMilliseconds -ge 5000) {
        # This Process object is the exact child we launched, never the installer
        # or a user app located by name/port. Termination is bounded as well.
        try { $Capture.process.Kill() } catch { if (-not $Capture.process.HasExited) { throw } }
        if (-not $Capture.process.WaitForExit(2000)) { throw 'Capture helper did not stop within its deadline.' }
        $Capture.status = if ($Stop) { 'cancelled' } else { 'timed-out' }
    } else {
        return
    }
    $Capture.done = $true
    Write-WizardDiagnostic $Capture.diagnosticPath 'capture-end' @{ helper = $Capture.process.Id; status = $Capture.status; exitCode = $Capture.process.ExitCode; elapsedMs = $Capture.clock.ElapsedMilliseconds }
    if ($Capture.status -ne 'captured') { Write-Warning "Installer diagnostic capture $($Capture.status): $($Capture.output)" }
    $Capture.process.Dispose()
}

function Write-WizardHang([IntPtr]$Window, [string]$OutputPath, [string]$DiagnosticPath) {
    $worker = Join-Path $PSScriptRoot 'windows-installer-hang.ps1'
    $ownerId = [OffGridWizard]::Owner($Window)
    $helper = Start-Process powershell.exe -ArgumentList "-NoProfile -File `"$worker`" -Window $($Window.ToInt64()) -OwnerId $ownerId -OutputPath `"$OutputPath`"" -WindowStyle Hidden -PassThru -RedirectStandardOutput "$OutputPath.stdout.log" -RedirectStandardError "$OutputPath.stderr.log"
    try {
        [void]$helper.Handle
        $finished = $helper.WaitForExit(5000)
        if (-not $finished) {
            $helper.Kill()
            if (-not $helper.WaitForExit(2000)) { throw 'Hang diagnostic helper did not stop.' }
        }
        Write-WizardDiagnostic $DiagnosticPath 'hang-diagnostic' @{ completed = $finished; exitCode = $helper.ExitCode; output = $OutputPath }
    } finally { $helper.Dispose() }
}

function Invoke-InstallerWizard([string]$Executable, [string]$InstallRoot, [bool]$Launch, [bool]$ExpectRunningPrompt) {
    if ([Diagnostics.FileVersionInfo]::GetVersionInfo($Executable).ProductName -ne 'OffGrid Desktop Install Test') { throw 'Only isolated test installers are allowed.' }
    $process = Start-Process -FilePath $Executable -ArgumentList "/currentuser /D=$InstallRoot" -WindowStyle Hidden -PassThru
    $clock = [Diagnostics.Stopwatch]::StartNew()
    $finishAt = -1L
    $finishPageAt = -1L
    $finishWindow = [IntPtr]::Zero
    $finishClosedMs = -1L
    $consented = $false
    $detailsOpened = $false
    $detailsItems = 0
    $detailsObserved = ''
    $clicked = @{}
    $unresponsive = @{}
    $evidence = New-Item -ItemType Directory -Path (Join-Path (Split-Path $InstallRoot) ('diagnostics/wizard-' + [Guid]::NewGuid().ToString('N'))) -Force
    $diagnosticPath = Join-Path $evidence.FullName 'events.jsonl'
    $captures = [Collections.Generic.List[object]]::new()
    $detailsCapture = $null
    $finishCapture = $null
    Write-WizardDiagnostic $diagnosticPath 'wizard-start' @{ process = $process.Id; executable = $Executable; launch = $Launch; runningAppPrompt = $ExpectRunningPrompt }
    try {
        while ($clock.Elapsed.TotalSeconds -lt 180) {
            foreach ($capture in $captures) { Update-WizardCapture $capture }
            if ($finishAt -ge 0 -and $finishClosedMs -lt 0) {
                if (-not [OffGridWizard]::IsWindow($finishWindow)) { $finishClosedMs = $clock.ElapsedMilliseconds - $finishAt }
                elseif ($clock.ElapsedMilliseconds - $finishAt -gt 2000) { throw 'Finish did not close the installer window within 2 seconds.' }
            }
            if ($process.HasExited) { break }
            foreach ($window in [OffGridWizard]::Windows('OffGrid Desktop Install Test')) {
                Assert-WizardResponsive $window $unresponsive $clock 'Installer UI failed consecutive responsiveness probes for at least 5 seconds.' $diagnosticPath
                if (-not [OffGridWizard]::IsWindowVisible($window)) { continue }
                $allChildren = [OffGridWizard]::Children($window)
                $detailControls = @($allChildren | Where-Object { [OffGridWizard]::Text($_) -match 'detail' -or [OffGridWizard]::Class($_) -match 'List' })
                if ($detailControls.Count -gt 0) { $detailsObserved = ($detailControls | ForEach-Object { "$( [OffGridWizard]::Class($_) ):$( [OffGridWizard]::Text($_) ):visible=$( [OffGridWizard]::IsWindowVisible($_) ):enabled=$( [OffGridWizard]::IsWindowEnabled($_) )" }) -join '; ' }
                if (-not $detailsOpened) {
                    $details = @($allChildren | Where-Object { [OffGridWizard]::Text($_).Replace('&', '') -eq 'Show details' -and [OffGridWizard]::IsWindowVisible($_) })
                    if ($details.Count -eq 1 -and [OffGridWizard]::IsWindowEnabled($details[0])) { [OffGridWizard]::Click($details[0]); $detailsOpened = $true }
                }
                if ($detailsOpened) {
                    foreach ($list in @($allChildren | Where-Object { [OffGridWizard]::Class($_) -eq 'SysListView32' -and [OffGridWizard]::IsWindowVisible($_) })) {
                        $items = [OffGridWizard]::Items($list)
                        if ($items -gt $detailsItems) {
                            $detailsItems = $items
                            Write-WizardDiagnostic $diagnosticPath 'details-progress' @{ window = $window.ToInt64(); items = $items; elapsedMs = $clock.ElapsedMilliseconds }
                            if ($items -ge 5 -and -not $detailsCapture) {
                                $detailsCapture = Start-WizardCapture $window (Join-Path $evidence.FullName 'installer-details.png') $diagnosticPath
                                $captures.Add($detailsCapture)
                            }
                        }
                    }
                }
                $button = [OffGridWizard]::GetDlgItem($window, 1)
                if ($button -eq [IntPtr]::Zero -or -not [OffGridWizard]::IsWindowEnabled($button)) { continue }
                $children = [OffGridWizard]::Children($window)
                $text = ($children | ForEach-Object { [OffGridWizard]::Text($_) }) -join "`n"
                $label = [OffGridWizard]::Text($button).Replace('&', '')
                # The native page can destroy its controls between enumeration and
                # text retrieval. Ignore that transition; the overall/Finish deadlines
                # still fail genuinely stuck windows.
                if ($label -eq '' -or -not [OffGridWizard]::IsWindow($button)) { continue }
                $key = "$window/$label"
                if ($clicked.ContainsKey($key)) { continue }
                if ($text.Contains('Setup needs to close OffGrid')) {
                    if (-not $ExpectRunningPrompt) { throw 'Unexpected running-app prompt.' }
                    $consented = $true
                } elseif ($label -eq 'Finish') {
                    if ($finishPageAt -lt 0) { $finishPageAt = $clock.ElapsedMilliseconds }
                    if (-not [OffGridWizard]::DpiAware($window)) { throw 'Installer is not DPI aware.' }
                    $checks = @($children | Where-Object { [OffGridWizard]::Text($_).Replace('&', '') -match '^(Run|Launch) ' })
                    # NSIS can enable Finish one message-loop turn before it creates
                    # the launch checkbox. Wait for the page to settle, but keep a
                    # bounded failure for a genuinely incomplete installer page.
                    if ($checks.Count -eq 0 -and $clock.ElapsedMilliseconds - $finishPageAt -lt 10000) { continue }
                    if ($checks.Count -ne 1) { throw "Expected launch checkbox on Finish, got $($checks.Count): $text" }
                    $visibleText = ($children | Where-Object { [OffGridWizard]::IsWindowVisible($_) } | ForEach-Object { [OffGridWizard]::Text($_) }) -join "`n"
                    if (-not $visibleText.Contains('OffGrid installed') -or -not $visibleText.Contains('Separately managed services, including Docker, are not upgraded by this installer.')) {
                        if ($clock.ElapsedMilliseconds - $finishPageAt -lt 10000) { continue }
                        throw "Finish must display installation and external-service guidance: $visibleText"
                    }
                    # Keep probing while a bounded helper captures the settled page.
                    # Click Finish only afterwards; its two-second close gate remains.
                    if ($detailsCapture -and -not $detailsCapture.done) { continue }
                    if (-not $finishCapture) {
                        $finishCapture = Start-WizardCapture $window (Join-Path $evidence.FullName 'installer-finish.png') $diagnosticPath
                        $captures.Add($finishCapture)
                    }
                    if (-not $finishCapture.done) { continue }
                    [OffGridWizard]::Check($checks[0], $Launch)
                    $finishAt = $clock.ElapsedMilliseconds
                    $finishWindow = $window
                } elseif ($label -notin @('Next >', 'Install')) {
                    throw "Unexpected wizard page ($label): $text"
                }
                $clicked[$key] = $true
                Write-Host "Installer control: $label (window=$window, elapsed=$($clock.ElapsedMilliseconds)ms)"
                Write-WizardDiagnostic $diagnosticPath 'click' @{ window = $window.ToInt64(); label = $label; elapsedMs = $clock.ElapsedMilliseconds }
                [OffGridWizard]::Click($button)
            }
            Start-Sleep -Milliseconds 50
        }
        if (-not $process.HasExited) { throw "Installer timed out (PID $($process.Id)); retained for diagnosis." }
        if ($process.ExitCode -ne 0) { throw "Wizard exited with $($process.ExitCode)." }
        if ($finishAt -lt 0) { throw "Wizard never reached Finish (elapsed=$($clock.ElapsedMilliseconds)ms, controls=$($clicked.Keys -join ','), details=$detailsObserved)." }
        if ($finishClosedMs -lt 0) { $finishClosedMs = $clock.ElapsedMilliseconds - $finishAt }
        if ($finishClosedMs -gt 2000) { throw "Finish blocked for ${finishClosedMs}ms." }
        if ($consented -ne $ExpectRunningPrompt) { throw 'Running-app consent was not exercised as expected.' }
        if (-not $detailsOpened -or $detailsItems -lt 1) { throw "Show details did not display installation activity (clicked=$detailsOpened, items=$detailsItems, controls=$detailsObserved)." }
        Write-WizardDiagnostic $diagnosticPath 'wizard-passed' @{ elapsedMs = $clock.ElapsedMilliseconds; finishClosedMs = $finishClosedMs; detailsItems = $detailsItems }
        [PSCustomObject]@{ finishClosedMs = $finishClosedMs; elapsedMs = $clock.ElapsedMilliseconds; launched = $Launch; runningAppConsent = $consented; detailsItems = $detailsItems }
    } catch {
        Write-WizardDiagnostic $diagnosticPath 'wizard-failed' @{ message = $_.Exception.Message; elapsedMs = $clock.ElapsedMilliseconds; details = $detailsObserved; detailsItems = $detailsItems; processExited = $process.HasExited }
        # Preserve the failing state without changing the failure or asking the
        # blocked window to paint. The separate read-only helper is bounded.
        foreach ($failedWindow in [OffGridWizard]::Windows('OffGrid Desktop Install Test')) {
            try { Write-WizardHang $failedWindow (Join-Path $evidence.FullName "hang-$($failedWindow.ToInt64()).json") $diagnosticPath }
            catch { Write-Warning "Could not collect installer wait chain: $($_.Exception.Message)" }
        }
        throw
    } finally {
        foreach ($capture in $captures) { Update-WizardCapture $capture -Stop }
    }
}
