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
    Rect r; GetWindowRect(window, out r);
    using (var bitmap = new System.Drawing.Bitmap(r.right-r.left, r.bottom-r.top)) {
      using (var graphics = System.Drawing.Graphics.FromImage(bitmap)) {
        var dc = graphics.GetHdc(); try { PrintWindow(window, dc, 2); } finally { graphics.ReleaseHdc(dc); }
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
  [DllImport("user32.dll")] public static extern bool PostMessage(IntPtr window, uint message, IntPtr w, IntPtr l);
  [DllImport("user32.dll")] static extern IntPtr SendMessageTimeout(IntPtr window, uint message, IntPtr w, IntPtr l, uint flags, uint timeout, out IntPtr result);
  public static string Text(IntPtr window) { var text = new StringBuilder(2048); GetWindowText(window, text, text.Capacity); return text.ToString(); }
  public static string Class(IntPtr window) { var text = new StringBuilder(256); GetClassName(window, text, text.Capacity); return text.ToString(); }
  public static int Items(IntPtr window) { IntPtr result; SendMessageTimeout(window, 0x1004, IntPtr.Zero, IntPtr.Zero, 2, 200, out result); return result.ToInt32(); }
  public static IntPtr[] Windows(string prefix) { var found = new List<IntPtr>(); EnumWindows((w,p) => { if (Text(w).StartsWith(prefix, StringComparison.Ordinal)) found.Add(w); return true; }, IntPtr.Zero); return found.ToArray(); }
  public static IntPtr[] Children(IntPtr window) { var found = new List<IntPtr>(); EnumChildWindows(window, (w,p) => { found.Add(w); return true; }, IntPtr.Zero); return found.ToArray(); }
  public static bool Responsive(IntPtr window, uint timeout) { IntPtr result; return SendMessageTimeout(window, 0, IntPtr.Zero, IntPtr.Zero, 2, timeout, out result) != IntPtr.Zero; }
  public static void Check(IntPtr window, bool value) { IntPtr result; SendMessageTimeout(window, 0xf1, value ? new IntPtr(1) : IntPtr.Zero, IntPtr.Zero, 2, 200, out result); }
  public static void Click(IntPtr window) { if (!PostMessage(window, 0xf5, IntPtr.Zero, IntPtr.Zero)) throw new Exception("Could not click test wizard control"); }
}
'@

function Assert-WizardResponsive([IntPtr]$Window, [hashtable]$Failures, [Diagnostics.Stopwatch]$Clock, [string]$Message) {
    $key = $Window.ToInt64().ToString()
    if (-not [OffGridWizard]::IsWindow($Window) -or [OffGridWizard]::Responsive($Window, 750)) {
        [void]$Failures.Remove($key)
        return
    }
    if (-not $Failures.ContainsKey($key)) {
        $Failures[$key] = $Clock.ElapsedMilliseconds
        return
    }
    # A busy CI runner can miss a short WM_NULL deadline while NSIS is
    # extracting files. Only a sustained failure is a genuine frozen wizard.
    if ($Clock.ElapsedMilliseconds - [long]$Failures[$key] -ge 5000) { throw $Message }
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
    while ($clock.Elapsed.TotalSeconds -lt 180) {
        if ($finishAt -ge 0 -and $finishClosedMs -lt 0) {
            if (-not [OffGridWizard]::IsWindow($finishWindow)) { $finishClosedMs = $clock.ElapsedMilliseconds - $finishAt }
            elseif ($clock.ElapsedMilliseconds - $finishAt -gt 2000) { throw 'Finish did not close the installer window within 2 seconds.' }
        }
        if ($process.HasExited) { break }
        foreach ($window in [OffGridWizard]::Windows('OffGrid Desktop Install Test')) {
            Assert-WizardResponsive $window $unresponsive $clock 'Installer UI stopped responding for at least 5 seconds.'
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
                        if ($items -ge 5) { [OffGridWizard]::Capture($window, (Join-Path (Split-Path $InstallRoot) 'installer-details.png')) }
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
                [OffGridWizard]::Capture($window, (Join-Path (Split-Path $InstallRoot) 'installer-finish.png'))
                [OffGridWizard]::Check($checks[0], $Launch)
                $finishAt = $clock.ElapsedMilliseconds
                $finishWindow = $window
            } elseif ($label -notin @('Next >', 'Install')) {
                throw "Unexpected wizard page ($label): $text"
            }
            $clicked[$key] = $true
            [OffGridWizard]::Click($button)
        }
        Start-Sleep -Milliseconds 50
    }
    if (-not $process.HasExited) { throw "Installer timed out (PID $($process.Id)); retained for diagnosis." }
    if ($process.ExitCode -ne 0) { throw "Wizard exited with $($process.ExitCode)." }
    if ($finishAt -lt 0) { throw 'Wizard never reached Finish.' }
    if ($finishClosedMs -lt 0) { $finishClosedMs = $clock.ElapsedMilliseconds - $finishAt }
    if ($finishClosedMs -gt 2000) { throw "Finish blocked for ${finishClosedMs}ms." }
    if ($consented -ne $ExpectRunningPrompt) { throw 'Running-app consent was not exercised as expected.' }
    if (-not $detailsOpened -or $detailsItems -lt 1) { throw "Show details did not display installation activity (clicked=$detailsOpened, items=$detailsItems, controls=$detailsObserved)." }
    [PSCustomObject]@{ finishClosedMs = $finishClosedMs; elapsedMs = $clock.ElapsedMilliseconds; launched = $Launch; runningAppConsent = $consented; detailsItems = $detailsItems }
}
