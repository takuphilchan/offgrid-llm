# Governed agent tasks

OffGrid's built-in task runner uses a local chat model and enabled tools. The
model must support structured tool calls; an installed model or successful
health check alone does not prove it can complete a useful task.

## Reusable agents in Chat

For verbatim text already in a task request, a compatible new job may offer
`workspace_write_from_input`. The agent selects unique boundary strings in that
root's request or an applied correction, and the service copies the selected
bytes through the same approved folder writer. It preserves Unicode and line
endings and verifies host read-back. It cannot select another job, prior tool
output or a pending correction, and it does not add permission. Ordinary writing
remains available. Verified bytes prove the selected span, not the correctness
of every model interpretation; see [qualification fixtures](../quality/native-agent-workflows.md).

Native agents share one runner for conversation, explanations, approved folder
reads/edits, trusted project commands, attached research connections, Knowledge
and configured collaborators. A tool being available does not mean it should be
called: a greeting or explanation normally needs no workspace inspection. The
selected profile, model and current grants determine the usable toolset.

Use `offgrid agent chat --model INSTALLED_MODEL --workspace-folder .` to keep a
conversation and its folder grant across follow-up requests. For project test
commands, explicitly select `--workspace-access trusted`; this uses your host
account and is not a sandbox. `agent run` is one foreground task, so its local
host session ends with that CLI invocation. Research and collaboration require
their corresponding authorized attachments; a folder grant supplies neither.

A proven rejected workspace file read is returned to the model for a bounded
correction, preserving the same root and recorded error. Repeated rejections
pause the task. This does not retry a write, renew access, settle an unknown
effect or treat an unavailable host as permission to use service files.

### Selected execution folders (preview)

On a matching service with `OFFGRID_ENABLE_EXECUTION_PREVIEW=true`, a native
client may explicitly approve a host folder. Chat's **Workspace** control selects
an already connected grant; it never creates access or uses the service cwd.
The selected host/folder and read/edit/command ceiling are shown before sending.
A disconnected selection is retained as unavailable, not silently substituted.
The host lease must remain running. In a matching desktop app, **Choose a local
folder** opens native folder selection and a native read/edit/trusted-command
confirmation. The renderer cannot nominate a path, service, command or approval.
**Stop workspace access** and the File/tray equivalent revoke that lease without
waiting for model inference. Quit, lock/suspend, account/workspace change or
renderer failure also stop the owned helper; unconfirmed stopping stays uncertain.
Older desktops without these optional IPC methods can still select a CLI host.
Browser users approve their host through the trusted CLI, not a web permission
dialog:

```bash
offgrid agent workspace start /absolute/test-project --access edit
# In another terminal, select the reported SESSION:
offgrid agent chat --profile PROFILE_ID --workspace SESSION
```

`trusted` access additionally permits project commands with your OS account's
authority. It is **not a sandbox**. For explicitly isolated Linux/WSL commands,
select an already-installed immutable image ID (no image is downloaded):

```bash
offgrid agent workspace start /absolute/test-project --access trusted \
  --backend isolated_linux --image sha256:YOUR_INSTALLED_IMAGE_DIGEST --network none
```

Only that folder is mounted at `/workspace`; edits there affect the host folder.
The default limits are 512 MiB, one CPU and 64 processes, configurable through
`--memory-mib`, `--cpu-millis`, and `--pids` before local review. `bridge` networking
must be explicitly selected. The current non-root host UID/GID is used. This
requires a native Linux host, local Docker socket and matching executor pack;
it does not silently fall back to trusted local commands when unavailable.

Source/release/desktop builds now stage a matching executor binary and digest
manifest. Native CLI installation retains versioned packs for rollback. Keep
CLI and executor assets together; a mismatched version/digest fails closed.
Updating assets does not restore saved grants or resume uncertain work.

Profiles may narrow host read/edit/process ceilings in **Advanced → Workspace**;
Chat's **Tools → Workspace** may narrow them again for a new turn. Inherit means
the selected, separately approved host ceiling, not automatic folder access.
All-false disables host tools. A turn cannot widen its profile, and a profile
cannot widen the actual host grant. Existing jobs keep their pinned policy.
Command activity distinguishes starting/running from actual exit, failed exit,
cancel/timeout and incomplete output; a start receipt is not proof of completion.

### Profile selection

On services advertising agent profiles and conversations, **Agents** is the
profile library. Create an agent, choose an installed model and instructions,
then select the Knowledge and tools it may use. **Use in Chat** selects the
saved revision. You can also select it from Chat's **Agent** control. Selecting
**No agent** returns future messages to ordinary Chat.

Leave **Attached agents** empty for one agent, or select up to four read-only
specialist profiles for bounded delegation. For example, a researcher can
gather evidence, then a document specialist can draft from those findings.
The parent agent receives the draft and performs any permitted document edit
and read-back itself. Child agents cannot control the desktop, change files,
or obtain broader access. They share the parent's budget; local runtime capacity
determines whether independent child work can actually run in parallel. Chat
shows each handoff and links to the saved child work.

Resource questions use the selected revision's effective attachments, not the
agent's fictional persona or earlier conversation. Document names are an
inventory, not evidence that every document was read. Content answers use
retrieved excerpts; no relevant excerpt does not mean no documents are attached.
Existing turns retain their pinned revision when you edit a profile.

The **Tools** control narrows resources for the next turn. Configuration is not
permission: computer use still requires host consent, and administrator-owned
connections and service filesystem tools do not become available to ordinary
Chat users. Without administrative grants, pure calculator/time tools remain
available. Profile instructions cannot override these restrictions.

Chat shows concise activity, access requests and results inline. **Inspect work**
opens the same job, not a copy. While supported work runs, **Send instruction**
queues a correction for its next safe boundary; actions already sent are not
cancelled. Reloading observes saved state and never submits again. If attaching
the answer failed, **Attach saved result** repairs bookkeeping without rerunning
inference or tools. An uncertain action is not replayed; a manual report remains
a human observation rather than independent proof.

Before asking you to report an uncertain computer outcome, the service checks
its saved, authenticated action receipt. If that result survived, it is recovered
without repeating the input and the task waits for explicit resume. If the host
disconnected before any conclusive receipt arrived, the outcome remains unknown;
do not repeat a submission just because the screen looks unchanged. Restart never
restores expired computer permission.

Profiles have immutable revisions. Save conflicts keep your draft; explicitly
load the latest revision or save a new profile. Archive removes future selection
without erasing historical work. Existing task links and the **Activity** inspector
remain available. This source implementation still has locale, integrated
client and native-platform qualification gates; do not infer deployment from
these documented contracts.

Hermes and OpenClaw are separate runtimes using OffGrid inference. Their own
tool execution is **not** governed by this runner's approval broker. See
[external agents](external-agents.md) for provider setup.

## Start and inspect work

Available tools and Connections remain separate, labelled administration views.

For an application request, the agent searches the full installed catalog by
name before launching. It must use the matching application, not a nearby entry
from a shortened list. If the name is missing or ambiguous, it asks for
clarification. A changed instruction or refreshed catalog requires a new lookup;
no extra computer consent is introduced by this check.
Their lists use the same headings and action controls as the rest of the workspace.
Testing a connection or opening tool details does not run a task or change its
approval policy. See [MCP setup](mcp.md).

Start work in **Chat**. Select a saved profile with **Agent**, describe the
outcome in the message, and send it. The linked job is saved before execution.
**Activity** provides history and a detailed inspector for that same
job, not another task composer. Model choice, tool restrictions, computer access
policy and voice controls live with the Chat composer. A model with reliable
structured tool calling is still required.

Available tools and remembered Full access are not instructions to act. A greeting
or clarification can finish as an ordinary answer without an application launch,
outcome declaration or access request. When real work is needed, the runner plans
from each observed result. Verified outcomes permit a final answer; outstanding
edits or unknown effects cannot be dismissed by prose. Profile instructions set
behavior while versioned computer protocols and the broker retain these limits.

When the model requests computer access, the saved task displays **Access needed**.
Full access must be enabled explicitly in the desktop app; it can then be reused
until the desktop session ends or the user revokes it. The task continues with a
fresh observation. Model-proposed names never grant access. Web users can
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

For work across applications, OffGrid verifies each current target under the
desktop task grant. The grant does not turn off action restrictions, and a
separate host file or service grant is still needed when applicable.

Legacy arbitrary workflow registration remains unavailable (`501
durable_coordination_unavailable`). Do not confuse bounded read-only delegation
with general recursive multi-agent or concurrent computer control. See the
[architecture](../advanced/ARCHITECTURE.md#bounded-durable-coordination) and
[quality evidence](../quality/README.md) for qualification limits.

### Reusable profiles (development API)

An owned profile saves instructions, an installed model, selected Knowledge,
authorized tools and a bounded iteration budget. Each edit creates a revision;
running work keeps its admitted revision. Missing models are reported, never
downloaded automatically. Settings are restrictions, not permissions.

Use one profile for single-agent work. A parent may also select up to four
direct read-only child profiles, with their revisions pinned at submission.
Children cannot control the desktop, delegate again, widen the parent's tools
or speak independently. Their reserved iterations come out of the parent's
budget. Existing unprofiled delegation retains the limits described above.

The service API is available in this development change; the profile library,
Chat selector and linked conversation experience are still being implemented.
See [profile API contracts](../reference/api.md#agent-profiles-development-api).

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

The current development CLI also accepts `offgrid agent chat --profile ID
--profile-revision N` and `--resume NAME`. Its default interactive command uses
saved conversation turns with the profile's budget. Explicit `--model` selection
is retained on follow-up turns and must match that profile's pinned model; it
does not silently modify a profile or pick a model from its name.
No profile/folder selection automatically grants host tools. A matching native
CLI can explicitly approve a folder and supervise a pinned worker in the
[native workspace preview](../reference/cli.md#native-workspace-preview).
Its reviewed file/command tools share canonical jobs and check their independent
grant, rather than borrowing computer consent. Matching desktop folder selection
and worker packaging are implemented in preview; general model/platform
qualification remains unfinished. Without that separate grant, service file tools
still mean the service filesystem, never the terminal's current directory.

The shared task UI restores the selected run after navigation
or reload, including pending approvals and committed tool results. Task drafts
save while editing, separately per account and identified workspace. Old
account-only drafts require explicit restoration; originals and newer text are
preserved. Browser drafts are not
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

In the updated renderer, **Next step** presents instruction updates only when
the saved task supports them. A lost save acknowledgment checks the same task's
recorded instruction ID; an explicit retry retains that ID. A terminal or
incompatible task instead offers **Start a new task from this**. This prepares a
draft, never changes the original result, never overwrites an existing new-task
draft and requires a separate **Start task** action.

Approvals remain attached to the exact task and call. Their expiry is visible;
after expiry, **Refresh** only reads the task, without approving or resuming it.
Unknown tools show their technical identity, an explicit effects warning and
exact canonical arguments. Stop immediately shows a pending acknowledgment;
failed acknowledgment remains labelled unconfirmed, not undone.

### Read results and output files

Saved answers and **Output files** precede the collapsed activity log. Generated
answers are not independent verification. **File integrity checked** specifically
means the service recorded checks of stored bytes and format, not factual
correctness of the contents. The renderer rechecks the downloaded bytes against
the selected digest and length before offering a file.

Downloads use the owner-authorized task artifact endpoint. Revoked, removed or
failed-integrity files show an error beside that file while retaining the saved
answer. There is no executable embedded document preview; download an unfamiliar
format and inspect it with an appropriate application. **Export evidence** under
**Context usage** retains the saved job record for inspection. These views do not
independently prove that a model's requested real-world outcome occurred.

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
In a terminal, `agent run` offers the current local folder once, then streams the
same saved job until its result or intervention. `--workspace-folder FOLDER` and
`--workspace-access read|edit|trusted` select/narrow that local session explicitly.
Declining or failing local approval submits no service-directory substitute.
`--detach` explicitly submits in the background; use an independently approved
`--workspace SESSION` for local files, since a detached run cannot retain a host
owned by the exiting CLI. Piped/nonterminal calls keep asynchronous submission
unless `--wait` or an explicit local folder requests following. JSON submission
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

Reconciliation records your statement as **human-reported**, preserving the
original call arguments and timestamp. It is not independent verification and
does not mark unrelated outcomes as complete. Typing `done` cannot prove the
requested document matches. Reconciliation does not
execute the uncertain tool again. If you cannot determine the outcome, leave the
run uncertain. Do not invent success or retry a destructive operation blindly.

An ordinary failure of a built-in file read, directory listing, calculation or
clock read is a failed task, not an unknown side effect. It does not require an
outcome form. A read interrupted by service restart can be explicitly resumed;
it is never automatically rerun on startup. Unknown failures of mutating or
external tools remain conservative, including legacy records without reliable
effect metadata.

A known argument or protected service-path rejection before filesystem effects
instead gets bounded corrective feedback. The runner allows up to three repair
attempts, then pauses with the rejection reason. You should not have to fill in
an unknown-outcome form for an operation proven not to have executed.

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

### Invalid proposals

Malformed computer-access proposals and stale/invalid computer actions rejected
before dispatch can be corrected in the same task. OffGrid records rejected
proposals as unexecuted and allows at most three consecutive automatic repairs.
If these fail, the task pauses with an explanation; correct the instruction or
review model compatibility, then explicitly resume. The repair count survives
restart. Scope/policy denials and uncertain effects do not enter this repair
loop, and old failed tasks are not automatically resumed.

Returning instructions or asking you to paste tool JSON after a rejected
proposal is not a successful repair. Those answers use the same bounded repair
budget and cannot complete the task. Explicit resume refreshes the budget but
does not erase the unresolved proposal. An explicit correction can change the
requested outcome; uncertain dispatched actions still require reconciliation.

When a model only returns text, the task is labelled **Answered**, with a notice
that no external tool action was performed. This is appropriate for writing and
question-answering, but it is not evidence that Chrome opened or a document
changed. A selected model still needs reliable structured tool calling.

## Tools and MCP

**Agents → Available tools** shows actual enabled tools and risk descriptors.
Disable unneeded tools there; choices persist across service restarts.
**Agents → Connections** tests and connects supported HTTP endpoints. Work,
Available tools and Connections share the navigation at the top of Agents;
management settings are separate from the task composer.
Supply a server URL, not an `npx` command in the URL field. Successful connection
configuration is persisted. External tools are still privileged, even when
served locally; inspect their capabilities and restrict what the service can access.

In task-first runs, granting computer access adds the available computer tools
without removing permitted MCP tools. Registry discovery applies the current
actor policy and hides disabled/denied tools; execution checks that policy again.
Unreviewed local shell/script tools are excluded from task-first work even under
another name. Reviewed workspace commands require their separate execution grant
and invocation policy. Computer policies do not authorize connected-service operations.
The agent may research through MCP before observing an application, but it must
observe before computer input and verify after completing computer work. A tool
result from an external service cannot substitute for application verification.

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

### Outcome checks and bounded completion repair

New task-first native workflows retain declared outcomes across context
compaction and reconnect. Completion with pending checks receives three
corrective opportunities in the same saved task, then pauses with remaining
checks intact. Only newly verified progress resets that budget; repeatedly
invalidating and rechecking the same outcome does not. Viewing or reconnecting
does not resume work. Use existing steering at a safe paused boundary to revise
the request; earlier revisions remain recorded.

Unsupported verification permits an explicitly unverified or partial result.
Neither a successful click nor generated prose is proof of task completion.
Uncertain mutations still require reconciliation and are never repeated by
completion repair. Service restart invalidates host access, even when the task's
evidence and repair count are restored.

Regression tests cover run-ID approvals, duplicate/expired grants, denial,
cancellation, process-restart recovery, storage failure before execution,
checkpoint isolation, and browser reload/draft handling. Model/tool interactions
in those tests use controlled fixtures. They do not certify model reasoning,
external MCP servers, or real Hermes/OpenClaw tasks on every machine.

See the [API contract](../reference/api.md) and
[roadmap](../ROADMAP.md) for remaining work and [quality evidence](../quality/README.md) for recorded checks.

## Reusable skills and profile memory

On a service advertising `agent-context-resources-v1`, the profile editor's
Advanced section includes Skills and Profile memory. Older services do not gain
these controls by interpreting saved labels as permission.

Import an offline Agent Skills folder containing `SKILL.md` and optional relative
support files. Imports allow at most 64 files, 64 KiB per file, 512 KiB total and
32 KiB of UTF-8 `SKILL.md`. Links, traversal and duplicate/case-colliding paths
are rejected. Nothing is downloaded or executed. Scripts and `allowed-tools`
frontmatter are context data, not tool permissions. Inspect before attaching.

```bash
offgrid agent skills import ./my-reviewed-skill
offgrid agent skills list
offgrid agent skills show SKILL_ID REVISION DIGEST
offgrid agent skills attach PROFILE_ID PROFILE_REVISION SKILL_ID REVISION DIGEST
```

Attachment saves a new immutable profile revision. Existing tasks retain their
original revision. The model sees only attached names/descriptions initially;
`skill_load` reads a selected file when needed. `skill_draft` stores an inert
draft with job provenance. Inspect it, then use `skills review ID REVISION DIGEST`
to create a reviewed copy, and explicitly attach that copy. Review never executes
it or silently changes a profile.

Skill authors can optionally declare a reviewed `response_contract` in a new
revision's frontmatter. It contains `version: 1`, an RE2 `pattern` matching the
whole answer, and a `max_bytes` limit. Write the same output requirements clearly
in the instruction body; the model receives those instructions, while code
checks the contract. See the [skill API contract](../reference/api.md) for limits.

Once loaded, the format applies to the entire final answer, including any
clarification or limitation. Review the pattern for those cases before attaching
the revision. A mismatch permits at most two answer-only corrections within the
turn's existing budget. Corrections cannot run tools or repeat earlier actions;
if the answer still fails validation, the turn fails with its earlier work saved.
This verifies format, not factual accuracy. Existing installed revisions are not
changed automatically, and conflicting contracts can prevent completion.

Automatic durable memory is distinct from transcript compression and defaults
off. It is scoped to your account, workspace and profile. Enable it explicitly
in the editor, or inspect/change its exact consent revision:

```bash
offgrid agent memory show PROFILE_ID
offgrid agent memory consent PROFILE_ID CONSENT_REVISION on
offgrid agent memory consent PROFILE_ID CONSENT_REVISION off
```

Only new root tasks pin the enabled consent. They can use bounded `memory_read`
and `memory_write` in their existing tool budget. Revocation blocks subsequent
reads/writes, and re-enable never restores old tasks. Children do not acquire
independent context-resource authority. Memory is untrusted historical context,
not a current observation, proof of accuracy or permission. Avoid credentials
and unrelated private content. No extra background model captures memory.

Inspect, edit or delete entries even when automatic use is off. `memory put
PROFILE FILE` accepts JSON containing a stable `operation`, `expected_revision`,
`label`, `content` and optional existing `id`; use `-` for stdin. Conflicts preserve
newer text. `memory delete PROFILE ID ENTRY_REVISION` removes live text and future
retrieval, not saved task evidence, backups or forensic SQLite remnants. Limits
are 8 KiB per entry, 128 entries/512 KiB per profile and bounded model retrieval.
Successful persistence is not evidence that every local model will recall or
apply the information correctly; real-model usefulness requires separate tests.

## Reviewed calendar schedules (preview)

Activity → Schedules manages recurring work on the same service. Choose an exact
profile revision, daily/selected weekdays, IANA timezone and wall time. Review the
model, resources, execution folder/backend, permissions and decision budget before
saving. New editor drafts are disabled until explicitly enabled. Creation,
inspection and refresh never execute work. No desktop consent is available to a
schedule; an existing independently approved execution host may be selected.
Trusted local commands run as the host account and are **not sandboxed**.

CLI controls use the same authenticated APIs:

```bash
offgrid agent schedules list
offgrid agent schedules create schedule.json
offgrid agent schedules show SCHEDULE_ID
offgrid agent schedules history SCHEDULE_ID
offgrid agent schedules update SCHEDULE_ID REVIEWED_REVISION schedule.json
offgrid agent schedules disable SCHEDULE_ID REVIEWED_REVISION
```

An example policy (replace the profile identity with one you inspected):

```json
{
  "settings": {
    "name": "Morning briefing",
    "prompt": "Summarize the supported findings in my attached Knowledge documents. Report missing evidence.",
    "enabled": true,
    "calendar": {"zone": "Africa/Harare", "hour": 9, "minute": 0, "weekdays": [1, 2, 3, 4, 5]}
  },
  "profile_id": "REVIEWED_PROFILE_ID",
  "profile_revision": 1,
  "max_iterations": 8
}
```

The service must remain awake and connected. It does not wake the OS or catch up
missed slots. DST gaps skip; repeated wall times use the first instant. Startup,
clock jumps or delayed preparation can record missed work. Prior running,
approval-waiting, interrupted or uncertain work blocks overlap. Occurrences link
one canonical root in Activity; there is no schedule replay/run-now button.

Profiles/models/budgets/grants are pinned, not silently upgraded. Knowledge
excerpts refresh under the same document IDs before admission; stale excerpts do
not become new authority. Current actor, profile, memory consent, tool policy and
host access are checked again. Unavailable resources record blocked occurrences;
reconnection does not replay them. Disable revokes new actions before stopping
owned jobs; already-dispatched effects still require truthful settlement.
Re-enable requires full review and never reauthorizes an older root. Storage
retains policy revisions and bounded occurrence history; disabling is not erasure.

Clock/storage/scripted-runner and browser fixtures verify these mechanics, not
local model quality, installed-platform control or unattended production readiness.
