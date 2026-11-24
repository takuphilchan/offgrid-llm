# Encrypted workspace backup implementation — 2026-11-20

The owning change is
[encrypted-workspace-backups](../../../openspec/changes/encrypted-workspace-backups/).
This is ongoing source/fixture evidence, not completion of M1/M7, administrator
controls or installed-platform qualification. All operations below used disposable
test state. No personal workspace, live installation or model weights were used.

## Dependency and existing behavior

Pinned `filippo.io/age` v1.3.2 and its `filippo.io/hpke` v0.4.0 dependency;
`golang.org/x/crypto` moved from v0.36.0 to v0.55.0. Both new modules carry
BSD-style redistribution licenses, inspected in their downloaded source. The
repository retains `go 1.25.0` and `toolchain go1.26.6`; no toolchain upgrade was
required. `go mod verify` passed. Module/sum changes were inspected; extra sums
include the upstream dependency graph's newer x/net/x/tools versions.

Existing Windows storage tests passed after dependency selection (10.326 s).
After the shared-reader/encryption refactor, the existing suite passed again
(6.922 s), retaining plaintext ZIP, WAL, version, traversal and no-replace checks.

## Implemented storage behavior

- Legacy functions remain wrappers around explicit encrypted/plaintext options.
  Inner ZIP format 1 is unchanged; transport encryption metadata is returned
  separately and never inserted into the manifest.
- Encryption writes directly to private ciphertext staging and retains a temporary
  file key only in memory for post-write verification. Recipient labels are
  forwarded; no extra recovery recipient is created.
- Verification authenticates every decrypted byte before parsing/extracting ZIP.
  A test deliberately corrupts an encrypted chunk inside ZIP padding: ordinary
  random-access ZIP extraction succeeds, but OffGrid verification rejects it.
- ZIP/ZIP64 counts, offsets and central-directory metadata are preflighted before
  allocating the ZIP inventory. Existing manifest/data limits remain, with a
  101 GiB raw-archive and 64 MiB directory-index ceiling.
- Real Windows test files/directories are created with a protected current-user
  and SYSTEM DACL before writing; Unix creation uses 0600/0700. Publication uses
  no-replace Windows write-through rename or Unix hard-link/rename plus directory
  sync. Staged files are flushed again after SQLite/WAL validation.
- Cancellation/fault boundaries retain truthful pre/post-publication outcomes.
  Cleanup failures preserve the original error and identify retained temporary
  state. Source files are opened through a directory root to prevent path escape.

## Review findings and corrections

The first round-trip tests failed because age legally returns
`(n == len(buffer), io.EOF)` at the end of a random-access read. The wrapper now
normalizes a complete final read; the unchanged tests pass.

Independent agent review found a quadratic prefix index: a legal 65,535-byte ZIP
name could retain roughly a GiB in intermediate strings. A bounded component trie
now replaces it. Before insertion, paths are limited to 4,096 UTF-8 bytes and 128
components (including the archive `state/` prefix), with cumulative limits of
200,002 nodes and 64 MiB of retained component strings. Cloning component strings
prevents a short substring from retaining an entire long path. Regressions exercise
deep names in tiny archives and both cumulative budgets. Manifest JSON inventory
is decoded incrementally, bounded by the actual ZIP entry count before allocation;
unknown/duplicate fields and trailing values fail without decoding arbitrary trees.
These are allocation bounds, not a claim that maximum-size archives use 64 MiB RAM.

Two independent-review regressions initially failed: cleanup deleted unrelated data
recreated at an already-published stage's old pathname, and verification accepted a
substituted backup path before publication. Cleanup now tracks file identity and is
disarmed when rename transfers ownership. Verification/publication recheck the
original identity, and restore extraction retains an anchored directory root.
Tests preserve recreated sentinel data and reject substituted verified paths.
Source copying compares observed, opened and final regular-file identities as well
as size/mtime. Unix archive and anchored source opens are nonblocking so a swap to
a FIFO cannot hang before the descriptor check. Both substitutions have regressions.

`BackupWorkspaceThen` retains one existing owner through verified backup publication
and the follow-up mutation. Tests prove the owner remains held, backup failure or
cancellation prevents the callback, callback failure retains a verifiable backup,
and replacing the canonical workspace root prevents the callback. Independent
review of the CLI/RAG integration found and corrected canonical-path reuse and an
invalid Unix read count before slicing terminal input. CLI execution results are
recorded by its implementation owner.

These checks detect observed replacement and cooperate with the existing workspace
owner. They do not exclude every check-to-pathname-syscall race against a hostile
process running as the same OS user. Input archives and stopped workspaces must
remain under the operator's control. There is no secure-erasure or perfect Go
memory-zeroization claim.

## Executed storage checks

All commands below used Go 1.26.6 and isolated state. Windows storage runs include
actual protected-DACL inspection of disposable files, directories and inherited
children; Linux runs inspect actual 0600/0700 permissions.

| Check | Actual result |
| --- | --- |
| Windows `go test ./internal/storage -count=1`, latest complete source | PASS, 8.316 s |
| Ubuntu `CGO_ENABLED=1 go test -race ./internal/storage -count=1`, after source-FIFO fix and reordered-chunk/final-write regressions | PASS, 6.980 s |
| Earlier Windows cancellation/publication regressions | PASS, 9.391 s |
| Windows subprocess crash-focused tests | PASS, 4.753 s |
| Windows independent age CLI X25519/Hybrid interoperation | PASS, 7.565 s package time |
| Ubuntu independent age CLI X25519/Hybrid and production-cost passphrase interoperation | PASS, 7.053 s package time |

The final Linux race checkpoint predates only an additional invalid-options test;
the latest full Windows run includes it. Tests cover native/multiple recipients,
incompatible labels, wrong/missing keys, bounded KDF input, header/middle/final and
unvisited-chunk corruption, explicit chunk reordering, truncation, appended and
concatenated ciphertext, authenticated invalid ZIP, strict inventory/CRC/SHA,
legacy archives, encrypted committed-WAL recovery, SQLite/foreign-key/version
rejection and no-replace races. A real ZIP64 central directory with 65,536 entries
is included in the metadata preflight tests.

Injected lifecycle failures and cancellation cover pre/post-publication boundaries,
cleanup failure preserves the original cause and retained path, and failing writers
exercise encrypted write/final-tag-close errors. These do not simulate a physical
disk power loss. Subprocesses exit at backup close/verify/publication and restore
stage/file-sync/publication boundaries; parent processes verify ciphertext remnants,
source preservation, truthful published state and explicit new-target recovery.
The crash fixtures never start a recovered service or replay tasks.

Repeated passphrase mechanics use reduced-cost fixture KDFs. The separate reference
CLI passphrase test uses production scrypt logN18 and an actual Unix PTY, checks
no echo, and successfully imports/verifies/restores then exports/decrypts the
archive. Its explicit reference CLI plaintext export is a disposable interoperation
fixture; OffGrid's encrypted operations do not create such a spool.

## Independent reference implementation

The reference executable is built from pinned `filippo.io/age@v1.3.2/cmd/age`
source, outside this module, with `GOTOOLCHAIN=local` and `-mod=readonly`.
`age --version` reports `(devel)` because it is a source build; these are not claimed
to be release binaries. Actual SHA-256:

- Windows: `f4d6ca8eaedb9650781f9634a5c79ee9ed00f06dc5ba50eec9c258690e68a06d`.
- Linux: `b0157ba51d3e909d5c478eb943cf43efb6f60918c9820af07de70d1832eca8fd`.

Prepare the pinned module in an existing Go cache (dependency network access may be
needed), then build from its directory rather than changing OffGrid's module:

```sh
go mod download filippo.io/age@v1.3.2
cd "$(go env GOMODCACHE)/filippo.io/age@v1.3.2"
GOTOOLCHAIN=local go build -mod=readonly -o /absolute/isolated/tools/age ./cmd/age
```

From the OffGrid checkout, the actual Linux harness invocation was:

```sh
OFFGRID_TEST_AGE_CLI=/mnt/d/offgrid-llm/build/qualification-tools/age \
  go test ./internal/storage -run TestEncryptedBackupAge -count=1 -v
```

The Windows harness sets the same variable to the absolute `age.exe` path and runs
the same Go command; its Unix-only PTY test is intentionally absent. Native-key
fixtures each import/export four files, 49,280 bytes, including committed WAL data
whose restored query returns `acknowledged before crash`. Exports are independently
decrypted by the CLI, then every manifest entry is CRC/SHA/size checked. Private
keys and passphrases are neither arguments nor environment values. Unix reference
PTY passphrase data is passed over stdin to the isolated harness.

## Fixture resource measurements

`TestBackupFixtureQualification` is opt-in and generates 64 separate 1 MiB,
deterministic incompressible files. It measures native X25519 backup, complete
verification, restore and asynchronous cancellation during real verification IO.
It samples Go heap at 5 ms; `/usr/bin/time -v` separately measures process RSS,
including fixture setup. It is neither a maximum-workspace guarantee nor a KDF
benchmark. The observed archive was 67,157,516 bytes for 67,108,864 source bytes.

Host: Intel Core 7 240H, 16 visible logical CPUs, WSL2 Linux x86-64 kernel
6.18.58.1, Go 1.26.6. The first run used `/tmp`, which `findmnt` identifies as tmpfs:
backup 2.637420 s, verify 543.461 ms, restore 596.612 ms, cancellation acknowledgment
84.332 microseconds, sampled peak Go heap 8,502,336 bytes (baseline 400,072), process
peak RSS 15,720 KiB. These RAM-backed timings do not qualify filesystem durability
or installed performance.

A separate run used a fresh private `/var/tmp/offgrid-backup-qual-*` directory,
confirmed by `findmnt` as ext4 on `/dev/sdd`: backup 1.744601 s, verify 566.599 ms,
restore 918.311 ms, cancellation acknowledgment 184.398 microseconds, sampled peak
Go heap 20,171,616 bytes (baseline 399,880), process peak RSS 16,140 KiB; PASS in
3.34 s. The heap and process counters are different measurements and are retained
as reported. WSL storage is still not a physical power-loss or installed-edition
qualification. An earlier attempt lost `TMPDIR` through cross-shell argument
handling, emitted `findmnt`/cleanup errors and actually used tmpfs; that failed
setup is retained separately and excluded from the ext4 result. Running a Bash
script directly fixed the harness quoting. Windows PowerShell rendered the time
tool's stderr as a native-command diagnostic; the final script and measured
process both returned zero, with the filesystem check and PASS visible.

To repeat against an explicitly selected scratch filesystem without touching a
workspace, create an empty private scratch directory and run:

```sh
go test -c -o /absolute/isolated/tools/storage-qualification.test ./internal/storage
findmnt -T /absolute/isolated/scratch
/usr/bin/time -v env TMPDIR=/absolute/isolated/scratch \
  OFFGRID_TEST_BACKUP_QUALIFICATION=1 \
  /absolute/isolated/tools/storage-qualification.test \
  -test.run '^TestBackupFixtureQualification$' -test.count=1 -test.v
```

## Remaining acceptance work

Source-size ratchet, strict OpenSpec validation and `git diff --check` passed;
the documentation checker inspected 446 maintained Markdown files with zero
problems. These checks validate structure, not the runtime behaviors above.

The independently inspected stable aggregate logs now record Windows full-package
storage/CLI/RAG passes (6.704/5.969/16.765 s) and Ubuntu/WSL full-package race passes
(6.401/82.458/80.867 s), with normal vet and assertions retained. Commands and
actual root-dispatcher subprocess/Windows ConPTY/Unix PTY evidence are in the
[CLI evidence](encrypted-backup-cli-2026-11-20.md). That aggregate checkpoint precedes
one final bounded-input correction: CLI key-file opening now uses nonblocking Unix
open before checking the originally observed regular-file identity. An isolated
regular-file-to-FIFO substitution regression passed with the race detector
(1.350 s); Windows focused key/secret/encrypted tests passed (2.740 s). The small
opener change leaves the existing size limit, identity check and safe errors intact.
Independent review and these combined checks satisfy the storage/CLI acceptance
gate; they are not completion of the following integration/qualification work.

Trusted administrator maintenance controls and actual renderer-to-host recovery
remain unimplemented at this checkpoint. Whole-project
checks for the completed feature must be rerun after that integration. Windows
source-level DACL and Linux source tests are not installed-edition qualification;
macOS behavior, real installed Windows/Linux journeys, physical storage crash
behavior and the M7 participant/usability protocol remain open.

For representative-workspace qualification, obtain a separately authorized isolated
copy, retain its inventory, SQLite committed-record counts and matching binary
version, and use the commands in the [recovery guide](../../advanced/workspace-recovery.md).
Record actual filesystem, archive size, elapsed/RSS measurements, cancellation and
retained-stage outcomes, then compare the new restored directory's hashes and
database records. Never use the original live root, overwrite an existing restore,
resume uncertain tasks or replace a missing real result with this fixture's result.
Keep the matching requirement open until those resources and evidence exist.
