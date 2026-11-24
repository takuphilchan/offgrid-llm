# Tasks

All tasks implement roadmap M2.07 through the existing browser/session owners.
Use only disposable files, profiles and loopback fixture services. No selected
personal file, live browser, account, model download or installation is authorized.

## 1. Exact upload content and bounded preparation

- [ ] 1.1 Implement a cancellable regular-file snapshot primitive with exclusive private staging, source identity/size/digest validation, bounded reads and owned cleanup. Reproduce source substitution and special-file failures, then verify exact captured bytes, pre-dispatch cancellation, oversize rejection and cleanup failure with `node --test apps/computer/test/browser-file-snapshot.test.mjs`; document the trusted local filesystem boundary in apps/computer/README.md.
- [ ] 1.2 Integrate snapshots into trusted desktop selection and companion upload, preserving the 512 MiB contract and exact observed target. Verify actual Chromium/HTTP input bytes for ordinary and >50 MiB files, source changes immediately before browser assignment, stale/foreign frame targets, manual grant invalidation and Stop; use `node --test apps/computer/test/browser-file-transfer.test.mjs` and focused desktop tests. Record selected-versus-remotely-received evidence and update tool/user descriptions.

Protected-storage recovery prerequisite for 1.1/1.2: shared host reservations must
be inspectable without mutation, and unresolved ownership must remain explicit.
The native diagnostic increment does not authorize resetting the registry or
retiring a reservation without verified owner cleanup.

## 2. Owned download lifecycle and provenance

- [ ] 2.1 Implement and verify the managed Chromium download monitor with real loopback streamed responses: exact initiating page/frame/action, native progress/cancellation, unsolicited/ambiguous downloads, per-file/aggregate limits, Stop and confirmed listener/process cleanup. Use `node --test apps/computer/test/browser-download-limits.test.mjs`; retain failing probe evidence and do not claim pre-completion bounds from completed-file checks.
- [ ] 2.2 Publish complete downloads exclusively with durable opaque artifact manifests and independently checked bytes, detected/declared type and bounded redacted source/time/document provenance. Verify duplicate names, occupied paths, symlink/reparse/special files, disk/write/publication failures and retained partial ownership using `node --test apps/computer/test/browser-file-transfer.test.mjs`; document lifecycle and recovery in apps/computer/README.md.

## 3. Canonical evidence and explicit artifact access

- [ ] 3.1 Add negotiated transfer-evidence support through existing hub/session/receipt/tool contracts, preserving legacy projections without upgraded claims. Test wrong actor/task/session, stale manifest, forged source/type, lost acknowledgement, restart and renewed pairing with `go test ./internal/computer ./internal/server ./internal/agents`, focused Linux race cases and companion journal tests. Update API contracts and run `npm --prefix apps/web run api:check` where schema changes apply.
- [ ] 3.2 Add trusted host artifact inspection/export and task presentation using existing artifact surfaces with correct local/service ownership. Verify native picker intent, exclusive destination, tamper rejection, no automatic opening, remote-service separation, keyboard/narrow/RTL/all-locale behavior and offline Stop with desktop and targeted Playwright suites. Update computer-task guides and list installed picker/human checks separately.

## 4. Integrated acceptance

- [ ] 4.1 Run a composed service/host/companion/Chromium transfer journey with independent HTTP/file digest oracles, exact local artifact inspection/export, cancellation and lost replies. Run complete companion/desktop suites, affected full/race Go checks, web type/build/locale checks and independent ownership review. Record actual failures and limits in dated evidence; fixtures do not establish model or installed-platform qualification.
- [ ] 4.2 Reconcile every scenario with implementation/tests/evidence and master M2.07; run `node scripts/maintenance/check-docs.mjs`, `node scripts/maintenance/check-source-size.mjs`, `git diff --check` and `openspec validate browser-file-transfers --strict --no-interactive`. Keep unfinished platform, real-model/site, performance and human gates open with runnable procedures.
