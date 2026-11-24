# Worker-copy cleanup ownership: 2026-11-20

This record covers the approved task 13.6 increment under
`agent-conversation-workflows`, following
[the bounded filesystem prerequisite](worker-copy-filesystem-bounds-2026-11-20.md).
The larger environment and supervision tasks 13.1 and 13.4 remain open.

## Audit and reproduced defect

Private worker sessions, input manifests, independent process ownership and
parent/job revocation already exist. Versioned authorized environment-template
reuse and explicit service-owned background supervision are not implemented by
this change. The current process environment uses a fixed allowlist, and enrolled
hosts depend on their existing live connection/heartbeat and client lifetime.
No existing client-owned grant is extended or restored here.

`WorkingCopies.Release` previously saved `releasing`, recursively removed the
generated pathname, then saved `released`. A final SQLite write failure left
the reservation unresolved. Recreating a different directory at the old pathname
and explicitly retrying Release deleted its unrelated sentinel and returned
success. The actual Windows regression failed in 0.335 s; the retained log is
`build/worker-copy-cleanup-recreated-baseline-20261120.log`.

## Implemented checkpoint

- Worker-copy schema 12 adds a host-private ownership table. New reservations
  record their ownership version atomically; native device/volume, inode/file
  identity and birth/creation time are persisted before the content directory
  or copied bytes. Publication must affect exactly one expected row. No private
  identity is added to API/worker receipt fields, manifests or credentials.
- Ownership and cleanup checkpoint writes also compare exact transactional
  readback before commit. All ownership fields, source record/digest and retained
  reservation must match the intended transition; ignored or trigger-altered
  writes roll back, including uncertain-state bookkeeping after a failed cleanup.
- A worker-use lease is committed before session admission. Only the retained
  closed supervisor can settle it, after endpoint/session/journal closure and
  native descendant proof. A failed settlement write can be retried by that
  same closed owner; restart cannot manufacture the proof.
- Cleanup commits an opaque quarantine name before a no-replace directory move,
  verifies the moved native identity and retains an `os.Root` for it. A raced
  unrelated object remains retained at quarantine; no rollback overwrites a
  newer original pathname. Inventory precedes deletion within the content and
  two artifact measurement allowances plus fixed metadata-directory headroom.
  Entries are removed in postorder with cancellation and identity checks.
  Windows disposal uses the exact opened handle with write sharing excluded;
  Unix pathname removal is restricted to this detached private namespace after
  proven worker settlement. No outer recursive deletion remains.
- A durable empty-directory checkpoint precedes final nonrecursive removal and
  reservation settlement. Recreated paths fail identity validation. A repeated
  already-published release does not inspect or modify a later directory at that
  name. Incomplete deletion/publication retains uncertainty and capacity.
- Schema-10 records are retained without inventing ownership from current paths.
  Their manifests remain inspectable. Cleanup without ownership proof fails
  explicitly. Version-11 identity-only rows acquire an unresolved lease during
  migration, and version 12 prevents older cleanup code reopening the store.
  Restart only repairs bookkeeping; it does not delete artifacts or restore
  sessions/processes/grants.
- The existing Windows Job owner retains the launched root and captured member
  handles before termination, waits on each exact handle, and requires the
  lifetime process count to match the retained set (maximum 4,096). Missing
  fast-exited members remain uncertain. Failed launch construction and bounded
  history eviction cannot erase cleanup uncertainty. Current native Linux
  process groups do not prove escaped descendant settlement, so process-used
  private copies retain their lease; this is safe incomplete behavior.
- Linux directory transitions are synced before detachment, empty-directory
  and released receipts. The existing Windows namespace persistence primitive
  does not prove physical power-loss durability; that qualification remains open.

## Verification checkpoint

| Check | Actual result |
| --- | --- |
| Lost-receipt/recreated-path baseline | Failed, 0.335 s; unrelated sentinel deleted by the old implementation |
| Same regression after ownership fix | Passed, 0.362 s package time |
| Focused Windows cleanup/publication/restart/cancellation/legacy tests and previous SQLite fault cases | Passed, 0.823 s |
| Windows Job accounting-only baseline, five repetitions | All failed, 1.027 s package time: retained living child handle returned `WAIT_TIMEOUT` after release |
| Same five actual Job cases with retained-handle settlement | Passed, 0.944 s |
| Expanded Windows quarantine/lease/Job boundary suite | Passed, 1.964 s |
| Earlier full Windows execution checkpoint, before transactional postcondition correction | Passed, 21.565 s; 165 top-level passes, 11 top-level skips and three additional subtest skips |
| Earlier full Linux execution race checkpoint, before transactional postcondition correction | Passed, 71.931 s; 171 top-level passes, four skips; no race report |
| Actual altered-lease baseline | Failed, 0.551 s: an AFTER UPDATE trigger restored `in_use=0` while worker admission succeeded |
| Same altered-lease case after exact transactional readback | Passed, 0.507 s |
| Expanded altered checkpoint/cleanup Windows subset | Passed, 1.901 s; subsequent standalone altered-settlement case awaits final gate |
| Final full Windows/Linux race gates after postcondition correction | Pending serialized test window |
| macOS runtime and additional filesystem qualification | Not run |

The first focused command after test additions could not compile because another
in-progress storage increment temporarily referenced an undefined helper. No
test ran. After that helper was corrected by its owner, the focused retry passed;
both logs remain under `build/worker-copy-cleanup-windows-focused*20261120.log`.

The focused scenarios use real generated directories and SQLite faults: identity
survives rename and store restart; a replacement directory and original retained
inputs both survive refusal; partial cancellation leaves later files and capacity;
over-budget inventory deletes nothing; ignored ownership publication cannot copy
input; legacy records are not adopted; and publication-success retries leave a
recreated directory untouched. None is evidence of model or installed-workflow
quality.

The expanded suite exercises nine actual interruption points: intent saved,
before/after detachment, detachment receipt, before/after an entry removal,
empty-directory receipt, and before/after final directory removal. Each keeps
capacity on failure and requires an explicit owned retry after restart. Separate
tests substitute another directory just before detachment, recreate the original
path after detachment, force settlement SQL failure, retain a live-worker lease,
and migrate version-11 ownership without inventing settlement. Native Windows
tests rename a file after opening its deletion handle and verify replacement
preservation; active/new writer sharing is rejected.

The full Windows log is
`build/worker-copy-cleanup-windows-full-20261120.jsonl`. Its existing skips cover
unavailable unprivileged symlink/permission fixtures, Linux-only isolation and
three explicitly opt-in container checks, plus an invocation-only crash helper.
The native Windows disposition, writer-exclusion and Job descendant tests ran;
none was skipped. Build/test success is not installed workflow qualification.

The Linux log is `build/worker-copy-cleanup-linux-race-full-20261120.jsonl`.
Its four skips are the three opt-in Docker fixtures and the invocation-only
crash helper. The actual `setsid` descendant fixture ran in 1.08 s: the leader
exited while its descendant remained alive; explicit cleanup refused, original
contents and reservation survived, and fixture teardown killed only the retained
pidfd-owned child. This proves conservative retention, not complete Linux native
descendant supervision. Existing per-file crash/recovery and Linux link/FIFO
tests also executed in the full race gate.

OpenSpec strict validation, the source-size ratchet, the seven documentation
checker tests and the 478-file maintained-document check passed. No module
dependency, public API or grant lifetime changed. Broader service/agent consumer
checks are coordinated separately; these package gates are not full-program
completion evidence.

## Review correction and remaining qualification

Independent review found a check/unlink interval in the first implementation.
The settled-owner lease, private quarantine and native Windows disposition above
address that interval within the enforced supervisor/private-state boundary.
Normal app writers cannot reuse a live copy through the existing storage owner;
captured stale worker paths cannot become cleanup authority. A hostile same-OS
user replacing all supervisor state is outside this boundary. Unix unlink is
not an inode-conditional primitive and is not presented as one.

An actual Windows Job test then found that `ActiveProcesses == 0` did not imply
the retained child's process handle was signaled. The original five failures
remain in `build/worker-copy-job-exit-baseline-20261120.log`; exact-handle waits
pass in `build/worker-copy-job-exit-fixed-20261120.log`. The new lifetime-count
check intentionally retains uncertainty when a fast descendant exited before
the owner could capture its handle. This remains incomplete general worker
supervision, rather than permission to erase unknown process history.

Task 13.6 stays unchecked: complete native Linux escaped-descendant ownership,
Windows unobserved-member recovery, macOS/runtime and filesystem qualification,
and Windows physical power-loss persistence still require work. Environment
template reuse and authorized service-owned background work also remain open.

A second independent review identified an altered-write hole in checking only
SQLite affected-row counts. The actual lease regression and corrected run remain
in `build/worker-copy-altered-lease-{baseline,fixed}-20261120.log`. Added trigger
cases cover identity before copied bytes, lease admission/settlement, quarantine
intent, detachment, empty-directory publication, final release and uncertainty
bookkeeping. The preceding full-package results qualify their earlier source
checkpoint; they are not substituted for the final corrected-source gates.

The operating-system distinction is concrete: Linux
[unlinkat](https://man7.org/linux/man-pages/man2/unlink.2.html) removes a pathname
relative to a directory descriptor; it does not accept the expected target inode.
Windows supports
[file disposition through an opened handle](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-setfileinformationbyhandle).
These primary contracts inform the remaining design decision and are not a
substitute for qualification.

The Windows implementation additionally uses the documented
[Job member list](https://learn.microsoft.com/windows/win32/api/winnt/ns-winnt-jobobject_basic_process_id_list),
[lifetime accounting](https://learn.microsoft.com/windows/win32/api/winnt/ns-winnt-jobobject_basic_accounting_information)
and per-process wait contracts. Neither an empty membership list nor a successful
termination request substitutes for an observed process exit in these tests.

No live workspace, installed service, model weights, personal browser session or
user files were used. All filesystem mutations were inside generated fixtures.

## Continuation checkpoint — 2026-11-21

The subsequent source separates exact leader exit from versioned descendant
termination evidence. Missing, invalid or unknown evidence keeps completion,
measured budget refunds, copy release and Stop unsettled. The narrow wire,
leader-observation, budget and completion tests passed: execution 0.306 s,
agents 0.595 s and server 0.489 s. Log:
`build/process-descendant-proof-focused-20261121.log`.

The first full execution run failed in 22.134 s on two older synthetic positive
receipt fixtures, which expected whole-command success without descendant proof.
Those fixtures now explicitly supply the complete evidence they intend to test;
the independent missing/unknown/future-proof negative cases remain strict.
The full execution package then passed in 19.462 s. Logs:
`build/execution-proof-full-handoff-20261121.log` and
`build/execution-proof-full-final-20261121.log`.

The actual native Go-test consumer previously failed because short-lived compiler
descendants exited before handle capture. That composed shutdown gate remains
unresolved. Windows completion-port lifetime capture was investigated but is not
implemented in this checkpoint. No empty Job membership/count or leader exit was
substituted for the missing proof. Linux race and complete consumers must be rerun
after the supervision change. Task 13.6 remains incomplete.

The subsequent [Windows watcher increment](worker-job-watch-2026-11-21.md) now
implements early membership-validated handle capture. That specific native
compiler/shutdown regression passed three times with the final source, while the
deliberately missed-notification test still rejects cleanup. The preceding
paragraph records the earlier checkpoint; remaining Linux/platform/qualification
requirements and task 13.6 are still incomplete.
