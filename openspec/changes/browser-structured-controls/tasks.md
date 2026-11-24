# Tasks

## 1. Search and modern controls

- [x] 1.1 Add literal role/name/text discovery with bounded scans, explicit coverage, exact references and query-bound continuation; verify Unicode, hidden controls, ambiguity, stale cursors and reply bounds in real Chromium fixtures, and document the contract in apps/computer/README.md.
- [x] 1.2 Extend open-shadow target/form freshness, root-local labels, contenteditable fill/read-back and supported custom checkbox/switch semantics; verify silent edits, host replacement, multiline Unicode, dynamic combobox options, sensitive fields and uncertain post-input results using `node --test apps/computer/test/*.test.mjs`.

## 2. Frame ownership and privacy

- [x] 2.1 Implement bounded frame inventory and explicit selection with tab/frame/document binding, nested approved-origin checks and no fallback on detach; verify duplicate labels, cross-origin scope, same-address reload, parent replacement and unavailable opaque frames in real Chromium tests.
- [x] 2.2 Make capture/read provenance and protected-field masking cover nested frames and open shadow roots, with whole-frame masking or explicit failure where necessary; verify captured pixels and transport limits, and document supported/unsupported structures.

## 3. Service integration

- [x] 3.1 Negotiate structured tools through existing session/hub/service capability contracts without widening legacy clients; test direct and prepared dispatch, actor/scope binding and durable read/effect identities with `go test ./internal/computer ./internal/server ./internal/agents` and companion session tests.
- [x] 3.2 Run applicable full companion, desktop, Go/race, web contract/check/build, docs/source checks and strict OpenSpec validation; record exact outcomes and maintain tracker M2 with attached-browser, visual, file and real-model qualification still open.
