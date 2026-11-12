# Tasks

## 1. Installer integrity and ownership

- [x] 1.1 Fix root and legacy installer verification before extraction and process refusal; update guidance and run `bash scripts/maintenance/test-installer-entrypoints.sh` and `bash scripts/maintenance/test-executor-install.sh` with failure fixtures.
- [x] 1.2 Align desktop/archive helpers with full payload and process-safety rules; verify `node --test scripts/maintenance/desktop-installers.test.mjs` and generated archive fixture tests. Native Setup service/worker detection and inspection-failure handling passed native guard/refusal checks and the final installed upgrade, preserving the existing quit flow.

## 2. Complete runtime discovery

- [x] 2.1 Resolve bundled CLI UI/speech/inference/execution paths without global environment settings; add executable-relative discovery tests and run affected Go suites, update CLI setup guidance.

## 3. First-answer preparation

- [x] 3.1 Add bounded explicit runtime preparation/readiness with CPU/GPU status and download progress/recovery; verify Go API/manager tests and contract generation.
- [x] 3.2 Integrate preparation into existing onboarding, preserve first-response completion tracking and auth/workspace boundaries; verify web checks/build and isolated onboarding browser scenarios, update setup docs.

## 4. Release trust and truthful distribution

- [x] 4.1 Add actual artifact trust verification/status and local preview identity; test verifier success/failure fixtures and document signing credential requirements without claiming unsigned artifacts trusted.
- [x] 4.2 Correct CPU/platform/download/upgrade guidance and unsupported archive selection; run hardware-selection fixtures and documentation checks.
- [ ] 4.3 Verify final signed/notarized artifacts on supported release hosts with actual credentials; record evidence or unresolved external prerequisites.

## 5. Build and install

- [ ] 5.1 Run applicable CI suites including Go, web contract/check/build, desktop and isolated package/installer checks; build the uniquely identified preview and record actual evidence.
- [x] 5.2 Identify installed app/workspace, preserve state, install the verified new Windows build, observe installed identity/startup and a real model reply; record evidence and rollback information.
- [ ] 5.3 Audit all acceptance scenarios and cross-platform qualification against actual evidence; keep remaining external gates explicitly incomplete.

## 6. Authorized commit and release

- [x] 6.1 Review the final source changes and release identity, prepare accurate release notes, and create focused commits after the installed upgrade is verified; preserve unrelated work and record validation.
- [ ] 6.2 Push the reviewed release source, require passing CI for its exact revision, publish through the maintained release workflow with actual trust checks, and verify the public assets, checksums and release identity.
- [x] 6.3 Implement and verify the user-authorized unsigned-preview publication channel with explicit manual selection, truthful notes/reports, prerelease marking and no stable container alias promotion; retain signed-channel enforcement.
