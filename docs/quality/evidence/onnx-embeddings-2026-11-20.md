# ONNX embedding implementation and qualification evidence

Date: 2026-11-20. Existing change: `inference-provider-runtimes`, tasks 9.1–9.4.
This is an in-progress implementation record, not a completed ONNX integration.

## Shared ownership prerequisite

Inspection found that `EmbeddingEngine.loadLocked` discarded `Unload` failures
after a failed load or invalid dimension handshake. A partially started runtime
could disappear from ownership and shared resource accounting. The new regression
reproduced both paths (failed in 0.949 s), preserving the original error assertions.

The embedding owner now retains that implementation without declaring it loaded,
records unresolved residency and sticky retirement uncertainty, and poisons the
existing scheduler's admission. Another embedding/native/legacy allocation cannot
reuse capacity. Explicit final cleanup still reaches the same implementation;
repeated failures preserve the first uncertainty without building an unbounded
nested error chain. A successful later cleanup does not silently rehabilitate
the poisoned service scheduler.

The focused ownership regression passed in 2.396 s, and the full Windows
`go test ./internal/inference -count=1` suite passed in 86.832 s. Independent review
then identified stale parked-reload metadata after a failed load whose cleanup
succeeds. The failed-load path now clears that metadata, requires a new explicit
load, and preserves any retirement poison. Added ordinary-cleanup/parked-reload
and partial-factory ownership tests verify these branches. All embedding-focused
Linux race checks passed in 1.315 s, including idempotent repeated cleanup; live
namespace/listener race checks passed in 3.291 s. No real process-stop failure or hardware reclaim qualification is inferred
from the injected backend error test.

## Worker implementation in progress

`runtimes/python/onnx_embedding_worker.py` implements a CPU-only offline worker
using the existing provider-v1 control/event envelope. It requires exact installed
dependency versions, explicit local graph/tokenizer/settings bytes, pinned
preprocessing, declared symmetric BERT tensor/pooling behavior and explicit CPU
execution. It rejects external graph tensors/custom domains before ONNX Runtime
loading, disables execution-provider fallback/telemetry, validates padding,
rejects token overflow instead of truncating input, masks padding for pooling,
and validates output shape, finite values and nonzero norms before normalization.

The worker applies bounded input/result frames, batches, token limits and output
vector counts. Cancellation uses the ONNX run option and the existing owning
process boundary; EOF cancels and bounds its generation-thread join. Unknown
memory use is reported as unmeasured. This does not impose a whole-machine hard
RAM limit or establish network-denied operation.

Nine Python contract tests passed under WSL Python in 0.370 s, including actual
NumPy pooling/norm arithmetic, JSON/dependency/settings/graph rejection, token
overflow and cancellation before results. They did not import ONNX Runtime or
load an ONNX model. After tightening the schema-version type and vector byte
budget, the full `runtimes/python/tests` pytest suite passed: 46 tests and 14
subtests in 1.50 s, including the existing MLX/Transformers worker contracts.
Later Python checks added exact Go base64url argument decoding and constrained
Python import paths: the complete Python suite passed 48 tests and 14 subtests
in 1.41 s. This still did not import ONNX Runtime or execute an ONNX model.

## Package, protocol and service integration

The package store now imports the bounded `onnx-embedding` layout, verifies
graph/tokenizer/settings digests and retains its existing model lease. Typed
provider embedding events reject wrong capabilities, non-finite/zero/ragged
vectors and cross-type fields. Batch usage is the sum of independently bounded
inputs rather than a falsely enlarged per-input context. `ONNXEmbeddingImpl`
withholds vectors until valid terminal counts/usage; it runs under the existing
EmbeddingEngine/scheduler, not another executor. Generation retirement uncertainty
also immediately marks the embedding owner unavailable and poisons admission.

Explicit startup configuration connects the runtime through the existing core.
The registry projects installed packages as `embedding:<id>@<revision>` for the
embedding API and Knowledge, rejects chat/delete fallback to colliding legacy
IDs, and does not initialize missing storage during read-only model listing.
Import/enrollment does not auto-select the new provider. Package identity is
compared against the persisted Knowledge identity before retrieval/re-enabling.
OpenAPI model engine/format enums and generated TypeScript include ONNX while
retaining concurrent browser pairing contract additions.

Before the namespace finding below, actual isolated Go-worker journeys passed:

- Windows focused inference/models checks: 12.148 s / 0.940 s; later expanded
  invalid-terminal/path/namespace checks: 83.256 s / 6.854 s under concurrent load.
- Public embedding handler, real owned fixture process, durable Knowledge
  ingestion/retrieval and changed-package identity refusal: 6.897 s. A follow-up
  verified that initial auto-enable does not select the newly imported provider:
  5.882 s. Public collision tests plus that journey: 6.807 s.
- Explicit core configuration without startup inference: 0.459 s.

The first public test compilation referenced a nonexistent `RAGContext.Sources`;
it failed before running and was corrected to the actual `Results` contract.
These fixture checks did not run ONNX Runtime, measure semantic quality or use
model weights. More importantly, their writable-runtime factory setup is now
**superseded** by the stricter namespace requirement below; they cannot establish
successful execution of the final provisioned mode.

## Review finding: directory handles do not seal Python imports

Independent review asked whether retained Windows directory/file read handles
prevent creation of new modules. The actual isolated regression failed in
0.710 s (test body 0.04 s): both the runtime root and `Lib/site-packages` accepted
new unpinned `.py` files while all original handles remained held. No finding was
erased or resolved by rescanning the same mutable namespace.

Final Windows admission now requires administrator/SYSTEM-owned runtime
directories/files, write/owner/ACL rights limited to trusted privileged
principals, non-retargetable ancestors, and a non-elevated service without
available ownership/restore/debug/impersonation overrides. Existing-file pins
remain held. Model data stays within the existing package/file lease; its fixed
graph/tokenizer/settings names cannot introduce executable import paths.
Python `._pth` files are checked before launch because CPython's path
configuration can explicitly enable site imports despite ordinary launch flags.
The worker also verifies executable/prefix/import roots before dependency imports.
Administrator/OS mutation is explicitly outside the boundary.

The final namespace rejection/ACL-policy regression passed in 1.496 s. Two
intermediate compilation failures (missing x/sys `FILE_DELETE_CHILD` constant,
then `ACCESS_MASK` versus `uint32`) preceded that pass and also blocked a sibling
browser fixture build; those were inference compile failures, not browser
results. The documented `0x0040` directory right and correct access-mask type
fixed both. Source-size checks are clean without new exemptions; protocol event
validation was moved intact into its cohesive helper file.

This environment is not elevated and has no administrator-provisioned ONNX
fixture/runtime. Successful final factory, public API/Knowledge and live
descendant qualification therefore remain **unrun**. Their retained positive
tests now explicitly require `OFFGRID_ONNX_WINDOWS_FIXTURE_RUNTIME`; absence
skips the external prerequisite rather than using a mutable substitute. The
separate namespace rejection test accepts only secure admission refusal or
demonstrated inability to add modules, preserving its original safety invariant.
The [qualification procedure](../../advanced/onnx-embeddings.md) supplies a
fresh isolated administrator-only fixture provisioner and exact commands. Its
PowerShell parser check passed; it has **not been executed**. Ordinary installed
packaging and macOS ownership remain incomplete. Linux source integration is
described below; its full provisioned execution is likewise unrun.

## Digest-bound snapshots and restricted Linux source integration

The trusted worker arguments now carry graph/tokenizer/settings digests from
the verified manifest. The Python worker hashes the exact immutable byte buffers
it then passes to the parsers. Changed bytes fail before model parsing; later
path mutations do not alter accepted buffers. External tensor data/custom
execution remains rejected. Windows retains its file leases; Linux may use the
ordinary package store because model input authority is now those verified
buffers, not continued path immutability.

Linux factory source reuses existing root-owned runtime DAC/inventory checks,
private state lease, pinned namespace launcher and retained pidfd/PID-namespace
owner through ProcessProvider. No runtime paths become writable or implicitly
provisioned. A self-contained Python prefix/import tree is required. Generic
process startup now owns both pipe pairs explicitly, closing child-side parent
copies even when namespace validation fails before `exec.Cmd.Start`; otherwise
that early rejection could leak those handles. A repeated pre-start failure FD
regression and a real protocol-through-namespace descendant retirement test are
prepared, along with settings mutation/FIFO and writable-runtime rejection tests.

The final Windows configuration/namespace/vector and immediate generation
retirement-poison regressions passed in 0.419 s before this Linux follow-up.
The updated Python suite passed 49 tests and 14 subtests in 1.57 s, including
the exact-buffer digest regression (the dedicated ONNX suite passed 12 tests
and 14 subtests in 1.16 s). Independent read-only review found no new blocker
in the Linux runtime/args/inventory, pipe ownership and parser-buffer changes;
it ran no tests and did not qualify native runtimes. Source-size, all 459
maintained Markdown files and strict existing-change validation passed.
Linux/process follow-up results are recorded in the final checkpoint below.
The explicit Linux `/opt` fixture provisioner and its full factory test
are runnable but unexecuted; they require administrator provisioning and must
run the service as its named ordinary user. These procedures do not establish
real ONNX/model/offline/semantic or hardware qualification.

Readback found that grouping by vector count alone rejected valid inputs whose
combined JSON exceeded the worker's 768 KiB prompt cap. An injected-runtime
regression reproduced the failure (0.716 s). Batches now account for exact JSON
encoding, including sixfold escaped characters, without truncating or reordering
inputs. Both large-input batching and oversized-single-input rejection checks
passed in 1.151 s. This fixture checks framing, not tokenizer/model semantics.

Public-route follow-up adds a portable multipart-import and V1/V2 catalog test;
it exercises metadata routing without requiring a runtime. UI readback also
found that V1 ONNX projections were offered legacy-file verification/removal.
The Models embedding category now reuses the typed package panel, retains GGUF
file controls, matches import categories explicitly and makes first ONNX import
available before any package exists. Successful package mutations refresh global
model selection. The API drift check initially passed, but TypeScript then found
that the package architecture enum still omitted `onnx-embedding`. The schema and
generated types now include that actual server-supported architecture; no cast
bypass was used. Regeneration, API drift check and production type/build passed.
The new package category/import/verify/remove/non-administrator tests plus
existing model-library/display checks passed **18/18 in 57.2 s**. The retained
ONNX package screenshot was inspected: controls and unqualified status are
visible without a legacy mutation card. This is rendered fixture evidence;
no additional readiness claim is inferred from importing package metadata.

The analogous managed-Ollama inventory does not independently prove that a new
DLL cannot appear after enumeration. Its existing explicitly trusted single-user
host contract must not be relabeled as immutable arbitrary-runtime sandboxing;
final runtime packaging/loader authority remains open in task 8.4.

Source-size ratchet and strict existing-change validation passed during this
increment. Full supported-platform runtime/model/semantic retrieval/offline/
performance/soak/frozen workflow qualifications remain open. No model weights,
dependency installations or existing Knowledge indexes were changed.

The older full inference/race results above predate ONNX integration; the latest
normal/race checkpoint follows below. A mistaken
`scripts/quality/check-source-size.mjs` invocation failed
with module-not-found; the actual `scripts/maintenance/check-source-size.mjs`
subsequently passed. No command-launch failure is counted as application proof.

## Final normal-suite checkpoint

With the final Windows import-namespace policy, Linux source integration and
byte-aware batching present, full Windows inference passed **89.101 s**, config
**2.381 s**, app composition **4.022 s** and API contracts **2.063 s**. The same
command failed the models package because its generic supported-layout fixture
still generated speech metadata and arbitrary filenames for the new ONNX layout.
The fixture now supplies the required CPU/BERT fields and exact artifact names;
validation and assertions were unchanged. Full models then passed **7.341 s**.

Focused server ONNX checks passed **0.928 s**, including actual multipart package
import, truthful V1/V2 catalogs and reserved-selector collision refusal through
public completion/embedding/preparation/agent/delete handlers. The final
provisioned Windows API/Knowledge journey explicitly skipped because its
administrator fixture was not supplied. Neither the full normal suite nor these
public metadata checks close that prerequisite.

A recovery review found that hashing absolute runtime/state/launcher paths into
the ONNX identity falsely invalidated a byte-identical relocated installation.
The pure regression failed in **0.333 s** before the correction and passed in
**0.381 s** afterward. Identity now excludes only those host locations; it keeps
the descriptor, relative executable/worker selection, runtime-file/dependency
pins, schema/runner and launcher digest. Seven changed-contract cases still
require a different identity. Every actual load still rechecks filesystem and
process authority at its selected location. This pure test does not substitute
for a provisioned runtime plus persisted Knowledge restore journey.

The final full Linux inference race suite passed **26.671 s**, using
`GOMAXPROCS=4 CGO_ENABLED=1 go test -race -p 1 ./internal/inference -count=1`.
It includes failed-load/admission poisoning, process/namespace teardown,
pre-start pipe-FD cleanup, settings FIFO/mutation rejection, digest contracts,
batching and relocation. Focused Linux model/server ONNX races passed
**1.029 s / 1.110 s**. A separate verbose check confirmed that the actual ONNX
protocol-through-owned-namespace fixture executed rather than kernel-skipped:
**1.128 s**, test body **0.09 s**, with retained pidfd checks for the parent and
`setsid` descendants after Close. This low-level lifecycle fixture uses a test
worker; it does not qualify the final administrator-provisioned factory or ONNX.

Translation checks, source-size ratchet, maintained-doc checks (470 files) and
strict OpenSpec validation also passed. No tests are running at this checkpoint.
The original transformation requirement remains partial: both final provisioned
runtime factories, actual model/tokenizer/semantic retrieval and resource/offline/
platform/frozen-workflow evidence, ordinary installed packaging and macOS
ownership remain required. Tasks 9.2–9.4 and tracker M5.05 remain open.

## Composed service and renderer metadata journey

The final `onnx-package-service.spec.ts` test ran against a freshly built native
service and the built renderer using the existing `test-web-workspace.mjs`
wrapper: **1/1 passed in 2.3 s**, test body **1.7 s**. The wrapper used an
ephemeral port, fresh disposable data/models/bin roots and its own process;
installation runtime configuration was not inherited. No API route was mocked.

The UI imported a complete manifest and three hashed synthetic artifacts,
observed one package-owned V2 embedding entry and the exact V1 selector,
confirmed runtime readiness/qualification were false and Knowledge stayed
disabled, verified through the package route, removed through the same exact
ID/revision and observed the selector disappear. Server logs confirm successful
verify/remove; the final defensive cleanup's repeated remove returned the
expected not-found response after removal. No worker or real model ran.

The actual service screenshot was inspected and retained with its service log
under `build/onnx-package-service-20261120/library.png` and `service.log`.
This closes the metadata lifecycle composition gap between earlier isolated UI
and handler tests. It does not close the provisioned/native runtime, installed
desktop, real-model, semantic retrieval or broader platform qualifications.

## Primary sources

- [ONNX Runtime Python sessions and execution providers](https://onnxruntime.ai/docs/api/python/api_summary.html)
- [ONNX Runtime explicit fallback control](https://onnxruntime.ai/docs/api/python/modules/onnxruntime/capi/onnxruntime_inference_collection.html)
- [ONNX external tensor data](https://onnx.ai/onnx/api/external_data_helper.html)
- [Tokenizers encoding, padding and attention masks](https://www.huggingface.co/docs/tokenizers/python/latest/api/reference.html)
- [CPython path initialization and ._pth precedence](https://github.com/python/cpython/blob/main/Modules/getpath.py)
- [Windows opened-file sharing modes](https://learn.microsoft.com/en-us/windows/desktop/FileIO/creating-and-opening-files)
- [Windows file and directory access rights](https://learn.microsoft.com/en-us/windows/win32/wmisdk/file-and-directory-access-rights-constants)
- [Windows access masks and ownership authority](https://learn.microsoft.com/en-us/windows-hardware/drivers/ifs/access-mask)
