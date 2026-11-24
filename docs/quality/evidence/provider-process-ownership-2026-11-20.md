# Provider process ownership follow-up — 2026-11-20

Status: Windows process-tree prerequisite implemented in the existing
`ProcessProvider`. Owned-local Ollama/vLLM startup, listener verification,
configuration integration, other-platform containment and real runtime
qualification remain incomplete under task 8.4/8.5 of
[`inference-provider-runtimes`](../../../openspec/changes/inference-provider-runtimes/tasks.md).

## Implemented behavior

Windows provider workers now start suspended and join a distinct non-inheritable
kill-on-close Job Object before their initial thread resumes. The job does not
permit process breakaway. Retiring one provider terminates its owned descendants
without terminating another provider's job. The existing provider factory,
scheduler, package lease and uncertain-stop contract remain authoritative.

Shutdown captures bounded descendant process handles, terminates the job, then
requires zero active process accounting, signaled captured handles and a reaped
direct child before reporting success. Changed process totals during the
termination snapshot, inaccessible accounting, failed termination or timeout
retain uncertainty instead of releasing the package lease. Shutdown is bounded
and concurrent callers observe one result. Startup failure also uses a bounded
stop path. Forced service exit closes the private job handle and terminates its
remaining owned children.

Follow-up review also found that generation/health failure paths still killed
only the root PID. A descendant retaining stdout could leave cancellation waiting
for EOF indefinitely. Failed/aborted generations and unhealthy workers now await
the same whole-runtime retirement result; blocked-write Stop initiates retirement
without waiting on the control path. Closing owned protocol pipes unblocks reads
even when process termination is uncertain. Initial-thread resume also checks
the opened thread handle's owning PID against the retained child process.

The non-Windows native-worker path preserves its existing behavior. Direct-child
termination or POSIX process groups alone are not accepted as proof for new
local Ollama/vLLM admission. No local endpoint configuration gate was opened by
this increment. HTTP adapter construction also rechecks authorization and
cancellation before invoking any local launch lease.

## Actual verification

- `go test ./internal/inference -run ProcessProvider -count=1` passed (6.799 s).
- The initial new Windows descendant test failed: job active-process accounting
  reached zero while a captured child process handle still reported running.
  The implementation was corrected to wait for captured process handles as
  well and reject tree changes during the termination snapshot. The failing
  assertion was preserved.
- `go test ./internal/inference -run ProviderProcessTree -count=1` passed
  (1.602 s), then passed again (1.705 s) after bounding startup cleanup.
  These are real temporary test processes, including a parent, child and
  grandchild. Cases cover surviving descendants after parent exit, independent
  provider isolation, concurrent close, forced service-owner exit and uncertain
  ownership handles. Every helper process is scoped to the isolated fixture;
  no model runtime, user application or installed model was launched.
- `go test ./internal/inference -count=1` passed (31.591 s) after the process-tree
  correction. The subsequent small startup-cleanup and HTTP authorization
  changes received the focused checks above/below; the broader program
  integration record owns any later combined-suite results.
- `go test ./internal/inference -run HTTP -count=1` passed (3.476 s), including
  rejected authorization/cancellation before a local process lease is acquired.
- After the cancellation review correction,
  `go test ./internal/inference -run ProcessProvider -count=1` passed (10.021 s).
  New real Windows child-process fixtures intentionally retain their parent's
  stdout and ignore cancellation. Generation timeout, health timeout, blocked
  input writes and malformed output all verify descendant exit before the
  provider reports settled failure. Linux focused race checks for the same
  ProcessProvider test selection passed (3.691 s); Windows-only descendant
  fixtures are not represented by that Linux race result.
- Source-size, strict OpenSpec and whitespace checks passed during this
  increment. They do not establish platform/runtime qualification.

No Windows race result, real Ollama/vLLM process qualification, physical-memory
or GPU reclamation measurement, Linux/macOS descendant-containment result or
installed-product qualification is claimed here. Task 8.4 remains open until
owned local HTTP execution is integrated and its listener/resource proof holds.

## Primary references

Microsoft documents automatic child association and its exceptions in
[Job Objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects),
termination of nested jobs in
[TerminateJobObject](https://learn.microsoft.com/en-us/windows/win32/api/jobapi2/nf-jobapi2-terminatejobobject),
accounting fields in
[JOBOBJECT_BASIC_ACCOUNTING_INFORMATION](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_basic_accounting_information),
and suspended startup in
[process creation flags](https://learn.microsoft.com/en-us/windows/win32/procthread/process-creation-flags).
These mechanisms constrain trusted runtime subprocess creation; they are not a
sandbox for arbitrary remote code or out-of-process service launches.
