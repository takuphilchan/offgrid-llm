# ONNX embeddings: installation contract and qualification

Status: partial implementation. CPU package/worker contracts and service routing
exist. Windows activation requires an explicitly provisioned read-only runtime;
the final provisioned factory journey is not qualified in this environment.
The restricted Linux factory similarly requires a root-provisioned runtime and
owned PID/user namespace; its full provisioned journey is unrun. macOS activation
is unavailable. Ordinary desktop runtime installation, real ONNX model quality
and supported-platform qualification remain open.
See the [evidence record](../quality/evidence/onnx-embeddings-2026-11-20.md).

## Package and selection

In Models, choose **Embeddings**, then **Advanced · offline import** to select
the package folder. Package Verify/Delete controls use the declared ID/revision;
GGUF embedding files keep their existing controls. Import through this existing
Models package upload or authenticated
`POST /api/v2/models/packages`. Multipart order is `manifest`, then each artifact
using its exact declared path in manifest order. Import checks all byte sizes
and SHA-256 hashes; it does not install Python or dependencies.

The initial package contract is:

| Field | Required value |
|---|---|
| `architecture`, runtime adapter | `onnx-embedding` |
| engine, format, backend | `onnx`, `onnx`, `cpu` |
| model family | `bert-symmetric` |
| adapter and runtime adapter revision | `bert-symmetric-v1` |
| runner revision | Exact revision of the explicitly installed worker |
| capabilities, modalities, tool calling | `["embeddings"]`, `["text"]`, `unsupported` |
| context length | Same as settings `max_tokens`, 1–8192 |
| artifacts | Exactly `model.onnx` (weights, <=512 MiB), `tokenizer.json` (tokenizer, <=16 MiB), `embedding.json` (config, <=64 KiB) |

The normal manifest schema/version, immutable ID/revision, license, languages and
artifact digests remain mandatory. Speech rates/voices, templates and extra
artifacts are unsupported. Graph external tensor data, custom operator domains,
automatic remote tokenizers and custom executable adapters are rejected.
The worker hashes the exact graph/tokenizer/settings byte buffers against the
service's manifest-bound pins before parsing; later changes to their paths do
not change those buffers.
Absolute runtime/state locations are checked at load but do not change embedding
identity when identical bytes move. Actual graph/tokenizer/settings, worker,
runtime/dependency revisions and relative executable selection remain bound;
changed contracts still require an explicit verified Knowledge rebuild.

`embedding.json` requires every field shown below; dimensions and token limits
must match the actual exported graph/tokenizer, not this illustrative example:

```json
{"schema_version":1,"family":"bert-symmetric","inputs":["input_ids","attention_mask"],"output":"last_hidden_state","pooling":"mean","normalize_l2":true,"dimensions":384,"max_tokens":512,"pad_token":"[PAD]","pad_token_id":0}
```

`token_type_ids` is the only optional third input. Inputs are rank-two int64;
output is float32 token embeddings for `mean`, `cls` or `last`, or rank-two
sentence embeddings for `none`. Padding is masked. Inputs are rejected on token
overflow; they are never silently truncated. Vector count, dimensions, finite
values, nonzero norm, normalization and terminal usage are validated before
publishing any result. Asymmetric query/document prefixes need another reviewed
adapter and are not supported by this family.

An installed package appears as `embedding:<id>@<revision>` in `/v1/models` and
Knowledge's embedding selection. Old model IDs are unchanged. Use that exact
selector in `/v1/embeddings`. Package metadata/digests remain the authority;
the prefix does not authorize execution. Chat and legacy model deletion cannot
fall back to a local file with the same selector. Use package removal instead.
Neither import nor enrollment changes the initial default Knowledge model.
An index built with a different model/runtime/tokenizer/settings identity is
refused until an explicit rebuild; retained source documents are preserved.

## Explicit Windows runtime enrollment

Set `ONNXEmbeddingRuntimeFile` via configuration key
`onnx_embedding_runtime_file` or environment variable
`OFFGRID_ONNX_EMBEDDING_RUNTIME_FILE` before starting the service. Its absolute
JSON path must be a bounded regular file. The JSON contains `schema_version: 1`,
`runner_revision`, absolute `runtime_directory` and separate `state_directory`,
relative `python`/`script` paths, `runtime_files` mapping **every** runtime file
to `sha256:<64 lowercase hex digits>`, and `dependency_versions` containing exact
installed versions of `onnxruntime`, `onnx`, `numpy` and `tokenizers`.

The runtime inventory is limited to 16,384 file pins, 20,000 entries, depth 32,
512 MiB per file and 4 GiB total. The interpreter, standard library, native
extensions, worker and dependencies must be within that inventory. Links,
hardlinks, added/missing files and changed hashes are rejected. The service never
downloads, installs, repairs or changes permissions of the runtime.

The runtime root/files/directories must have administrator/SYSTEM ownership and
ACLs granting the service read/execute access, with write/owner/ACL authority
reserved to trusted privileged principals. Ancestors must not allow ordinary
users to retarget the tree. The service must run non-elevated without enabled
privileged groups or available ownership/restore/debug/impersonation overrides.
State is a separate account-private writable directory with an exclusive lease.
Administrator/OS mutation is outside this boundary. This is process and import
namespace ownership, not a filesystem or network sandbox for arbitrary code.

Python uses `-I -B -S`, a pinned adjacent `._pth` file with only in-tree relative
search paths and no `import site`, and a fixed `Lib/site-packages` directory.
External virtual-environment bases are rejected. The worker checks its resolved
executable, prefixes and import paths, and verifies dependency versions before
loading the local graph. CPU memory is reported as unmeasured, never zero-cost;
this increment does not provide a whole-machine hard memory limit.

## Isolated provisioned fixture gate

The independent metadata-only service/renderer journey needs no native runtime
or weights. After building the web UI, run the existing isolated service wrapper
with a freshly built OffGrid executable:

```powershell
$onnxQualificationRoot = Join-Path $env:TEMP ('offgrid-onnx-check-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $onnxQualificationRoot | Out-Null
go build -o (Join-Path $onnxQualificationRoot 'offgrid.exe') ./cmd/offgrid
node scripts/test/test-web-workspace.mjs (Join-Path $onnxQualificationRoot 'offgrid.exe') tests/onnx-package-service.spec.ts --workers=1
```

That check verifies first import, both catalogs, package verification/removal and
no runtime/Knowledge auto-activation. Its synthetic artifacts are not ONNX models.

The procedure below is **not executed automatically** and does not use weights.
It creates a fresh administrator-owned fixture under Program Files. Run its
provisioning step only with explicit operator permission, then run tests from
the ordinary service-user shell. The test overrides writable state with a fresh
private temporary directory; it never uses a live service workspace.

```powershell
go build -o "$env:TEMP\offgrid-onnx-fixture.exe" ./internal/inference/testdata/onnxworker
[Security.Principal.WindowsIdentity]::GetCurrent().User.Value
# In an administrator shell, pass the ordinary user's SID printed above:
.\internal\inference\testdata\onnxworker\provision-windows-fixture.ps1 -FixtureExecutable "$env:TEMP\offgrid-onnx-fixture.exe" -ServiceUserSID 'S-1-5-21-...'
# Back in the ordinary shell, use the exact runtime.json path printed by provisioning:
$env:OFFGRID_ONNX_WINDOWS_FIXTURE_RUNTIME = 'C:\Program Files\OffGridONNXFixture.<issued-id>\runtime.json'
go test ./internal/inference -run '^TestONNXOwnedEmbeddingPackageSchedulingAndCancellation$' -count=1 -v
go test ./internal/server -run '^TestONNXPublicEmbeddingsAndKnowledgePreservePackageIdentity$' -count=1 -v
```

The fixture checks public routing, retained package/file ownership, cancellation
of live descendants and durable Knowledge identity rejection. Synthetic fixture
vectors do not measure ONNX accuracy or real runtime behavior.

## Restricted Linux enrollment and fixture

Linux runtime JSON also requires absolute `namespace_launcher` and its complete
`namespace_launcher_sha256` pin for the reviewed util-linux `unshare` binary.
The service must be non-root with no inherited/permitted/effective/ambient
capabilities. Runtime files/directories and their ancestors require root
ownership without group/other writes, links or hardlinks. A self-contained
Python distribution must resolve its prefixes/import paths inside that tree;
dependencies use `lib/python<major>.<minor>/site-packages`. External virtual
environment bases are unsupported. The state directory is owned privately by
the service under a root-owned non-writable parent; its advisory lease remains
cooperative, not protection from malicious same-UID processes. The package store
may stay service-owned because all actual model parser buffers are digest-bound.

The existing namespace monitor retains pidfds and verifies user/PID namespace
identity, including init-driven descendant teardown. This is process lifecycle
containment, not a filesystem/network sandbox. No cgroup, sysctl, ownership,
runtime installation or kernel policy is changed automatically. Missing kernel
or deployment prerequisites are explicit failures. Privileged administrator
mutation and trusted base OS components are outside the runtime-file boundary.

The following explicit provisioner creates a fresh fixture under `/opt`; it has
not been executed in this work. It creates no model weights and changes no live
installation. Run tests as the ordinary user, using only its newly issued path:

```sh
go build -o /tmp/offgrid-onnx-fixture ./internal/inference/testdata/onnxworker
sudo python3 internal/inference/testdata/onnxworker/provision-linux-fixture.py "$(id -u)" /tmp/offgrid-onnx-fixture
OFFGRID_ONNX_LINUX_FIXTURE_RUNTIME=/opt/offgrid-onnx-fixture-<issued-id>/runtime.json \
  CGO_ENABLED=1 go test -race ./internal/inference -run '^TestONNXLinuxProvisionedFactorySchedulingAndCancellation$' -count=1 -v
```

## Remaining real qualification

With an explicitly supplied, reviewed offline Python distribution and actual
ONNX package, record exact manifest/runtime/dependency digests and hardware.
Use isolated service/Knowledge state. Run cold/warm embedding calls and the
frozen retrieval corpus three times, recording counts/dimensions, token usage,
pooling/normalization, semantic rankings, supported/unsupported questions and
latency/peak committed memory. Compare against independently computed reference
vectors for the same tokenizer and graph. Exercise active cancellation, process
crash, malformed/truncated output, repeated retirement/reload, wrong digests and
token overflow. Reopen the persisted index, verify source-linked retrieval, then
change an identity and confirm refusal without silent rebuild.

Run again with external networking disabled and record attempted connections;
worker source flags alone are not offline evidence. Repeat on every advertised
CPU/platform profile. Provisioned Linux/Windows runtime ownership needs actual
qualification, and macOS ownership/ordinary installed packaging must be completed;
fixture or documentation results cannot
close those requirements or the real-model qualification gate.
