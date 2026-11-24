param(
    [Parameter(Mandatory=$true)][string]$FixtureExecutable,
    [Parameter(Mandatory=$true)][string]$ServiceUserSID
)
$ErrorActionPreference = 'Stop'
$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = [Security.Principal.WindowsPrincipal]::new($identity)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw 'Run this explicit isolated-fixture provisioner as administrator; the service/tests must run as the ordinary target user.'
}
if ($ServiceUserSID -notmatch '^S-1-5-21-\d+-\d+-\d+-\d+$') { throw 'Provide the ordinary service user SID.' }
$serviceSID = [Security.Principal.SecurityIdentifier]::new($ServiceUserSID)
$administrators = [Security.Principal.SecurityIdentifier]::new('S-1-5-32-544')
$systemSID = [Security.Principal.SecurityIdentifier]::new('S-1-5-18')
$fixture = Get-Item -LiteralPath $FixtureExecutable
if ($fixture.PSIsContainer -or ($fixture.Attributes -band [IO.FileAttributes]::ReparsePoint) -or $fixture.Length -gt 64MB) { throw 'Fixture executable must be a bounded ordinary file.' }
$programRoot = [IO.Path]::GetFullPath([Environment]::GetFolderPath('ProgramFiles'))
$destination = [IO.Path]::GetFullPath((Join-Path $programRoot ('OffGridONNXFixture.'+[Guid]::NewGuid().ToString('N'))))
if (-not $destination.StartsWith($programRoot+[IO.Path]::DirectorySeparatorChar,[StringComparison]::OrdinalIgnoreCase) -or (Test-Path -LiteralPath $destination)) { throw 'Fixture target must be a fresh Program Files child.' }
$runtime = Join-Path $destination 'runtime'
New-Item -ItemType Directory -Path $runtime | Out-Null
Copy-Item -LiteralPath $fixture.FullName -Destination (Join-Path $runtime 'fixture.exe')
[IO.File]::WriteAllText((Join-Path $runtime 'worker.py'),'Isolated protocol fixture marker; this is not Python/ONNX execution.',[Text.UTF8Encoding]::new($false))
[IO.File]::WriteAllText((Join-Path $runtime 'fixture._pth'),"Lib`n",[Text.UTF8Encoding]::new($false))
$pins = @{}
foreach ($file in Get-ChildItem -LiteralPath $runtime -File) { $pins[$file.Name] = 'sha256:'+(Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant() }
$configuration = @{
    schema_version=1; runner_revision='worker-v1'; runtime_directory=$runtime
    python='fixture.exe'; script='worker.py'; runtime_files=$pins
    dependency_versions=@{onnx='1.0';onnxruntime='1.0';numpy='1.0';tokenizers='1.0'}
    state_directory=(Join-Path $destination 'unused-state-tests-must-override')
}
$configurationPath = Join-Path $destination 'runtime.json'
[IO.File]::WriteAllText($configurationPath,($configuration|ConvertTo-Json -Depth 6),[Text.UTF8Encoding]::new($false))
# All mutations below target only the fresh literal destination created above.
# Existing installations and source executable permissions are never changed.
$entries = @(Get-Item -LiteralPath $destination) + @(Get-ChildItem -LiteralPath $destination -Recurse -Force)
foreach ($entry in $entries) {
    if ($entry.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Unexpected link in isolated fixture.' }
    if ($entry.PSIsContainer) {
        $acl = [Security.AccessControl.DirectorySecurity]::new()
        $inherit = [Security.AccessControl.InheritanceFlags]'ContainerInherit,ObjectInherit'
    } else {
        $acl = [Security.AccessControl.FileSecurity]::new()
        $inherit = [Security.AccessControl.InheritanceFlags]::None
    }
    $acl.SetOwner($administrators)
    $acl.SetAccessRuleProtection($true,$false)
    foreach ($sid in @($administrators,$systemSID)) {
        $acl.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new($sid,'FullControl',$inherit,'None','Allow'))
    }
    $acl.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new($serviceSID,'ReadAndExecute',$inherit,'None','Allow'))
    Set-Acl -LiteralPath $entry.FullName -AclObject $acl
}
Write-Output "Created isolated protocol fixture: $configurationPath"
Write-Output 'Return to a non-elevated shell as the named service user. Set OFFGRID_ONNX_WINDOWS_FIXTURE_RUNTIME to this config path, then run the opt-in ONNX tests.'
