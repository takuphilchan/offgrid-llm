param([Parameter(Mandatory = $true)][string]$Root)
$ErrorActionPreference = 'Stop'
$fixtureRoot = [IO.Path]::GetFullPath($Root)
if ([IO.Path]::GetDirectoryName($fixtureRoot) -ne [IO.Path]::GetTempPath().TrimEnd('\') -or -not [IO.Path]::GetFileName($fixtureRoot).StartsWith('offgrid-recovery-electron-')) {
    throw 'Disposable recovery fixture root required'
}
# Hold one actual extracted file without FILE_SHARE_DELETE. This creates a real
# Windows cleanup failure; it never modifies permissions or production code.
$deadline = [DateTime]::UtcNow.AddSeconds(30)
[Console]::Out.WriteLine('ready')
$handle = $null
try {
    while ($null -eq $handle -and [DateTime]::UtcNow -lt $deadline) {
        foreach ($stage in [IO.Directory]::GetDirectories($fixtureRoot, '.offgrid-restore-*')) {
            $file = [IO.Path]::Combine($stage, 'payload.bin')
            try {
                $handle = [IO.File]::Open($file, [IO.FileMode]::Open, [IO.FileAccess]::Read, [IO.FileShare]::ReadWrite)
                [Console]::Out.WriteLine((@{stage = $stage; file = $file} | ConvertTo-Json -Compress))
                break
            } catch [IO.IOException] { }
        }
        if ($null -eq $handle) { Start-Sleep -Milliseconds 2 }
    }
    if ($null -eq $handle) { throw 'No restore payload observed before deadline' }
    [void][Console]::ReadLine()
} finally {
    if ($null -ne $handle) { $handle.Dispose() }
}
