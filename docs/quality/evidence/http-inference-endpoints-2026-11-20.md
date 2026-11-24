# HTTP inference endpoint implementation — 2026-11-20

Status: implementation and isolated contract tests. This record does not qualify
a real Ollama/vLLM installation, a hardware profile or an agent model.

## Implemented boundaries

- Explicit `ollama`/`vllm` provider descriptors and factories use the existing
  provider registry, scheduler and terminal/tool event gate.
- Ollama NDJSON and vLLM SSE preserve streaming text, usage, terminal reason and
  validated complete tool decisions. Truncation, malformed/ambiguous JSON,
  excessive output, changed identity and missing budget evidence fail closed.
- Request cancellation and duplicate request IDs cannot replay generation.
  Close releases an owned local lifecycle lease only after active calls end;
  remote close makes no local model-memory claim.
- Exact origins, pinned dial IPs, normal TLS verification, disabled proxies and
  blocked redirects prevent destination expansion. Per-request authority obtains
  the scoped credential separately from model metadata.
  Current local interface addresses are rejected for remote resource enrollment
  and dialing. Operator attestation still owns placement behind NAT/forwarders;
  a non-loopback URL alone is not physical remote-resource evidence.
- Ollama version/model digests and vLLM version/served names are checked. vLLM
  weight identity remains an explicit deployment-owner attestation; its model
  list is not a cryptographic digest. Ollama forwarded/cloud aliases are rejected.
- Service startup loads an opt-in remote enrollment file without network access.
  Actor/pin/credential checks repeat at selection and each outbound request.
  Existing text chat/API and text-only agents use explicit `endpoint:id@revision`
  identities. Model lists expose only currently authorized enrollments and do
  not mark them runtime-ready, smoke-tested or qualified.
- The reserved endpoint namespace takes precedence over legacy local registry
  collisions. Missing/revoked endpoint access cannot reuse local chat, tokenizer,
  projector, cached lease or local-policy qualification. Local-only APIs reject
  that identity and preserve the colliding local record and files.

## Primary-source findings

Reviewed 2026-11-20: [Ollama chat](https://docs.ollama.com/api/chat) specifies
NDJSON streaming controls and terminal evaluation counts;
[model listing](https://docs.ollama.com/api/tags) exposes the manifest digest.
[Ollama cloud](https://docs.ollama.com/cloud) confirms a local API may route to
cloud inference. The [official response types](https://github.com/ollama/ollama/blob/main/api/types.go)
expose `remote_host` and `remote_model`, which this adapter rejects rather than
mistaking loopback access for local execution.

[vLLM serving](https://docs.vllm.ai/en/latest/serving/openai_compatible_server/)
uses OpenAI-compatible chat streams and requires suitable chat templates; tool
behavior is model-dependent. Transport support does not establish model quality.

## Verification

- Focused Go inference/service/config checks passed for endpoint wire handling,
  scheduler integration, truncation, cancellation, duplicate IDs, pin/actor/file
  revocation, missing credentials, malformed configuration and the existing HTTP
  text routes. Tests use isolated in-process servers and temporary files.
- Strict OpenSpec validation and source-size ratchet passed.
- `go test ./internal/inference ./internal/server ./internal/config ./internal/app -count=1`
  passed: 27.421 s / 278.600 s / 1.745 s / 5.580 s. Log:
  `build/http-provider-checks.log`.
- Focused Linux `CGO_ENABLED=1 go test -race ./internal/inference ./internal/server -run HTTP -count=1`
  initially passed. After review, a byte-limit reader was corrected to report
  overflow rather than artificial EOF. NDJSON/SSE regressions with >32 MiB of
  trailing data after a valid terminal both passed. The final inference race
  checks passed (9.726 s); that invocation's dependency vet encountered an
  incomplete concurrent agent edit, so the command was not a complete pass.
  Once the dependency edit completed, `CGO_ENABLED=1 go test -race ./internal/server -run Endpoint -count=1`
  passed (1.576 s), including canonical endpoint text-profile execution. Logs:
  `build/http-provider-race-final.log` and
  `build/http-provider-service-race-recheck.log`.
- The first attempted Linux race command failed in shell argument quoting and
  executed no tests; it was corrected without changing application behavior.
- Configuration environment/file precedence and persistence passed in the
  focused Go config test. Documentation checked 428 maintained Markdown files
  with zero local-link problems; structural validation is not a runtime test.
- Generated API types, `npm.cmd --prefix apps/web run api:check` and
  `npm.cmd --prefix apps/web run check` passed after adding endpoint engines to
  the model-list schema. Installed package engine enums were not broadened.
- The local-interface remote-placement regression and configured enrollment
  tests passed after the additional resource-domain check. This check cannot
  identify a same-host service hidden behind NAT; the explicit operator
  deployment attestation remains necessary.

- Review found and fixed a local-registry-first collision in chat routing.
  `go test ./internal/server -run Endpoint -count=1` passed (1.587 s) after the fix.
  Real imported local registry fixtures cover both HTTP chat modes, durable chat,
  canonical profile/job revocation, malformed endpoint IDs, local completions,
  embeddings, preparation, legacy agent runs, computer qualification, deletion,
  cached local leases, tokenizer exclusion, local model policies and model-list
  visibility. An initial compile attempt used an incorrect new test type name;
  that test typo was corrected before the passing run. The full combined-suite
  checkpoint above predates this final routing correction; subsequent aggregate
  runs are recorded by the program integration evidence.
- No Anthropic `count_tokens` endpoint is implemented in the current API. The
  existing agent context counter explicitly returns unavailable for endpoint
  identities, including older tasks without the text-only flag; it never counts
  a remote prompt with the local llama tokenizer. Provider-correct Anthropic
  token counting remains part of the separate API implementation requirement.
- Authenticated endpoint review found that async jobs intentionally discard
  HTTP request context, while endpoint selection had depended on that context.
  Canonical text jobs and schedule admission now use their persisted actor ID
  through a private server context. Each endpoint selection/request checks the
  live user store under its lock; removed/disabled actors cannot retain grants
  via mutable request user pointers or become anonymous owners after auth is
  disabled. `go test ./internal/server -run Endpoint -count=1` passed (2.703 s),
  and Linux `CGO_ENABLED=1 go test -race ./internal/server -run Endpoint -count=1`
  passed (4.638 s). Tests authenticate through real middleware and exercise a
  canonical background job with the production enrollment selector and an
  isolated provider fixture. The first schedule assertion incorrectly supplied
  an interactive config; it was corrected to run the actual scheduled config
  normalizer. A retry encountered incomplete concurrent storage edits at compile
  time; the passing retry followed completion of those dependency edits.

## Remaining work

At this remote-adapter checkpoint, owned local Ollama/vLLM supervision, tool-qualified
service agent routing, structured-output/multimodal qualification, endpoint
administration/readiness UI, real runtime/hardware trials and frozen model
workflows remain open. Existing MLX/Transformers platform gates are unchanged.
No live endpoint, personal data, model weight download, installation or release
was used to obtain this evidence.

The subsequent [Windows process ownership increment](provider-process-ownership-2026-11-20.md)
implements one local-supervision prerequisite in the existing native worker
lifecycle. That prerequisite alone does not open the local HTTP enrollment gate.

The subsequent [managed Windows Ollama increment](managed-ollama-windows-2026-11-20.md)
adds opt-in file enrollment and owned local text execution on explicitly
acknowledged trusted single-user Windows hosts. It also records the reproduced
typed-nil factory and uncertain-start admission defects and their fixes. Local
vLLM, POSIX containment, authenticated multi-user runtime isolation, additional
model layouts, tool qualification and real provider evidence remain open.

Usage and configuration: [enrolled HTTP providers](../../advanced/http-inference-endpoints.md).
