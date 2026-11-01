# Assemble and verify the CPU worker's app-local dependencies. Never search PATH
# or copy DLLs from installed third-party software or the user's system directory.
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$BundleDirectory,
    [Parameter(Mandatory = $true)][string]$CMakeCache,
    [switch]$VerifyOnly
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$bundle = (Resolve-Path -LiteralPath $BundleDirectory).Path
$cache = Get-Content -LiteralPath $CMakeCache
$toolLine = @($cache | Where-Object { $_ -match '^CMAKE_LINKER:FILEPATH=' })
if ($toolLine.Count -ne 1) { throw 'Expected one CMake-selected linker path' }
$compilerTool = ($toolLine[0] -split '=', 2)[1] -replace '/', '\'
if ($compilerTool -notmatch '^(.*)\\VC\\Tools\\MSVC\\([^\\]+)\\bin\\Hostx64\\x64\\link.exe$') {
    throw 'The Windows CPU bundle requires the CMake-selected x64 MSVC linker'
}
$visualStudio = $Matches[1]
$compilerRevision = $Matches[2]
$dumpbin = Join-Path (Split-Path -Parent $compilerTool) 'dumpbin.exe'
if (-not (Test-Path -LiteralPath $dumpbin -PathType Leaf)) { throw 'Compiler-matching dumpbin is unavailable' }
$redistRevision = (Get-Content -LiteralPath (Join-Path $visualStudio 'VC/Auxiliary/Build/Microsoft.VCRedistVersion.default.txt') -Raw).Trim()
if ($redistRevision -notmatch '^\d+\.\d+\.\d+$') { throw 'Invalid Microsoft redistributable revision' }
$redist = Join-Path $visualStudio "VC/Redist/MSVC/$redistRevision/x64"
$runtimeFiles = @{}
foreach ($group in Get-ChildItem -LiteralPath $redist -Directory) {
    if ($group.Name -notmatch '^Microsoft\.VC\d+\.(CRT|OpenMP)$') { continue }
    foreach ($dll in Get-ChildItem -LiteralPath $group.FullName -File -Filter '*.dll') {
        if ($runtimeFiles.ContainsKey($dll.Name)) { throw "Ambiguous redistributable: $($dll.Name)" }
        $runtimeFiles[$dll.Name] = $dll.FullName
    }
}

# Windows-owned libraries and API-set forwarders only. C++/OpenMP/OpenSSL are
# deliberately not system exceptions even when this build machine has them.
$systemLibraries = @('kernel32.dll', 'kernelbase.dll', 'ntdll.dll', 'advapi32.dll',
    'shell32.dll', 'shlwapi.dll', 'user32.dll', 'userenv.dll', 'gdi32.dll',
    'ole32.dll', 'oleaut32.dll', 'ws2_32.dll', 'bcrypt.dll', 'crypt32.dll',
    'secur32.dll', 'rpcrt4.dll', 'msvcrt.dll', 'ucrtbase.dll', 'version.dll',
    'winmm.dll', 'psapi.dll', 'normaliz.dll')
$queue = [System.Collections.Generic.Queue[string]]::new()
$queue.Enqueue('llama-server.exe')
$visited = @{}
$records = [System.Collections.Generic.List[object]]::new()
while ($queue.Count -gt 0) {
    $name = $queue.Dequeue()
    if ($visited.ContainsKey($name)) { continue }
    $visited[$name] = $true
    $file = Join-Path $bundle $name
    if (-not (Test-Path -LiteralPath $file -PathType Leaf)) { throw "Missing bundled runtime: $name" }
    if ((Get-Item -LiteralPath $file).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw "Linked runtime is not portable: $name" }
    $output = & $dumpbin /nologo /dependents $file 2>&1
    if ($LASTEXITCODE -ne 0) { throw "Cannot inspect PE dependencies: $name" }
    $dependencies = @($output | ForEach-Object {
        if ($_ -match '^\s+([A-Za-z0-9_.-]+\.dll)\s*$') { $Matches[1].ToLowerInvariant() }
    } | Sort-Object -Unique)
    if (-not $dependencies.Count) { throw "No inspectable dependencies in $name" }
    foreach ($dependency in $dependencies) {
        if ($systemLibraries -contains $dependency -or $dependency -match '^(api|ext)-ms-win-[a-z0-9-]+\.dll$') { continue }
        if (-not $runtimeFiles.ContainsKey($dependency)) {
            throw "Unapproved worker dependency $dependency in $name; do not resolve it from PATH"
        }
        $target = Join-Path $bundle $dependency
        $source = $runtimeFiles[$dependency]
        if (-not (Test-Path -LiteralPath $target)) {
            if ($VerifyOnly) { throw "Missing app-local dependency $dependency required by $name" }
            Copy-Item -LiteralPath $source -Destination $target
        }
        if ((Get-FileHash -LiteralPath $source -Algorithm SHA256).Hash -ne (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash) {
            throw "Bundled $dependency differs from the compiler's redistributable"
        }
        $queue.Enqueue($dependency)
    }
    $records.Add([ordered]@{name=$name; sha256=(Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash.ToLowerInvariant(); dependencies=$dependencies})
}

# Smoke-check the exact bundle with no developer PATH, OpenSSL or VS environment.
# The dependency walk above also catches DLLs installed globally in System32.
$start = [Diagnostics.ProcessStartInfo]::new()
$start.FileName = Join-Path $bundle 'llama-server.exe'
$start.Arguments = '--version'
$start.WorkingDirectory = $bundle
$start.UseShellExecute = $false
$start.CreateNoWindow = $true
$start.RedirectStandardOutput = $true
$start.RedirectStandardError = $true
$start.EnvironmentVariables.Clear()
$start.EnvironmentVariables['SystemRoot'] = $env:SystemRoot
$start.EnvironmentVariables['WINDIR'] = $env:SystemRoot
$start.EnvironmentVariables['PATH'] = Join-Path $env:SystemRoot 'System32'
$start.EnvironmentVariables['TEMP'] = [IO.Path]::GetTempPath()
$start.EnvironmentVariables['TMP'] = [IO.Path]::GetTempPath()
$process = [Diagnostics.Process]::new()
$process.StartInfo = $start
try {
    if (-not $process.Start()) { throw 'Worker smoke check could not start' }
    $stdout = $process.StandardOutput.ReadToEndAsync()
    $stderr = $process.StandardError.ReadToEndAsync()
    if (-not $process.WaitForExit(15000)) { $process.Kill(); $process.WaitForExit(); throw 'Worker version check exceeded 15 seconds' }
    $version = $stdout.Result + $stderr.Result
    if ($process.ExitCode -ne 0 -or $version -notmatch 'version:') {
        throw "Worker version check failed ($($process.ExitCode)): $version"
    }
} finally { $process.Dispose() }
$manifest = [ordered]@{schema_version=1; platform='windows-amd64-cpu'; compiler_revision=$compilerRevision; redist_revision=$redistRevision; files=@($records); smoke='version-only; no model inference'; version=$version.Trim()}
if (-not $VerifyOnly) {
    $json = $manifest | ConvertTo-Json -Depth 6
    [IO.File]::WriteAllText((Join-Path $bundle 'runtime-dependencies.json'), $json + "`n", [Text.UTF8Encoding]::new($false))
}
Write-Output "Windows worker dependency closure and isolated version check passed ($($records.Count) files)"
