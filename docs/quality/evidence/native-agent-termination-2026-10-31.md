# Native agent termination diagnosis — 2026-10-31

The reviewed-agent artifact journey initially failed. The final candidate below
passed three fresh Windows CPU runs with the named model; this does not qualify
other models/platforms, the broader workflow suite or the whole release.

The prior private worker trace (`offgrid-agent-diagnostic-8m389t1s`) showed a
147-byte complete JSON tool proposal followed by further non-whitespace,
non-EOG tokens. The service retained only the parser's JSON content and waited
for clean termination; no artifact was dispatched before cancellation.

This continuation tested the actual installed-model metadata rather than its
filename: Qwen 2.5 3B, qwen2 architecture, GPT-2 tokenizer, EOS 151645, context
metadata 32768. Its 2509-character embedded template was extracted privately.
No weights or packages were downloaded. Test processes used ephemeral loopback
ports and disposable state, not the installed service or user workspace.

## Results

- A diagnostic extension of the pinned native parser test binary passed all
  118 tests and 587 assertions with a simple schema. Repeating with the actual
  embedded template and captured service schema passed 589 assertions, including
  complete message/artifact acceptance and rejection of trailing prose and a
  second JSON object. The temporary native test source was restored afterward.
- A normal bundled CPU worker answered minimal `json_schema` and `json_object`
  requests with `finish_reason=stop` in 2.70s and 0.81s. An actual-schema short
  artifact request also stopped normally in 6.02s.
- The actual service-to-worker request was captured using a **test-only Go
  overlay** against synthetic profile/task data. It contains the owned strict
  single-step schema, temperature 0, no native tool list, `parallel_tool_calls`
  false, streaming with usage, and a 2048 output-token budget. Application source
  and production logging were not modified to capture requests.
- Replaying that body with streaming disabled, a diagnostic 128-token output
  cap and a 2048-token worker context completed in 95.97s (1978 prompt tokens,
  42 output tokens). Its artifact content omitted the requested final newline;
  this is not an exact-content success.
- A streaming replay with an 8192-token context, batch 128 and the same bounded
  128-token output cap reproduced the failure in 111.91s. The content stream
  ended its JSON proposal, but the worker kept sampling until the token limit.
  Its final event was a 500 `server_error`: output did not match the expected
  `peg-native` format. There was **no successful terminal finish reason**.

The normal bundle and the original desktop-test worker have identical SHA-256
`9032f1a3d30a57268ce99c297024d1190db3c44b5adca4c5a5ae66ebcdb6eeba`.
This rules out a different normal worker binary between those two tests. The
token-logging diagnostic binary has a different digest and is not a release
asset. All diagnostic processes above reached terminal state and were stopped.

## Next comparison and boundaries

The streaming and non-streaming replays also differed in context allocation and
batch size. Therefore this evidence **does not yet prove streaming is the sole
cause**. Repeat with identical worker settings, then inspect runtime sampler
state if the discrepancy persists. Generated-grammar acceptance by itself does
not prove runtime sampling enforces that grammar at every token.

Do not dispatch on a complete-looking JSON prefix, ignore the error event,
invent `finish_reason=stop`, extend production deadlines, or claim a false
artifact result. The existing service rejection remains appropriate. Exact
newline/content verification and the real desktop journey remain required even
after clean termination is repaired.

Local private evidence: `build/workspace-outcome-experience/schema-diagnostic-tests*.log`,
`schema-diagnostic.gbnf`; temporary directories `offgrid-agent-diagnostic-qc8255k1`,
`offgrid-schema-probe-hwpi576q`, `offgrid-schema-probe-tid4y3gv` and
`offgrid-schema-probe-53ea6tnt`. Verbose runtime logs contain synthetic test
prompts and are not included in ordinary exports or this repository.

## Confirmed filename grammar defect and candidate repair

The identical-settings non-streaming comparison also returned HTTP 500
(`offgrid-schema-probe-iuh04xlq`). Streaming is not the sole cause.

The generated artifact-name rule was `"\\\"" ([^/\\\\:]+ ".txt") "\\\""`.
Its negated character class permits a raw quote and subsequent braces/prose;
the native converter does not intersect this regex with JSON string syntax.
A new model-free native test demonstrated that a complete artifact JSON object
followed by ` trailing prose.txt"}}}` is **invalid JSON but accepted by the old
grammar**. Earlier rejection tests had no later filename suffix, so they did
not expose this alternate parse. This explains why sampling could continue
after the parser had exposed the complete-looking JSON prefix.

The candidate uses JSON-aware string generation by removing `pattern` only
from the copied, private single-step generation schema. It retains type/length
constraints, local-reference rebasing, the original tool catalog and final
schema validation. Schema traversal preserves literal const/enum/default data.
No artifact format, filename authority, user instruction or execution grant is
changed. Native external-tool and public response-format paths are unchanged.

Regression tests also exposed missing final argument-schema validation at the
single-step SSE emission boundary. The candidate adds the same complete-decision
gate already used by non-streaming and native-tool responses. Invalid filenames
now fail before a completed call reaches a public callback. Existing runner
authorization remains independent.

Evidence so far:

- Native diagnostic: **118 tests, 588 assertions passed**, including reproduction
  of the malformed-JSON acceptance and its rejection after schema projection.
  Temporary native source restored to SHA-256
  `2467bdad40a26b53dffe9d29392218249cc8ccf045d3f3537ea22a07f2e1994e`.
- Identical normal worker/settings with the projected schema terminated with
  `finish_reason=stop`: cold non-streaming **96.78s**, warm streaming **6.84s**.
  These are diagnostic observations, not a streaming performance comparison.
  The cold result still omitted the requested final newline; the streamed result
  preserved it. Clean termination is not exact-content qualification.
- `go test ./internal/inference -count=1`: passed (22.584s), including streamed
  and non-streamed pattern enforcement, Unicode/quoted filenames, immutable
  original catalogs, nested definitions and literal data preservation.
- `go test ./internal/server ./internal/agents -count=1`: passed
  (105.947s and 57.430s). Existing artifact authority/recovery checks remain intact.
- `go build -o build/workspace-outcome-experience/offgrid-schema-fix.exe ./cmd/offgrid`:
  passed. Source-size ratchet passed.

Private diagnostic evidence: `filename-grammar-tests.log`, `schema-inference-tests.log`,
`schema-agent-tests.log` under `build/workspace-outcome-experience`, and runtime
probe `offgrid-schema-probe-t6vzw2k7`. No diagnostic worker or verbose logging
was installed. The packaged desktop journey still determines whether this
candidate resolves the real user-facing artifact workflow.

## Packaged desktop result and remaining exact-content failure

The first candidate package correctly refused startup because its newly built
service had the default `dev` identity while the desktop was 0.4.21. This was
a qualification-package assembly error; the guard was preserved. Rebuilding
with `-trimpath -ldflags "-X main.Version=0.4.21"` corrected the test package.
The failed workspace `offgrid-desktop-native-reply-e3VXel` remains evidence.

The next fresh packaged run (`offgrid-desktop-native-reply-TfikSh`) reached
`completed` in **176.724s**, with exactly one `task_save_artifact` action and one
private download. It no longer hung on the initial structured decision. However,
the independent downloaded-byte comparison **failed**: the 39-byte result
omitted the requested final newline. The stored-byte digest/format verification
was accurate for those bytes, but it did not prove fulfillment of the user's
exact-content request. This is not a successful end-to-end qualification, and
the preview/restart assertions after that comparison were not reached.

The user also opened the image picker in that disposable window and encountered
a missing `home/Desktop` path. The harness had created AppData but not standard
user folders. It now creates and checks Desktop/Documents/Downloads/Pictures/
Music/Videos before launch, and labels its windows as isolated verification.
The missing folders were also created in that specific retained test home.
No installed-user Desktop or other user data was moved or removed.

A follow-up candidate clarifies the existing artifact content description:
exact user-supplied text must not be trimmed, including requested trailing
newlines. This is guidance, not a deterministic fidelity guarantee. The frozen
byte comparison remains unchanged; no newline is appended by the service and
no failed evidence is discarded.

## Final candidate: three packaged passes — 2026-11-01

The whitespace-guidance candidate stored the correct 40 bytes but its desktop
poll missed completion at the unchanged 180-second boundary
(`offgrid-desktop-native-reply-7QppGj`). Retained storage independently matched
the exact requested bytes/digest. The durable event log subsequently recorded
completion before teardown; the harness still correctly reports its timeout,
not a pass.

The next inspection found that the single-step adapter moved its catalog from
before the current user request to after it on the tool continuation. Even an
unchanged catalog therefore invalidated a large cached prompt prefix. The final
candidate anchors it before the current user exchange and retains that position
across tool results. Canonical history, descriptions, selected tools, authority,
final validation and deadlines remain unchanged. A regression compares the
serialized message prefix before/after an actual tool-result continuation.

Three fresh packaged desktop runs then passed **all** existing assertions:

| Isolated workspace suffix | Agent journey | Result |
|---|---:|---|
| `W0Ehkl` | 82.033s | Passed |
| `TsTG2q` | 92.502s | Passed |
| `CGL9iE` | 82.062s | Passed |

Each run used the UI to create a resource-free reviewed profile, submit one
private-download request, verify one action and the exact 40-byte result,
check SHA-256 `956f95c9c1e5225979c0b44c1826e39d45e42d4231f4ca7d8238de5473a8a23d`,
inspect its preview, and restart without replay or changed saved history.
No host executor, desktop grant, external connection or installed workspace was
used. The normal worker digest above is unchanged. Service candidate digest:
`aeab8926d0e2e046b0916e248187be36df9d690f5128748c1a97b643e0a246c8`.
Model: Qwen 2.5 3B Q4_K_M, Windows CPU, two threads, 8192 context.
These observations are not a general latency guarantee or an all-model claim.

Final screenshot review found a duplicate integrity explanation in the artifact
preview. The duplicate was removed while retaining the explanation once above
the output list; all **26 artifact/result browser tests** passed afterward.
This small presentation follow-up does not change inference or download bytes.

`go test ./...` passed after preserving ignored diagnostic overlay sources under
underscore-prefixed filenames (Go had initially tried to compile two diagnostic
copies under `build/`). The initial failure remains in `release-go-tests.log`;
the passing run is `release-go-tests-r2.log`. Inference tests also passed after
the prefix fix (22.568s). Windows dependency packaging passed with the installed
Visual Studio CMake added to the test process PATH; the first invocation lacked
that tool and is retained. These setup corrections did not change product code.

Commands and logs are retained under `build/workspace-outcome-experience`:
`desktop-schema-fix-test-r4.log` through `r6.log`, `schema-prefix-tests.log`,
`release-runtime-package-r2.log`, and `release-artifact-ui.log`. Complete private
results/screenshots remain in the named disposable directories. No failing
fixture was weakened and no production deadline was extended.

## Pre-publication regression checks — 2026-11-01

The final built renderer passed **666 tests with 28 opt-in/environment skips**
in 5.7 minutes. This invocation did not set `OFFGRID_VISUAL_PHASE`; the previous
682-pass run includes the additional 16 visual cases. The final history-feedback
p95 was 49.3ms; warm routes ranged 43–121ms and cold routes 433–734ms. Cold route
cost is reported separately, not claimed to meet the warm 250ms gate.

Desktop contracts passed **92 tests with one skip**, companion contracts **73**,
and Python SDK/worker protocol checks **74**. The first Python invocation lacked
pytest; an isolated ignored virtual environment supplied test dependencies and
the unchanged suite passed. No installed inference Python environment changed.

The CI Linux race package list passed except the executor's subprocess-build
test, which exceeded its unchanged 60-second deadline during the first parallel
cold build. Running that package alone passed in 24.721s. This is a retained
initial failure and passing rerun, not a claim that the first suite was green.
Full `go test ./...` had already passed on Windows. Structural, provider-hook,
documentation and scoring unit checks passed **42 tests**; maintained docs,
repository/application/developer layout and source-size checks passed. API
generation, TypeScript/build, strict change validation and workflow syntax also
passed. Native Windows parser and compiled dependency-bundle checks passed as
recorded above. Remote CI and release packaging still require their own result.

Commands are the existing `.github/workflows/ci.yml` commands, with Windows
`npm.cmd` and an isolated Python interpreter. Logs are retained under
`build/workspace-outcome-experience/release-*`; no private captures or raw model
traces are included in Git. Other-platform native behavior, real vision quality,
screen-reader/manual usability and participant gates remain unverified.

### CI reconnect-test correction

The first pushed candidate (`12146185`, CI run `41258825366`) passed parser,
Windows dependency packaging and both Python jobs, but its native jobs cancelled
seven desktop tests. The first background-reconnect test awaited an unreferenced
production timer without a test-owned event-loop handle. Electron's window keeps
the real application alive; standalone Node 22 correctly exited that test wait.

An explicit two-second referenced observation deadline now keeps only the test
alive, rejects a missing state, and cleans up its listener/timer. The production
timer remains unreferenced so Quit is not delayed. The original test reproduces
the cancellation under CI's Node **22.23.2**; the corrected runtime suite passes
**23/23** under that version, and the full local desktop suite passes **92 with
one skip**. Linux Node 24 passes the corrected suite too. Logs retain before/after
results under `runtime-reconnect-node22*.log`; cross-platform CI must rerun on the
corrected source before tagging.

The next revision (`f00f607d`, run `41259715522`) passed both macOS native jobs
and the earlier failing desktop test on Windows. Its Linux browser job passed
658 cases but failed eight startup comparisons because they read historical CSS
with `git show` from a shallow checkout. The exact baseline file is now a test
fixture (SHA-256 `3622a5a27f91d402f4489506f7e0b4c037f80b8afd417b84d8391fb7f872902d`),
not a changed baseline or skipped assertion. All eight comparisons passed locally
after this fixture repair. The historical CSS remains test-only and is not shipped
in the renderer. CI must still complete on the final revision.
