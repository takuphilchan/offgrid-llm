# Workspace ownership and offline recovery

Status: tested maintenance primitives; not a completed migration/update system.
The transactional workspace migration and administrator UI controls are still
pending. These commands do not upgrade schemas or change your active installation.

## Single writer

Current builds acquire `.offgrid-owner.lock` in `OFFGRID_DATA_DIR` before opening
stores or preparing the data layout. A second service or maintenance operation
fails with an actionable error. The operating system releases ownership after
process exit, including crashes; an existing lock file does not mean a stale lock.
Do not delete the lock file to force access.

Use local storage. UNC shares are rejected; mapped network drives are not reliably
detectable and are unsupported. Older binaries and other programs do not honor
this new lock. Stop them explicitly before maintenance. Do not run direct-file
legacy CLI operations concurrently; their service-only conversion remains pending.

During shutdown the service cancels execution, drains HTTP requests and agent
workers, persists interrupted/uncertain checkpoints, closes knowledge storage,
then releases ownership. If a worker cannot drain, ownership remains held until
process exit. An uncertain tool outcome is never automatically retried.

## Back up a stopped workspace

Use the actual data directory from your configuration, not the models directory.
Stop the service using its existing supervisor/desktop/container controls first.
Maintenance commands never stop an externally managed service for you.

```sh
offgrid workspace backup --data-dir /path/to/data --output /path/to/backups/workspace.zip
offgrid workspace verify /path/to/backups/workspace.zip
```

The destination directory must already exist and be outside the workspace. A
backup never overwrites an existing archive. Native Windows accepts normal quoted
Windows paths. Add `--json` for one machine-readable result; errors use exit status
1, invalid usage 2, and cancellation 130.

The archive contains **all regular files in the data directory**, including
conversations, agent snapshots, events, knowledge databases and WAL files,
artifacts, identities, and credentials stored there. It excludes the ownership
lock. Preserve models, runtime binaries, configuration, integrations, MCP servers,
and external agent homes **outside that directory separately**. This is not yet a
portable offline installation pack or an external-agent environment backup.

Archives are unencrypted private material. Restrict access and use encrypted
storage for copies. SHA-256 hashes detect damaged files; they do not authenticate
an archive supplied by an attacker. Use only your own trusted backups.

Backup refuses symlinks, special files, nonportable paths, and case/normalization
collisions rather than silently skipping files. Limits: 100,000 files and 100 GiB
uncompressed data. Failed/incomplete backups are not published as usable archives.

## Verify and restore without replacing newer data

Verification checks the format, complete inventory, byte counts, ZIP checksums,
and SHA-256 digests. It does not prove task correctness or database usability.
Restore additionally opens database copies and checks SQLite integrity and foreign
keys before activating the staged directory.

```sh
offgrid workspace restore /path/to/backups/workspace.zip --data-dir /path/to/restored-data --yes
```

The restore target must **not exist**, and its parent must exist. The command
requires the backup's application version; `dev` builds cannot restore. It does
not fall back to overwriting or merging a workspace. File paths are validated
before extraction, permissions are restricted, and activation refuses races with
another creator of the target directory. The original data stays untouched.

After checking the restored directory, start the matching application with
`OFFGRID_DATA_DIR` pointing to it and restore the separately retained model/runtime
and external configuration paths as needed. Do not start an older application
against a newer schema. Keep the original directory until you have verified the
restored conversations, knowledge, identities, and agent recovery states.

This is deliberately a manual recovery flow. Revision-qualified matched snapshots,
automatic pre-update backup, migration, approved update/rollback, signatures, and
administrator UI controls still require implementation and qualification.
