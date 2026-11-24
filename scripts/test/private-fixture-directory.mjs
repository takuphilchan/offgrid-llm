import { mkdtemp } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, isAbsolute } from 'node:path';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';

/** Only creates a new disposable directory. Never changes an existing ancestor's permissions. */
export async function privateFixtureDirectory() {
 if (process.platform !== 'win32') return mkdtemp(join(tmpdir(), 'offgrid-owner-qualification-'));
 // Shared Windows Temp ancestry may permit deletion by another account. Use the
 // account's local application data directory and protect only the new fixture.
 const script = `
$ErrorActionPreference = 'Stop'
$base = [Environment]::GetFolderPath('LocalApplicationData')
if (-not [IO.Path]::IsPathRooted($base)) { throw 'Private fixture parent unavailable' }
$target = Join-Path $base ('offgrid-owner-qualification-' + [Guid]::NewGuid().ToString('N'))
$created = New-Item -ItemType Directory -Path $target -ErrorAction Stop
if ($created.FullName -ne [IO.Path]::GetFullPath($target)) { throw 'Fixture identity mismatch' }
$identity = [Security.Principal.WindowsIdentity]::GetCurrent().User
$acl = New-Object Security.AccessControl.DirectorySecurity
$acl.SetOwner($identity)
$acl.SetAccessRuleProtection($true, $false)
foreach ($sid in @($identity.Value, 'S-1-5-18', 'S-1-5-32-544')) {
 $principal = New-Object Security.Principal.SecurityIdentifier($sid)
 $rule = New-Object Security.AccessControl.FileSystemAccessRule($principal, 'FullControl', 'ContainerInherit,ObjectInherit', 'None', 'Allow')
 $acl.AddAccessRule($rule)
}
Set-Acl -LiteralPath $created.FullName -AclObject $acl
[Console]::WriteLine($created.FullName)
`;
 const { stdout } = await promisify(execFile)('powershell.exe', ['-NoProfile', '-NonInteractive', '-EncodedCommand', Buffer.from(script, 'utf16le').toString('base64')], { windowsHide: true });
 const directory = stdout.trim();
 if (!isAbsolute(directory) || !/offgrid-owner-qualification-[a-f0-9]{32}$/.test(directory)) throw Error('Private fixture path invalid');
 return directory;
}
