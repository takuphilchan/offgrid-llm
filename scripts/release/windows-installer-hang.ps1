param(
    [Parameter(Mandatory = $true)][long]$Window,
    [Parameter(Mandatory = $true)][uint32]$OwnerId,
    [Parameter(Mandatory = $true)][string]$OutputPath
)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'windows-installer-ui.ps1')
$target = [IntPtr]::new($Window)
if (-not [OffGridWizard]::IsWindow($target) -or [OffGridWizard]::Owner($target) -ne $OwnerId -or
    -not [OffGridWizard]::Text($target).StartsWith('OffGrid Desktop Install Test', [StringComparison]::Ordinal)) {
    throw 'Hang diagnostics require the original isolated installer window.'
}

# This helper has a deadline enforced by its parent. WCT follows only the test
# process's threads; do not capture memory, other windows, or named lock objects.
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class OffGridWaitChain {
  // WAITCHAIN_NODE_INFO: two DWORD enums and a 528-byte, 8-byte-aligned union.
  // Only the thread fields are exposed; lock names can contain private paths.
  [StructLayout(LayoutKind.Explicit, Size=536)] public struct Node {
    [FieldOffset(0)] public int Type;
    [FieldOffset(4)] public int Status;
    [FieldOffset(8)] public uint Process;
    [FieldOffset(12)] public uint Thread;
    [FieldOffset(16)] public uint WaitTime;
    [FieldOffset(20)] public uint ContextSwitches;
  }
  [DllImport("advapi32.dll", SetLastError=true)] static extern IntPtr OpenThreadWaitChainSession(uint flags, IntPtr callback);
  [DllImport("advapi32.dll")] static extern void CloseThreadWaitChainSession(IntPtr session);
  [DllImport("advapi32.dll", SetLastError=true)] static extern bool GetThreadWaitChain(IntPtr session, UIntPtr context, uint flags, uint thread, ref uint count, [Out] Node[] nodes, out bool cycle);
  [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr window, out uint process);
  public sealed class Result { public bool Success; public int Error; public bool Cycle; public Node[] Nodes; }
  public static Result Read(uint thread) {
    var session = OpenThreadWaitChainSession(0, IntPtr.Zero);
    if (session == IntPtr.Zero) throw new System.ComponentModel.Win32Exception(Marshal.GetLastWin32Error());
    try {
      var nodes = new Node[16]; uint count = 16; bool cycle;
      bool success = GetThreadWaitChain(session, UIntPtr.Zero, 0, thread, ref count, nodes, out cycle);
      int error = success ? 0 : Marshal.GetLastWin32Error();
      Array.Resize(ref nodes, (int)Math.Min(count, 16));
      return new Result { Success = success, Error = error, Cycle = cycle, Nodes = nodes };
    } finally { CloseThreadWaitChainSession(session); }
  }
}
'@
$process = Get-Process -Id $OwnerId
[uint32]$observedOwner = 0
$uiThread = [OffGridWaitChain]::GetWindowThreadProcessId($target, [ref]$observedOwner)
if ($observedOwner -ne $OwnerId) { throw 'Installer window changed during diagnostics.' }
$chains = @($process.Threads | ForEach-Object {
    $threadId = $_.Id
    $chain = [OffGridWaitChain]::Read($threadId)
    @{ thread = $threadId; ui = $threadId -eq $uiThread; success = $chain.Success; error = $chain.Error; cycle = $chain.Cycle; nodes = @($chain.Nodes | ForEach-Object {
        $node = @{ type = $_.Type; status = $_.Status }
        if ($_.Type -eq 8) { $node.process = $_.Process; $node.thread = $_.Thread; $node.waitTime = $_.WaitTime }
        $node
    }) }
})
$controls = @([OffGridWizard]::Children($target) | ForEach-Object {
    @{ window = $_.ToInt64(); class = [OffGridWizard]::Class($_); text = [OffGridWizard]::Text($_); visible = [OffGridWizard]::IsWindowVisible($_); enabled = [OffGridWizard]::IsWindowEnabled($_) }
})
@{ process = $OwnerId; window = $Window; uiThread = $uiThread; chains = $chains; controls = $controls } |
    ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $OutputPath -Encoding UTF8
