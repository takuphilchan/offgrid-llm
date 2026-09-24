# Governed agent tasks

OffGrid's built-in task runner uses a local chat model and enabled tools. The
model must support structured tool calls; an installed model or successful
health check alone does not prove it can complete a useful task.

Hermes and OpenClaw are separate runtimes using OffGrid inference. Their own
tool execution is **not** governed by this runner's approval broker. See
[external agents](external-agents.md) for provider setup.

## Start and inspect work

On matching task-first builds, open **Agents**, describe the outcome and choose
**Start task**. The task is saved before execution. Model selection is under
**Task settings**; application and browser setup are not prerequisites for
submitting a task. A model with reliable structured tool calling is still required.

When the model requests computer access, the saved task displays **Access needed**.
Review the requested application/site and action-approval policy in the desktop
app. Locally approve the actual target; model-proposed names never grant access.
The same task then continues, beginning with a fresh observation. Web users can
choose **Continue in desktop** to open that saved task in the matching workspace.
The deep link carries only its ID, no credentials or executable actions. The
desktop app and service must use compatible builds; a container update alone
cannot update the host companion. Manual pairing is not part of this flow.

Navigation or reload does not start another task. An unacknowledged submission
retains its request ID for a safe retry. A cancelled or uncertain action is not
automatically retried. Access requests survive restart; connected computer
authority does not. If a model check fails, the saved task remains pending
access; repair the model/runtime or cancel and reuse the task with another model.

Older compatible clients retain the explicit `/v1/agents/run` workflow below.
The CLI submits, inspects and controls task-first jobs using the same job APIs;
`waiting_for_input` directs users to the desktop consent UI, not a bypass flag.

### Workflows and multiple agents

A task can keep a work plan, archive/retrieve older exact context, and delegate
bounded read-only analysis to linked subtasks. Describe the desired outcome;
there is no topology setup form. Delegation uses the same local model and runner,
up to four children per group and eight per task, without copied computer access
or approvals. Dependencies run after their prerequisites. Inspect each child
for findings, approvals or recovery; the parent waits for all children and cannot
silently ignore a failed branch. After restart, explicitly resume the parent.

For work across applications, OffGrid can verify the current target and request
the next application's local consent in the **same task**. It releases the old
session; it does not silently grant whole-desktop authority.

Legacy arbitrary workflow registration remains unavailable (`501
durable_coordination_unavailable`). Do not confuse bounded read-only delegation
with general recursive multi-agent or concurrent computer control. See the
[architecture](../advanced/ARCHITECTURE.md#bounded-durable-coordination) and
[reliability plan](../advanced/product-reliability-plan.md) for qualification limits.

### CLI tasks

Start the service with `offgrid serve`, then choose an installed model from
`offgrid list`. With authentication enabled, set `OFFGRID_API_KEY` in the client
environment; do not put keys in screenshots or issue reports.

```bash
offgrid agent chat --model YOUR_INSTALLED_MODEL_ID
offgrid agent run "Calculate 25 * 47 and report the result" --model YOUR_INSTALLED_MODEL_ID --wait
offgrid agent tasks
offgrid agent status RUN_ID
```

The web UI uses **Agents → Work**. It restores the selected run after navigation
or reload, including pending approvals and committed tool results. Task drafts
save while editing, separately per signed-in account. Browser drafts are not
encrypted or synchronized between devices; a warning means storage failed and
the page must remain open to retain the in-memory text.

Run styles influence the instructions, not permissions:
`react` uses incremental tool/result steps, `cot` requests careful analysis with
a concise justification, and `plan-execute` asks for a short plan and verified
steps. CLI `--style plan` selects `plan-execute`. No private reasoning trace is
promised. Use `--max-steps` (1–50) to bound model iterations. This runner does not
implement the old `--template` examples; use clear task instructions or the API's
legacy `/v1/agents/run` API's `system_prompt`.

### Pause, adjust and recover

The task toolbar provides **Pause**, **Take over**, and **Stop** when applicable.
Pause stops new scheduling; an already dispatched effect may be uncertain, never
silently rolled back. Take over also revokes that task's computer session. After
manual work, **Reconnect access** obtains fresh local consent. Stop is not undo.
The companion's local emergency stop remains available without the service.

While a task is safely paused, **Update instruction** saves a follow-up without
starting a second job. Resume explicitly when ready. Reusing the same instruction
request ID retries the save without appending it twice. Active delegation must be
resolved rather than silently discarded by a changed instruction.
Completed subtasks remain inspectable and do not prevent follow-up instructions.

Stopping from a task's access panel cancels that saved task and its own pending
computer setup; it does not stop another task's connected application. The local
emergency stop is separate. This requires the matching updated desktop host,
including its scoped-stop IPC; an older host is directed to update before setup.

```bash
offgrid computer run "Read the selected document and summarize it" --model YOUR_MODEL
offgrid computer setup RUN_ID
offgrid agent pause RUN_ID
offgrid agent steer RUN_ID UNIQUE_REQUEST_ID "Focus on the three main findings."
offgrid agent resume RUN_ID
offgrid agent takeover RUN_ID
offgrid agent reconnect RUN_ID
offgrid agent export RUN_ID
```

`computer pause|resume|takeover|stop RUN_ID` are aliases for the same lifecycle.
`computer setup RUN_ID` opens local consent in the installed desktop app; it does
not bypass it. `agent run --request-id ID` safely retries a lost submission.
Without `--wait`, submission returns the saved run immediately. JSON submission
returns a snapshot; inspect that ID for completion rather than resubmitting.

### Verified workspace artifacts

Ask for a downloadable text, Markdown, JSON or CSV artifact. The bounded
`task_save_artifact` tool saves up to 128 KiB, independently rereads the bytes,
checks their SHA256 and parses JSON/CSV before reporting saved output. CSV is
limited to 5,000 rows with consistent columns and no formula-like cells. Use JSON
for values rejected by that safeguard. This does not save files inside a host
application or verify factual claims. Application outcomes still need their
own explicit checks; a plan marked done is not evidence.

Artifact links appear in the task. **Context usage → Export evidence** downloads
the owner-scoped activity, plan, child links and verification records, not private
model checkpoints or screenshots. The CLI equivalent is:

```bash
offgrid agent export RUN_ID > evidence.json
offgrid agent artifact RUN_ID SHA256 --output new-report.csv
```

Downloads recheck task ownership and digest integrity. CLI download never
overwrites an existing file. Deleting a task removes its download authority;
referenced child history must be retained until the parent is removed.

## Review one exact action

A risky tool pauses the existing run. Review its tool name, full canonical
arguments, and expiry in the UI or CLI. Approval lasts 15 minutes and applies
to one invocation, even if another invocation has identical arguments.

```bash
offgrid agent approve RUN_ID APPROVAL_ID
offgrid agent deny RUN_ID APPROVAL_ID
offgrid agent cancel RUN_ID
```

Approving resumes that checkpoint. It does not rerun earlier successful tools.
Deny is saved on the server; closing the page is not denial or cancellation.
A duplicate or stale approval is rejected. If approval expired, inspect the
run and use `offgrid agent resume RUN_ID` to issue a fresh approval request.

## Restart and uncertain outcomes

The service writes each pending call before executing it, then writes its
result before proceeding. It never resumes side effects automatically at startup.

- `interrupted`: execution stopped at a safe checkpoint. Inspect, then
  `offgrid agent resume RUN_ID`.
- `uncertain`: a tool may have changed an external target, but its result was
  not durably confirmed. Inspect files/services affected by that exact call.
  Cancellation cannot undo side effects.
- `waiting_for_approval`: review and approve/deny the existing call.
- `failed` or `cancelled`: terminal; not silently retried.

For an uncertain call, record only an outcome you actually verified:

```bash
offgrid agent reconcile RUN_ID CALL_ID "Verified the requested file exists with the expected content"
offgrid agent resume RUN_ID
```

Reconciliation appends this verified result to the conversation; it does not
execute the uncertain tool again. If you cannot determine the outcome, leave the
run uncertain. Do not invent success or retry a destructive operation blindly.

An ordinary failure of a built-in file read, directory listing, calculation or
clock read is a failed task, not an unknown side effect. It does not require an
outcome form. A read interrupted by service restart can be explicitly resumed;
it is never automatically rerun on startup. Unknown failures of mutating or
external tools remain conservative, including legacy records without reliable
effect metadata.

Built-in filesystem tools access the service's filesystem, not automatically
your desktop's files. A Windows drive request sent to a Linux container now
pauses for local application access instead of attempting that path inside the
container. Granting access is not evidence that files were listed or changed:
the task must observe the selected application before reporting a result.

## Manage task history

Use the delete button beside a task in Agents or Activity, or **Clear removable
tasks**, then confirm the listed items. Both views operate on the same saved
tasks: deleting one also removes its Activity entry and download authority.
The dialog reports partial failures; newly started work is not silently included.

Deletion is separate from stopping work. Active tasks and unresolved side effects
cannot be deleted. Child history stays until its parent is removed, and a parent
with unresolved children cannot be removed. Audit records, existing backups and
files created by tools are retained; this is history cleanup, not secure erasure.

Old task history without a checkpoint remains visible but is not resumable.
A damaged or unwritable task store disables task operations with an explicit
error. Preserve the data directory, repair the cause, and restart. Do not delete
history as a routine repair.

## API lifecycle

```bash
curl -X POST http://127.0.0.1:11611/v1/agents/run \
  -H "Content-Type: application/json" \
  -d '{"model":"YOUR_INSTALLED_MODEL_ID","prompt":"Calculate 25 * 47","async":true}'
```

Creation returns HTTP 202 with `run_id`. Poll `GET /v1/agents/tasks/{id}`.
POST actions to `/v1/agents/tasks/{id}/{action}`; approve/deny take
`{"approval_id":"...","async":true}`, resume/cancel take `{"async":true}`,
and reconcile takes `{"call_id":"...","result":"verified outcome"}`.

Do not submit `approved_tools`, `approved_tool_calls`, or a replacement prompt
to resume work. These are rejected. Run actions are bound to the initiating
account and are administrator-level surfaces. Task history is scoped to that
account plus legacy unowned history.

SSE mode (`stream: true`) reports durable status and completed steps, followed
by `done`, `approval_required`, or `error`. It does not emit model-token deltas.
A disconnect leaves the run active: recover by ID instead of resubmitting it.

## Tools and MCP

**Agents → Available tools** shows actual enabled tools and risk descriptors.
Disable unneeded tools there; choices persist across service restarts.
**Agents → Connections** tests and connects supported HTTP endpoints. Work,
Available tools and Connections share the navigation at the top of Agents;
management settings are separate from the task composer.
Supply a server URL, not an `npx` command in the URL field. Successful connection
configuration is persisted. External tools are still privileged, even when
served locally; inspect their capabilities and restrict what the service can access.

To remove an MCP server, open **Agents → Connections**, choose **Remove
connection** beside its name, and confirm. This forgets the saved connection,
closes its transport and removes its tools from discovery and execution. Saved
connections that cannot reconnect are also listed so they can be removed.
Task history and the remote server's data are not deleted. Tasks using the
removed tools may fail; removal does not undo actions already sent to a server.
Reconnect explicitly if you need the server again. Removal is administrator-only,
including through `DELETE /v1/agents/mcp?name=<URL-encoded connection name>`.
A persistence failure leaves the connection intact and displays a retryable error.

Computer use is unavailable unless a supported driver is configured. Do not
interpret a page or status card as evidence of working desktop control.

## Validation boundary

Regression tests cover run-ID approvals, duplicate/expired grants, denial,
cancellation, process-restart recovery, storage failure before execution,
checkpoint isolation, and browser reload/draft handling. Model/tool interactions
in those tests use controlled fixtures. They do not certify model reasoning,
external MCP servers, or real Hermes/OpenClaw tasks on every machine.

See the [API contract](../reference/api.md) and
[delivery plan](../advanced/product-reliability-plan.md) for remaining work.
