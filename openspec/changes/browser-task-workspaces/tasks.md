# Tasks

## 1. Scoped network and managed tabs

- [x] 1.1 Enforce explicit bounded origin sets through route checks and pinned CONNECT destinations; test permitted second origin, denied third/private redirects, TLS and legacy behavior with `node --test apps/computer/test/network.test.mjs` and document the scope in apps/computer/README.md.
- [x] 1.2 Add stable task tab/document identities, eight-tab cap, serialized open/select/close/list and exact observation binding; test cross-tab stale references, reloads, popups, concurrent calls, close/capacity and uncertain navigation with `node --test apps/computer/test/browser-workspace.test.mjs`.

## 2. Consent and transport integration

- [x] 2.1 Negotiate workspace features and explicit scope in companion/Go hub and tool discovery, rejecting native mixing and unsupported requests; run `go test ./internal/computer ./internal/server ./internal/agents` and companion session tests, documenting contracts in apps/computer/README.md.
- [x] 2.2 Expose explicit origin selection through CLI and desktop consent with bounded validated IPC, accurate displayed sites and legacy fallback; run companion/desktop tests, API/type/build checks and update docs/guides/computer-tasks.md.

## 3. Integrated evidence

- [x] 3.1 Run applicable full Go/race, companion, desktop, web API/check/build, source/docs checks and strict OpenSpec validation; record exact evidence and remaining real-model/platform/browser requirements without claiming full M2 completion.
