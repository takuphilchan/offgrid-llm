param([Parameter(Mandatory = $true)][string]$InstallerPath)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'windows-installer-ui.ps1')
$stage = 'validate the isolated installer package'
$diagnosticPath = ''
$externalFixtureProcess = $null

trap {
    $message = "${stage}: $($_.Exception.Message)" -replace '[\r\n]+', ' '
    Write-WizardDiagnostic $diagnosticPath 'installer-test-failed' @{ stage = $stage; message = $message }
    if ($externalFixtureProcess -and -not $externalFixtureProcess.HasExited) { $externalFixtureProcess.Kill() }
    if ($env:GITHUB_ACTIONS -eq 'true') {
        Write-Output "::error title=Windows installer qualification::$message"
    } else {
        Write-Output "Windows installer qualification failed: $message"
    }
    break
}

# Only the deliberately isolated installer-test.cjs identity is accepted.
$installer = (Resolve-Path -LiteralPath $InstallerPath).Path
$testName = 'OffGrid Desktop Install Test'
if ([Diagnostics.FileVersionInfo]::GetVersionInfo($installer).ProductName -ne $testName) {
    throw 'Refusing to install a normal/release package. Build desktop/installer-test.cjs first.'
}
$registryPath = 'HKCU:\Software\37f1826c-126f-448e-8e4c-ce1b846307b9'
$uninstallKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\37f1826c-126f-448e-8e4c-ce1b846307b9'
if ((Test-Path -LiteralPath $registryPath) -or (Test-Path -LiteralPath $uninstallKey)) {
    throw 'An installer smoke-test registration already exists. Inspect it before retrying; nothing was changed.'
}
$testRoot = (New-Item -ItemType Directory -Path (Join-Path ([IO.Path]::GetTempPath()) ('offgrid-install-' + [Guid]::NewGuid().ToString('N')))).FullName
$diagnosticDirectory = (New-Item -ItemType Directory -Path (Join-Path $testRoot 'diagnostics')).FullName
$diagnosticPath = Join-Path $diagnosticDirectory 'test-events.jsonl'
$installRoot = [IO.Path]::GetFullPath((Join-Path $testRoot $testName))
$appExe = Join-Path $installRoot ($testName + '.exe')
$uninstaller = Join-Path $installRoot ('Uninstall ' + $testName + '.exe')

function Invoke-TestProcess([string]$Executable, [string]$Arguments, [int]$ExpectedExit = 0) {
    Write-WizardDiagnostic $diagnosticPath 'process-start' @{ stage = $stage; executable = $Executable }
    $process = Start-Process -FilePath $Executable -ArgumentList $Arguments -WindowStyle Hidden -PassThru
    if (-not $process.WaitForExit(180000)) { throw "Timed out waiting for test process $($process.Id); inspect it before cleanup." }
    Write-WizardDiagnostic $diagnosticPath 'process-end' @{ stage = $stage; process = $process.Id; exitCode = $process.ExitCode }
    if ($process.ExitCode -ne $ExpectedExit) { throw "Test process failed with exit code $($process.ExitCode), expected $ExpectedExit. Evidence: $testRoot" }
}

$stage = 'install the isolated package silently'
Write-Output "Installing isolated test package into $installRoot"
Invoke-TestProcess $installer "/S /currentuser /D=$installRoot"
if (-not (Test-Path -LiteralPath $appExe)) { throw 'Installed application is missing.' }
$registered = (Get-ItemProperty -LiteralPath $registryPath).InstallLocation
if ([IO.Path]::GetFullPath($registered).TrimEnd('\') -ne $installRoot) { throw 'Unexpected test install registration; refusing further actions.' }

# Real installed Electron and bundled Go service, with disposable profiles.
$stage = 'start the installed desktop and bundled service'
$smokeOutput = & node (Join-Path $PSScriptRoot 'test-desktop-startup.mjs') $appExe
if ($LASTEXITCODE -ne 0) { throw 'Installed-app startup checks failed; test installation retained for inspection.' }
$smoke = ($smokeOutput -join "`n") | ConvertFrom-Json
$sentinel = Join-Path $smoke.evidence 'isolated-profile/desktop-workspace/data/installer-preservation.txt'
[IO.File]::WriteAllText($sentinel, 'OffGrid installer preservation fixture')
$before = (Get-FileHash -LiteralPath $sentinel -Algorithm SHA256).Hash

# Keep an independently owned fixture alive across repair, Finish and uninstall.
# Only this process object is cleaned up; never identify services by port/name.
$externalRoot = (New-Item -ItemType Directory -Path (Join-Path $testRoot 'external-fixture')).FullName
$fixtureScript = Join-Path $PSScriptRoot 'desktop-external-fixture.mjs'
$externalFixtureProcess = Start-Process -FilePath (Get-Command node).Source -ArgumentList "`"$fixtureScript`" `"$externalRoot`"" -WindowStyle Hidden -PassThru
$fixtureReady = Join-Path $externalRoot 'ready.json'
for ($attempt = 0; $attempt -lt 100 -and -not (Test-Path -LiteralPath $fixtureReady); $attempt++) { Start-Sleep -Milliseconds 100 }
if (-not (Test-Path -LiteralPath $fixtureReady)) { throw 'External installer fixture did not start.' }
$externalPort = (Get-Content -LiteralPath $fixtureReady | ConvertFrom-Json).port
$externalURL = "http://127.0.0.1:$externalPort"
$externalBefore = Invoke-RestMethod -Uri "$externalURL/fixture-status" -TimeoutSec 2

# Exercise same-version repair/update; this does not qualify every older upgrade.
$stage = 'repair the installed package silently'
Invoke-TestProcess $installer "/S /currentuser /D=$installRoot"
if (-not (Test-Path -LiteralPath $appExe)) { throw 'Reinstall removed the application.' }
if ((Get-FileHash -LiteralPath $sentinel -Algorithm SHA256).Hash -ne $before) { throw 'Reinstall changed the workspace fixture.' }
if ((Invoke-RestMethod -Uri "$externalURL/fixture-status" -TimeoutSec 2).digest -ne $externalBefore.digest) { throw 'Repair changed the external workspace.' }

# Exercise the actual Finish checkbox and callback, not /S or a separately
# launched executable. A dedicated profile/port prevents touching the user's app.
$oldProfile = $env:OFFGRID_DESKTOP_HOME
$oldPort = $env:OFFGRID_PORT
$oldHidden = $env:OFFGRID_DESKTOP_TEST_HIDDEN
$oldRunAsNode = $env:ELECTRON_RUN_AS_NODE
$env:ELECTRON_RUN_AS_NODE = $null
$env:OFFGRID_DESKTOP_HOME = Join-Path $testRoot 'finish-workspace'
$env:OFFGRID_DESKTOP_TEST_HIDDEN = '1'
$env:OFFGRID_PORT = [string]$externalPort
try {
    $stage = 'complete the installer and launch the application'
    $finish = Invoke-InstallerWizard $installer $installRoot $true $false
    $ready = $false
    for ($attempt = 0; $attempt -lt 60; $attempt++) {
        try {
            $identity = Invoke-RestMethod -Uri "http://127.0.0.1:$env:OFFGRID_PORT/api/v2/system" -TimeoutSec 1
            $externalState = Invoke-RestMethod -Uri "$externalURL/fixture-status" -TimeoutSec 1
            if ($identity.product -eq 'offgrid' -and $externalState.uiRequests -gt $externalBefore.uiRequests) { $ready = $true; break }
        } catch {}
        Start-Sleep -Milliseconds 500
    }
    if (-not $ready) { throw 'Finish did not launch a working installed application.' }
    # Silent installation cannot terminate active work without user consent.
    $stage = 'refuse a silent reinstall while the application is running'
    Invoke-TestProcess $installer "/S /currentuser /D=$installRoot" 2
    $identity = Invoke-RestMethod -Uri "http://127.0.0.1:$env:OFFGRID_PORT/api/v2/system" -TimeoutSec 2
    if ($identity.product -ne 'offgrid') { throw 'Silent reinstall stopped active work.' }
    $activeTestProcesses = @(Get-Process -Name $testName -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $appExe })
    if ($activeTestProcesses.Count -eq 0) { throw 'Silent reinstall stopped the desktop even though the external service survived.' }
    $stage = 'repair interactively while the application is running'
    $reinstall = Invoke-InstallerWizard $installer $installRoot $false $true
    # Unchecked launch must not leave a desktop/backend running after reinstall.
    $testProcesses = @(Get-Process -Name $testName -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $appExe })
    if ($testProcesses.Count -ne 0) { throw 'Unchecked Finish launched the app or reinstall did not close it.' }
    if ((Invoke-RestMethod -Uri "$externalURL/fixture-status" -TimeoutSec 2).digest -ne $externalBefore.digest) { throw 'Interactive reinstall changed the external service.' }
    if ((Get-FileHash -LiteralPath $sentinel -Algorithm SHA256).Hash -ne $before) { throw 'Interactive reinstall changed saved data.' }
} finally {
    $env:OFFGRID_DESKTOP_HOME = $oldProfile
    $env:OFFGRID_PORT = $oldPort
    $env:OFFGRID_DESKTOP_TEST_HIDDEN = $oldHidden
    $env:ELECTRON_RUN_AS_NODE = $oldRunAsNode
}

# Resolve and validate the exact target before invoking an uninstaller that
# recursively removes application files. Never use a production registration.
$resolvedUninstaller = (Resolve-Path -LiteralPath $uninstaller).Path
if (-not $resolvedUninstaller.StartsWith($testRoot + '\', [StringComparison]::OrdinalIgnoreCase)) { throw 'Uninstaller escaped test directory.' }
if ([IO.Path]::GetFullPath((Get-ItemProperty -LiteralPath $registryPath).InstallLocation).TrimEnd('\') -ne $installRoot) { throw 'Test registration changed; refusing cleanup.' }
$stage = 'uninstall the isolated package and preserve its workspace'
Invoke-TestProcess $resolvedUninstaller "/S /currentuser _?=$installRoot"
if (Test-Path -LiteralPath $appExe) { throw 'Uninstall did not remove the test application.' }
if (Test-Path -LiteralPath $uninstallKey) { throw 'Uninstall left its test registration.' }
if ((Get-FileHash -LiteralPath $sentinel -Algorithm SHA256).Hash -ne $before) { throw 'Uninstall changed the workspace fixture.' }
if ((Invoke-RestMethod -Uri "$externalURL/fixture-status" -TimeoutSec 2).digest -ne $externalBefore.digest) { throw 'Uninstall changed the external service.' }
$externalFixtureProcess.Kill()
$externalFixtureProcess.WaitForExit()

[PSCustomObject]@{ passed = $true; installRoot = $installRoot; startupEvidence = $smoke.evidence; finish = $finish; reinstall = $reinstall; tested = 'clean install, installed startup, Finish with/without launch, silent running-app refusal, interactive running-app reinstall, uninstall, fixture preservation, compatible legacy external-service survival' } | ConvertTo-Json
