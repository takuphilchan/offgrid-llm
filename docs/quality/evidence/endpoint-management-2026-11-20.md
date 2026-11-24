# Endpoint administration foundation — 2026-11-20

Scope: existing `inference-provider-runtimes` tasks 8.4.1–8.4.3 and M5.04.
Full endpoint creation/update, owned vLLM, qualified tools and real-runtime/model
qualification remain open. This increment changes no default provider and uses
no installed-user state, model weights, real endpoint account or personal browser.

## Implemented boundary

- Existing startup owner-file routes and `ProviderRegistry` remain the only
  endpoint enrollment authority. New administration accepts provisioned identity
  and expected configuration/policy revisions, never URLs, credentials or launch
  paths. Current administrator permission is reread from the live user store;
  probing also requires that actor's current enrolled chat permission.
- A bounded service-owned subtractive policy requires explicit review after
  migration/missing state, stale identity or restore. Source edits/removal,
  credential loss and corruption fail closed. Actor/time and pinned configuration
  are stored with each decision. Full historical audit/create/update is still
  task 8.4.4, not silently excluded scope.
- Requests acquire cancelable policy leases. Durable disable and admission
  serialize under the same lock; later requests cannot acquire a lease, admitted
  transports/generations are canceled, and an earlier queue selection grants no
  later dispatch permission. Remote effects already dispatched are not undone.
  The whole generation retains cancellation even while its callback is blocked.
- The production composition acquires the existing OS workspace ownership lock
  in `internal/app/core.go` before installing routes in `composition.go` and
  keeps it through cleanup barriers. The policy mutex assumes that enforced
  single service writer; it is not an interprocess CAS or hostile same-user
  filesystem sandbox. Existing ownership subprocess/crash tests exercise that
  prerequisite.
- New `storage.WriteDurableJSON` retains private temporary-file creation,
  syncs content and publishes with Unix parent-directory sync or Windows
  replacement/write-through. Publication errors may occur after rename; policy
  uncertainty cancels all admitted leases and blocks reuse for this service.
  Original `WriteJSON` behavior is unchanged. Cleanup retains original identity
  and disarms after publication so a recreated temp path is not removed.
- Restore verifies historical archive data, then replaces any historical epoch
  with 32 random bytes encoded as 64 lowercase hex characters through the retained
  stage root, syncs and publishes only a new target. The returned manifest still
  describes the original verified archive. Subsequent backup records the actual
  new marker and policy; old enabled grants require review against the new epoch.
  The first explicit review also creates a nonempty initial activation marker;
  status reads create none. Review found that treating absent markers as an empty
  original epoch could reactivate old grants after marker loss; existing policy
  now requires a present valid marker and never silently repairs it.
- Endpoint config/policy opens are bounded and identity checked; Unix uses
  nonblocking/no-follow open so a regular-to-FIFO substitution cannot hang before
  fstat. Windows uses a read handle denying writes/deletion while observed.
  A red management regression reproduced embedded credentials in a changed
  owner's URL being displayed after startup (**0.387 s**). File reads now reuse
  the transport's pure exact-origin parser, rejecting userinfo/query/path/fragment
  before metadata can be returned. Raw secret-bearing URLs are never echoed.
- Models renders endpoint-specific review, enable/disable, connection checks and
  ordinary-user selection. No endpoint card invokes legacy file maintenance.
  Probe metadata success never changes qualification. Unknown response shapes
  fail the panel closed without crashing unrelated Models flows.

## Actual checks so far

- Restore-epoch regression first reproduced missing/reused historical markers
  and missing pre-publication boundary, then passed **3.614 s** after integration.
  Historical absent/valid/malformed epochs, unchanged source, repeated restore,
  next-backup digest inventory and failure-before-publication are covered.
- Full Windows storage/app/CLI/RAG after durable publication: **6.527 / 1.984 /
  5.724 (`cmd/offgrid`) / 1.152 (`internal/cli`) / 14.681 s**, all passed.
  Earlier full pre-durable checks also passed; they are not substituted for the
  later result. Private durable snapshot failure/cleanup checks passed **2.428 s**.
- Endpoint service subset passed **1.409 s** including before/after-publication
  injected failures, cancellation, sticky uncertainty, migration, restart,
  restored old enabled policy, corrupt policy/marker, stale revisions and live
  administrator/actor revocation. Earlier actual-loopback HTTP test passed:
  receiver observed exactly one dispatched generation, it terminated after
  disable, and three later attempts failed without outbound generation.
- API generation/check, TypeScript and production web build passed. Existing
  Vite native-loader/static startup-script/chunk-size warnings remain.
- Rendered endpoint + ONNX regression **7/7 passed, 11.3 s**. Initial run was
  **5/7**: missing endpoint response fields crashed ModelsPage, corrected with
  bounded response validation; one test expected `Select` instead of the existing
  `Select model` label, corrected without changing product behavior. The ONNX
  import/verify/remove regression remained intact.
- Actual isolated service plus built renderer **1/1 passed, 2.8 s** (test body
  2.2 s): review/enable, unavailable credential rejection before generation,
  disable, persisted policy, owned service restart and changed-owner-config
  restart/review rejection. No mocked service responses or model activation.
  Disposable workspace: `C:\Users\phil\AppData\Local\Temp\offgrid-web-qualification-hqXqDh`.
  Screenshot/log retained under `build/endpoint-management-20261120/`.
- Initial development command failures are retained: guessed storage opener
  symbol caused a compile failure before tests (corrected to existing
  `openBackupSourceFile`); focused old fixtures lacked required isolated DataDir
  and explicit review (updated through the real administration flow); a PowerShell
  wildcard formatting invocation and an npm invocation from repository root
  failed before their respective checks and were rerun correctly.

Full Windows inference including blocked-callback policy cancellation passed
**88.208 s**; full Linux inference/storage race passed **23.850 / 5.818 s** and
focused endpoint server race passed **4.721 s** before the final origin/initial-
epoch review fixes. A buffered-terminal cancellation regression also passed
**0.347 s**: the existing terminal gate already withheld success/tool delivery,
so no unnecessary production stream guard was added. Final checks for the origin,
initial-epoch and strengthened queued/probe scenarios follow this checkpoint.

The 19 new messages now cover nine base and 20 preview languages; structural
coverage is **1639/1639**, no missing catalog copy. The first supplemental check
correctly rejected untracked new source references; references were registered
without modifying any review approval. Native linguistic/usability qualification
remains pending. Python's Windows app alias was unavailable during the additive
translation merge; the same bounded JSON merge ran through Node, with no install.

### Final foundation review checks (2026-11-21 local time)

- Full Windows storage / `cmd/offgrid` / `internal/cli` / RAG passed
  **8.255 / 5.494 / 1.274 / 15.049 s**, including the initial nonempty activation
  marker and missing-marker anti-rollback regressions.
- The stronger actual scheduler queue fixture initially failed **7.283 s** for
  the server subset: direct service calls omitted the foreground scheduling
  context that HTTP admission normally supplies, so the queue correctly counted
  them as background work. The fixture now supplies that context and still
  requires an observed queued request, cancellation and exactly one receiver
  generation. No scheduler behavior or deadline was weakened.
- Corrected endpoint server subset passed **2.143 s**, managed enrollment subset
  **0.640 s**, HTTP inference subset **10.329 s**. The endpoint set includes
  metadata-only probe success, queued/admitted cancellation and rejection of
  changed owner origins with embedded credentials, query, path or fragment.
- API check, TypeScript, production build and full translation structure checks
  passed. Endpoint renderer **5/5 passed, 9.9 s**, ONNX package regression
  **3/3 passed, 5.1 s**. English, Arabic RTL and Chinese preview screenshots were
  inspected and retained in `build/endpoint-management-20261120/`; controls are
  readable without horizontal overflow. This is layout evidence, not native
  translation approval. An initial command named a nonexistent ONNX pattern and
  selected only the five endpoint tests; the real ONNX file was then run explicitly.
- Early owner-workflow helper tests are additional, incomplete task 8.4.4 work.
  Its Windows writer positive fixture first rejected this host's TEMP ancestry,
  which grants Modify/DELETE to additional sandbox accounts (**2.333 s** server
  subset). The positive fixture now creates only its own protected directory
  under verified LocalAppData ancestry; no existing ACL was changed. The full
  owner mutation/API/renderer workflow remains open.
- Final Linux races passed: full storage **5.951 s**, endpoint server subset
  **4.888 s**, HTTP inference subset **5.704 s**. A freshly rebuilt owned service
  plus production renderer repeated the actual administration journey:
  **1/1 passed, 3.5 s** (body **2.9 s**), isolated workspace
  `C:\Users\phil\AppData\Local\Temp\offgrid-web-qualification-rFIrZ2`.
  Final screenshot/log remain alongside the earlier checkpoint. A docs/source
  checker invocation initially used the wrong script directory and ran no
  checks; the actual `scripts/maintenance` entry points were then used.

## Required remaining evidence

Complete owner create/update/audit/activation workflow, final administrator-
provisioned local factory journeys, real Ollama/vLLM model/tool/stream/cancel,
offline/resource/platform/frozen workflow qualification and usability remain
required. Metadata fixtures and UI success do not establish those outcomes.
