# Real MSVC/OpenMP binaries exercise packaging without models or native input.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$work = Join-Path $root ('build/windows-runtime-smoke-' + [Guid]::NewGuid().ToString('N'))
$source = Join-Path $work 'source'
New-Item -ItemType Directory -Path $source -Force | Out-Null
@'
cmake_minimum_required(VERSION 3.20)
project(runtime_bundle_test CXX)
find_package(OpenMP REQUIRED)
add_executable(llama-server main.cpp)
target_link_libraries(llama-server PRIVATE OpenMP::OpenMP_CXX)
add_library(unbundled SHARED unbundled.cpp)
add_executable(unsafe-worker unsafe.cpp)
target_link_libraries(unsafe-worker PRIVATE unbundled)
'@ | Set-Content -LiteralPath (Join-Path $source 'CMakeLists.txt') -Encoding ASCII
@'
#include <cstdio>
#include <iostream>
#include <omp.h>
int main() { std::cout << "version: runtime-fixture " << omp_get_max_threads() << "\n"; return 0; }
'@ | Set-Content -LiteralPath (Join-Path $source 'main.cpp') -Encoding ASCII
'extern "C" __declspec(dllexport) int unbundled() { return 1; }' | Set-Content -LiteralPath (Join-Path $source 'unbundled.cpp') -Encoding ASCII
'extern "C" __declspec(dllimport) int unbundled(); int main() { return unbundled(); }' | Set-Content -LiteralPath (Join-Path $source 'unsafe.cpp') -Encoding ASCII
$build = Join-Path $work 'cmake'
& cmake -S $source -B $build -A x64 -T host=x64
if ($LASTEXITCODE -ne 0) { throw 'Fixture CMake configuration failed' }
& cmake --build $build --config Release --parallel 2
if ($LASTEXITCODE -ne 0) { throw 'Fixture compilation failed' }
$cache = Join-Path $build 'CMakeCache.txt'
$bundle = Join-Path $work 'bundle'
New-Item -ItemType Directory -Path $bundle | Out-Null
Copy-Item -LiteralPath (Join-Path $build 'Release/llama-server.exe') -Destination $bundle
$prepare = Join-Path $PSScriptRoot 'prepare-windows-runtime.ps1'
function Expect-Rejection([string]$pattern, [scriptblock]$operation) {
    $failure = $null
    try { & $operation } catch { $failure = $_.Exception.Message }
    if (-not $failure -or $failure -notmatch $pattern) { throw "Expected '$pattern', received '$failure'" }
}
Expect-Rejection 'Missing app-local dependency' { & $prepare -BundleDirectory $bundle -CMakeCache $cache -VerifyOnly }
if (@(Get-ChildItem -LiteralPath $bundle -File).Count -ne 1) { throw 'Verify-only modified the bundle' }
& $prepare -BundleDirectory $bundle -CMakeCache $cache
& $prepare -BundleDirectory $bundle -CMakeCache $cache -VerifyOnly
$manifest = Get-Content -LiteralPath (Join-Path $bundle 'runtime-dependencies.json') -Raw | ConvertFrom-Json
if ($manifest.files.name -notcontains 'vcomp140.dll') { throw 'OpenMP dependency was not exercised' }

$archive = Join-Path $work 'bundle.zip'
Compress-Archive -LiteralPath $bundle -DestinationPath $archive
$unpacked = Join-Path $work 'unpacked'
Expand-Archive -LiteralPath $archive -DestinationPath $unpacked
& $prepare -BundleDirectory (Join-Path $unpacked 'bundle') -CMakeCache $cache -VerifyOnly

# Keep every fixture recoverable: rename the DLL rather than deleting it.
$openmp = Join-Path $bundle 'vcomp140.dll'
Move-Item -LiteralPath $openmp -Destination ($openmp + '.saved')
Expect-Rejection 'Missing app-local dependency vcomp140.dll' { & $prepare -BundleDirectory $bundle -CMakeCache $cache -VerifyOnly }
Move-Item -LiteralPath ($openmp + '.saved') -Destination $openmp
[IO.File]::WriteAllBytes($openmp, [byte[]](1,2,3))
Expect-Rejection 'differs from the compiler' { & $prepare -BundleDirectory $bundle -CMakeCache $cache -VerifyOnly }

$unsafe = Join-Path $work 'unsafe'
New-Item -ItemType Directory -Path $unsafe | Out-Null
Copy-Item -LiteralPath (Join-Path $build 'Release/unsafe-worker.exe') -Destination (Join-Path $unsafe 'llama-server.exe')
Copy-Item -LiteralPath (Join-Path $build 'Release/unbundled.dll') -Destination $unsafe
Expect-Rejection 'Unapproved worker dependency unbundled.dll' { & $prepare -BundleDirectory $unsafe -CMakeCache $cache }
Write-Output "Windows runtime packaging regression passed: valid closure/version, archive roundtrip, missing/tampered DLL, no verify mutation, unapproved dependency. Evidence: $work"
