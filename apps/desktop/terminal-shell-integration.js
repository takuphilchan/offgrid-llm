'use strict';

// Fixed first-party initialization, never assembled from terminal output or a
// model response. No profile edits, execution-policy changes or additional shell
// history persistence. Markers are NOT authenticated action receipts.
const powershellIntegration = String.raw`
function global:Send-OffGridMarker($value) {
  try {
    $json = ConvertTo-Json -InputObject $value -Compress
    $data = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($json))
    [Console]::Write([string][char]27 + ']633;OffGrid;' + $data + [char]7)
  } catch {}
}
$global:OffGridCommandPending = $false
function global:prompt {
  $ok = $?
  $nativeExit = $global:LASTEXITCODE
  if ($global:OffGridCommandPending) {
    # LASTEXITCODE survives cmdlet failures. Report an exact native code only
    # when it changed; otherwise preserve failure without inventing a code.
    $code = if ($ok) { 0 } elseif ($nativeExit -ne $global:OffGridPreviousExit) { $nativeExit } else { $null }
    Send-OffGridMarker @{v=1;event='end';exit=$code;failed=(-not $ok)}
    $global:OffGridCommandPending = $false
  }
  Send-OffGridMarker @{v=1;event='prompt';cwd=$PWD.Path}
  'PS ' + $PWD.Path + ('>' * ($nestedPromptLevel + 1)) + ' '
}
try {
  Import-Module PSReadLine -ErrorAction Stop
  Set-PSReadLineKeyHandler -Key Enter -ScriptBlock {
    $line = $null; $cursor = 0
    [Microsoft.PowerShell.PSConsoleReadLine]::GetBufferState([ref]$line, [ref]$cursor)
    $tokens = $null; $parseErrors = $null
    $null = [System.Management.Automation.Language.Parser]::ParseInput($line, [ref]$tokens, [ref]$parseErrors)
    $incomplete = @($parseErrors | Where-Object { $_.IncompleteInput }).Count -gt 0
    [Microsoft.PowerShell.PSConsoleReadLine]::AcceptLine()
    if (-not $incomplete -and -not [string]::IsNullOrWhiteSpace($line)) {
      $global:OffGridCommandPending = $true
      $global:OffGridPreviousExit = $global:LASTEXITCODE
      Send-OffGridMarker @{v=1;event='begin';command=$line.Substring(0,[Math]::Min(4096,$line.Length));cwd=$PWD.Path;truncated=($line.Length -gt 4096)}
    }
  }
} catch { Send-OffGridMarker @{v=1;event='unavailable'} }
`;
module.exports = { powershellIntegration };
