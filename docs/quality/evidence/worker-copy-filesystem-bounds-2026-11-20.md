# Worker-copy filesystem bounds: 2026-11-20

This increment continues `agent-conversation-workflows` task 13.5 and the existing
`native-agent-workspace-execution` implementation. Tasks 13.1, 13.3 and 13.4 remain
open. It does not qualify the entire worker architecture or installed platforms.

## Existing behavior and the observed gap

The existing `WorkingCopies` store and `PrepareWorker` path already reserve
storage, create independent sessions/cwd/process ownership/journals, preserve
dirty and untracked inputs, and pin required inputs and assignment versions.
The Linux exchange and Windows replacement backends already preserve the actual
displaced file, retain per-file receipts and refuse blind rollback/replay. This
increment extends those owners; it does not introduce another executor.

Inventory and growth measurement previously used `fs.WalkDir`, whose directory
listing happens before the visitor can reject an excessive entry count. File
reads used `Root.Open` after inspection, which can follow an in-root symbolic
link or block indefinitely if the regular file becomes a FIFO. Replacement sync
opened its pathname again without retaining the identity just observed.

The isolated Linux regression replaced a previously inspected regular file with
a FIFO with no writer. The previous implementation failed
`TestWorkingCopyOpenRejectsRacedFIFOWithoutWriter` (0.269 s): its open waited for
a writer. The test supplied a disposable writer only to release that blocked
reader before reporting failure. The original log is
`build/worker-copy-fifo-baseline-20261120.log`.

Independent review then identified an intermediate parent-substitution error in
the new bounded walker. A deterministic relative in-root symlink test reproduced
an accepted 4-byte measurement instead of the original 8 bytes (Linux failure,
0.012 s, `build/worker-copy-parent-swap-linux-relative-baseline-20261120.log`).
The first absolute-link attempt did not reproduce this defect because Root
already rejects absolute symlinks; Windows skipped that baseline because this
account lacks symbolic-link privilege. Those attempts are not counted as proof
of the intended boundary. The corrected relative-link regression passes after
the fix below.

## Implemented boundary

- `working_copy_walk.go` enumerates in positive batches of at most 256 entries.
  It charges entries as read against one traversal allowance, including pending
  parent siblings, before sorting the bounded list. Inventory retains separate
  limits for included entries and excluded Git roots, with their sum as the
  enumeration cap; measurement uses its original single entry cap. Excluded
  subtrees are skipped. `File.Readdir` captures file metadata from the opened
  directory rather than later resolving a pathname. A rooted directory identity
  check rejects a replaced parent before accepting completed measurement.
  Cancellation is checked before and after each batch,
  between entries and at completion. Manifest ordering is unchanged.
- `working_copy_open*.go` opens components relative to retained directory
  handles. Linux/macOS use `openat` with no-follow and nonblocking flags. Windows
  uses relative `NtCreateFile` with `FILE_OPEN_REPARSE_POINT`, checks each opened
  handle for reparse attributes, and requires intermediate directories. Opened
  file identity, type, size and modification time must match inspection.
  Windows enumeration requires handle-returned file-ID capabilities; unavailable
  capabilities fail explicitly instead of using Go's pathname-based ID fallback.
  Unsupported platforms fail explicitly. No new module dependency was added.
- Replacement and create/delete integration retain the file identity obtained
  during post-dispatch hashing through sync. A substituted path is an error;
  no failure path deletes displaced bytes or rolls back a newer edit.
- Oversized or unsupported growth measurements continue to mark the record
  uncertain and reserve the full host allowance. They cannot reopen capacity.

The implementation was checked against Go 1.26.6's local `os.Root` implementation:
supplying `O_NOFOLLOW` to `Root.OpenFile` alone does not establish this boundary,
because Root resolves in-root links itself. The
[versioned Go Root contract](https://pkg.go.dev/os@go1.26.6#Root) also explicitly
allows relative symbolic links inside the root. The Windows relative-handle and
reparse-open contract is described in Microsoft's
[NtCreateFile documentation](https://learn.microsoft.com/windows/win32/api/winternl/nf-winternl-ntcreatefile).
These sources explain the implementation choice; they do not replace tests.

## Validation checkpoint

| Check | Actual result |
| --- | --- |
| Pre-fix isolated Linux FIFO regression | Failed as described above, 0.269 s |
| Windows focused working-copy/replacement checks | Passed, 8.982 s, before the final empty-read/final-entry cancellation and directory-substitution regressions |
| Corrected relative-link parent-substitution baseline | Failed, 0.012 s; preserved original bytes but accepted a wrong measurement |
| Linux focused opener/walker checks after parent-metadata correction | Passed, 0.010 s, including actual FIFO and relative-link cases; later Windows capability guard/junction fixture not part of this checkpoint |
| Final Windows execution suite | Passed, 21.782 s; 144 top-level tests passed, 11 skipped, plus three subtest skips |
| Final Linux execution suite with race detector | Passed, 71.247 s; 153 top-level tests passed, four skipped; no race report |
| Actual Windows NTFS junction rejection | Passed, 0.020 s, through `FSCTL_SET_REPARSE_POINT` in a generated disposable root; no symlink privilege or shell required |
| Actual per-file process-crash recovery, in final Linux race suite | Passed, 6.250 s; no automatic replay |
| Independent review | Parent-substitution finding reproduced and fixed; corrected walker, Windows capability guard and replacement sync re-reviewed with no additional blocking finding; reviewer did not rerun tests |
| macOS runtime qualification | Not run; hardware unavailable |

Final commands, run sequentially with no parallel heavy verification:

```powershell
$env:GOMAXPROCS = '4'
go test -p 2 ./internal/execution -count=1 -json
wsl.exe -d Ubuntu --cd /mnt/d/offgrid-llm --exec env CGO_ENABLED=1 GOMAXPROCS=4 TMPDIR=/tmp /usr/local/go/bin/go test -p 2 -race ./internal/execution -count=1 -json
```

Full JSON logs are `build/worker-copy-execution-windows-final-20261120.jsonl`
and `build/worker-copy-execution-linux-race-final-20261120.jsonl`. Windows fixture
state was on C: NTFS; Linux fixture state used native `/tmp`. Windows skips cover
Linux-only/opt-in isolation checks, permission/link cases and an invocation-only
crash helper. Linux skips are the three opt-in Docker isolation checks and that
helper; actual integration crash subprocesses ran. Linux executed every new
relative-link/FIFO regression. Windows separately executed the new junction test.
The Windows relative-link parent test's pass wrapper contains skipped subtests
and is not counted as actual symbolic-link qualification.

One focused Linux launch incorrectly allowed the WSL shell to interpret the
test regex, failing before any Go test ran. The corrected `--exec` invocation
passed and its log is `build/worker-copy-rooted-linux-focused-exec-20261120.log`.
Strict OpenSpec validation, the maintained-doc link check (470 files), the
source-size ratchet and scoped `git diff --check` passed.

Regression coverage includes a synthetic ten-million-entry reader that produces
only the admitted budget plus one entry before rejection; exact real-directory
limits, parent/child shared accounting, cancellation, deterministic order and
Git exclusions; persisted conservative reservations after oversized measurement;
same-size/time inode replacement; leaf and ancestor links pointing to the same
original file; and a directory replaced during traversal. Existing replacement
and crash tests ran in the final package gates above.

## Remaining acceptance work

The broader independently admitted environment contract includes versioned
authorized environment templates and the remaining preparation/cleanup cases.
Background supervision and grant lifetime remain task 13.4. Task 13.3 still
requires its complete platform/backend qualification, including unsupported
backends and installed workflow evidence. This bounded change cannot close any
of those tasks.

No atomic multi-file snapshot, filesystem quota, sandbox, or universal immunity
to a hostile same-user process is claimed. File contents can change concurrently
between observations; hashes, retained identities and conservative receipts
detect the cases they observe. Cancellation cannot interrupt arbitrary kernel
filesystem stalls, but FIFO substitution no longer introduces a writer wait.
No live service, installation, user workspace, model download or native personal
session was used. All fixtures use generated disposable roots.
