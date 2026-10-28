# Native-agent real-model fixtures

These opt-in Linux/WSL fixtures exercise the installed CLI and an already running,
matching service. They are separate from default CI and do not qualify another
model, backend, native desktop platform or the broader frozen workflow suite.

Prerequisites: an installed trusted CLI/executor, an explicitly enabled native
execution preview on a loopback service, an already installed model, Python and
Go 1.24+ on the host PATH. No model, runtime or Go toolchain is downloaded. For an
authenticated service, set `OFFGRID_API_KEY`; it is not printed or passed as an
argument. `--endpoint` accepts only a loopback HTTP origin and follows no redirects.

Run from a login shell so the reviewed `go` command resolves like the user's
terminal. Replace the example paths/model explicitly:

```sh
python3 scripts/test/test-native-agent-conversation.py \
  --cli /absolute/path/to/offgrid --model INSTALLED_MODEL \
  --endpoint http://127.0.0.1:11611 --allow-disposable-workspace

python3 scripts/test/test-native-agent-stop.py \
  --cli /absolute/path/to/offgrid --model INSTALLED_MODEL \
  --endpoint http://127.0.0.1:11611 --allow-disposable-workspace
```

The explicit flag authorizes only fixture-owned temporary folders, synthetic
files, disposable profiles and the trusted Go test command. Each fixture accepts
local consent only when the prompt identifies its exact new folder. It does not
approve applications, desktop control, arbitrary commands or another folder.
Trusted execution still uses the signed-in OS account; it is not a sandbox.

The conversation test requires one grant across listing, exact follow-up reading,
Unicode file creation/read-back and a real test command. It checks canonical root
identity, original-file hashes, execution-session continuity, actual output/exit,
no duplicate command start and the assistant's final test-result answer. Correct
tool arguments alone do not prove that a model satisfied the user's request.

For the typed user-input-write acceptance case, add `--require-input-source` to
the conversation command. The same prompt is used; the fixture additionally
requires every write to use `workspace_write_from_input` with matching original
request and selected-content digests. Exact host bytes remain required. For a
typed write, a matching native receipt qualifies only when it records the
writer's independent re-open/read-back, exact path/host/session, byte count and
digest, with matching original input and span provenance. Another model-issued
read is optional in that case. The separate follow-up-read turn remains required.
A correct file created by ordinary writing is not a pass for this additional
acceptance case. The original fixture rejection of a verified native read-back
is retained in the evidence rather than relabelled a complete workflow pass.

Add `--check-abstention` to prepend a greeting and a conceptual explanation.
Both must answer without workspace actions, followed by the same file and test
workflows in the same conversation. This distinguishes useful tool use from
calling tools merely because a folder grant is present.

The Stop test interrupts only its own CLI after its disposable test process starts.
It observes that process's identity and termination and rejects a late write.
Local termination and the service's stop acknowledgment are reported separately;
an uncertain service state is not silently reconciled or relabelled complete.

Failures are retained as failures. Cleanup targets only fixture-owned canonical
roots and profiles. It never resubmits an uncertain action. Temporary evidence
directories are private, remain outside the approved folder and are retained for
inspection; no automatic evidence deletion is performed. A failed Stop can leave
its bounded fixture process running briefly, which remains a failure.

Run at least three fresh invocations per advertised model/backend. Report every
failed attempt and the exact build/environment. Results belong in the existing
[native-agent evidence](evidence/native-agent-execution-2026-10-25.md), with full
suite and unavailable platform checks kept separate. Boundary-only fixture tests
can run without inference:

```sh
python3 scripts/test/test_native_agent_fixture.py
```

## Native-tool scope evaluation (development)

The `native-workspace-v1` candidate scope covers conversation, attached skill
loading and reviewed workspace file/process tools. Its frozen nineteen cases are
defined in `internal/models/native_workspace_suite.go`; every case needs three
repetitions on the same measured model/runtime. Case records, exact suite and
capability-catalog digests are required. A passed aggregate without those records
cannot select the scope. Scoped evidence projects as `scope_qualified`, never
general `workflow_qualified` readiness. All cases must pass with zero unauthorized
actions, repeated uncertain mutations and false completion before activation.

The runner filters discovery and the final model menu, then independently checks
every dispatch, including internal tools and saved calls. This scope excludes
desktop/browser control, remote MCP, delegation, memory changes and skill authoring.
Qualification adds no folder grant or profile permission. Native selection remains
disabled without an explicitly installed measured policy and matching evidence.

For an isolated candidate comparison against an already running local service:

```sh
OFFGRID_TEST_COMPLETION_URL=http://127.0.0.1:11611/v1/chat/completions \
OFFGRID_TEST_CONTEXT_MODEL=INSTALLED_MODEL \
OFFGRID_TEST_DECISION_ADAPTER=native \
OFFGRID_TEST_NATIVE_SCOPE=native-workspace-v1 \
OFFGRID_TEST_DISPOSABLE_EXECUTION=1 \
go test ./internal/server \
  -run '^TestNative(Skill|Conversation|ConversationWork)InstalledModel$' \
  -count=3 -timeout=35m -v
```

These tests use a synthetic adapter pin only in temporary task stores to exercise
the actual scope boundary. They do not install qualification or activate native
tools in the running service. Review semantic answer quality alongside mechanical
assertions. Real activation also requires measured installation/runtime identity
and the current capability catalog; this comparison alone does not supply them.
Set `OFFGRID_TEST_PROFILE_FIXTURE` to an explicitly reviewed exported profile when
testing that profile's skill instructions; it does not alter the installed profile.
Desktop/visual and broader release claims retain the full thirty-workflow suite.

The separately versioned `native-workspace-v2` suite uses the reviewed
`checklist-contract` fixture. It preserves the original suite and failed results.
For that suite, select `OFFGRID_TEST_NATIVE_SCOPE=native-workspace-v2`, set
`OFFGRID_TEST_RUNTIME_CONTEXT` to the independently observed context, and use
`^TestNative(ContractSkill|Conversation|ConversationWork)InstalledModel$` as the
test selector. The fixture digest and suite digest are enforced independently;
three formatting repetitions cover one case, not the other eighteen cases.

### Measured service activation

Startup reads the optional service-owned file
`DATA_DIR/model-capabilities/native-agent-installations.json` before binding the
agent runner. This is an installation/evaluation artifact, not a public API,
profile setting or model-generated configuration. Its version-1 document has a
`models` array. Each entry binds `model_id`, the complete measured `fingerprint`,
`scope`, `suite_sha256`, `capability_sha256`, `platform` (`OS-ARCH`), and a
`runtime_files` map of reviewed roles to absolute installation paths. The current
owner supports the two text-only workspace scopes; projector/preprocessing
qualifications cannot enter through this path.

The installation owner must enumerate the complete runtime dependency inventory,
including `executable` for the owned llama.cpp program and `offgrid-service` for
the running service image. Both service and engine bytes matter: changing agent
code invalidates evidence even if the engine build label is unchanged. Model
files come from the installed model registry, not this inventory. Hardware
identity excludes personal identifiers and volatile free-memory readings.

An inventory is **not qualification**. Selection additionally requires matching
passed per-case records in the service's capability-evidence store. Before
inference, the existing lifecycle lease protects an observation–measurement–
observation check against the actual owned child, model, template, context and
launch configuration. Operational failures do not rewrite recorded evidence.
No model loads or downloads happen during cold selection.

To avoid re-reading multi-gigabyte artifacts for every decision, the installation
owner can reuse already computed byte hashes in a bounded, process-local cache.
Reuse requires the same device/inode, size, mode, ownership, link count, and both
kernel change and modification timestamps on supported Linux local filesystems.
Equal size and a restored modification timestamp are insufficient. Changed files
are rehashed; unsupported filesystems and other platforms use full measurement.
Open-handle/runtime binding, current qualification, hardware, process generation,
template, context and launch configuration are still checked. This cache stores
no permissions, observations, model output or qualification results and does not
survive a service restart.

Missing inventory leaves the existing adapter available. Invalid inventory is
reported at startup without taking down ordinary service operation. New jobs
with stale measured evidence retain the compatibility path; already pinned
native jobs stop instead of silently switching adapters or replaying effects.
Removal/replacement of installed files still requires the ordinary ownership and
lifecycle rules. Hashing is not a sandbox or protection from an administrator
modifying a live process.

Qualification must use the exact candidate service/runtime and retain the actual
per-case evidence. Synthetic records used by temporary test fixtures must never
be copied into an installed service. The isolated comparison above and startup
unit tests do not establish that production activation has been performed.
