# Governed agent tasks

OffGrid's built-in task runner uses a local chat model and enabled tools. The
model must support structured tool calls; an installed model or successful
health check alone does not prove it can complete a useful task.

Hermes and OpenClaw are separate runtimes using OffGrid inference. Their own
tool execution is **not** governed by this runner's approval broker. See
[external agents](external-agents.md) for provider setup.

## Start and inspect work

Start the service with `offgrid serve`, then choose an installed model from
`offgrid list`. With authentication enabled, set `OFFGRID_API_KEY` in the client
environment; do not put keys in screenshots or issue reports.

```bash
offgrid agent chat --model YOUR_INSTALLED_MODEL_ID
offgrid agent run "Calculate 25 * 47 and report the result" --model YOUR_INSTALLED_MODEL_ID
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
`system_prompt`.

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
**Agents → MCP connections** tests and connects supported HTTP endpoints.
Supply a server URL, not an `npx` command in the URL field. Successful connection
configuration is persisted. External tools are still privileged, even when
served locally; inspect their capabilities and restrict what the service can access.

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
