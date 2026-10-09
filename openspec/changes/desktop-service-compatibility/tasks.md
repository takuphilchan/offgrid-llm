# Tasks

Implementation and local verification are complete; platform-runner limitations and test evidence are recorded in docs/advanced/product-reliability-plan.md (2026-10-09). Checked tasks do not imply cross-platform release qualification. Commands below run from the repository root unless a working directory is stated. Use isolated fixtures, not the live container or installed application.

## 1. Renderer and service compatibility contract

- [x] 1.1 Define bridge protocol 1 from the current preload/main interface and generate bounded, index-bound `desktop-compatibility.json` in the existing renderer build. Add tests for deterministic generation, CRLF normalization, missing fields, oversized metadata, and mismatched fingerprints; verify `npm --prefix web/app run build` produces matching metadata and test assertions pass.
- [x] 1.2 Add the optional `desktop_bridge` identity projection in `internal/server/system_identity.go` with distinct missing/invalid/ready states, preserving old fields and uncached public access. Add Go fixtures for valid manifests, custom UI roots, missing/corrupt metadata, unsupported schema, and private-data exclusion; verify `go test ./internal/server -run 'Test.*(SystemIdentity|DesktopBridge|BuildRevision)' -count=1` passes.
- [x] 1.3 Update OpenAPI/generated types and `docs/advanced/client-contracts.md` with metadata semantics, bridge evolution, and legacy handling. Run `npm --prefix web/app run api:generate`, `npm --prefix web/app run api:check`, and `npm --prefix web/app run check`; all must pass with no schema drift.
- [x] 1.4 Include and validate compatibility metadata in existing desktop, container, and reused-runtime packaging paths. Add packaging regression assertions that stale/missing metadata cannot be presented as a matching new package; verify the existing package checks and the newly added fixture checks pass without replacing a live container.

## 2. Attachment policy, legacy transition, and trust

- [x] 2.1 Add explicit external/owned assessment contexts in `desktop/backend.js` and `runtime.js`, structured reason codes, and diagnostic decision basis. Test different versions/renderer hashes with supported contracts, unknown protocols, missing capabilities, invalid metadata, exact owned-package mismatches, cancellation, hostile responses, and single-flight retry; verify `npm --prefix desktop test` passes.
- [x] 2.2 Review the reported 0.4.14 and corresponding 0.4.15 renderer/preload contracts and add only evidenced exact legacy tuples with fixture/source references. Test that a later desktop renderer fingerprint still accepts those tuples, unknown tuples fail, and invalid new metadata cannot use fallback; verify desktop tests pass and document the reviewed mapping in `docs/advanced/client-contracts.md`.
- [x] 2.3 Preserve trusted-main-frame and startup-only IPC guards while binding workspace bridge admission to the accepted attachment. Add injected service-change tests for reassessment and invalidation of old computer authority, without sending native input. Run `npm --prefix desktop test` and verify compatible attachment cannot bypass authentication, consent, or native protocol checks.
- [x] 2.4 Update `desktop/README.md` to distinguish contract-compatible external attachment from exact bundled consistency and describe protocol review rules. Verify the documentation against the passing policy fixtures and retain the owned-process/no-external-shutdown contract.

## 3. Startup, Settings, and installer recovery

- [x] 3.1 Render reason-specific startup copy in all nine locales, remove duplicated guidance, and retain monochrome styling, focus, and screen-reader status. Add assertions for external-service update guidance, owned-package repair, unrelated listeners, and timeout; verify `npm --prefix desktop test` and startup fixture assertions pass without hiding technical diagnostics.
- [x] 3.2 Keep Settings component versions and ownership accurate and add compatibility basis in the existing diagnostics group without a new configuration panel. Update relevant renderer fixtures; run `npm --prefix web/app run check` and `npm --prefix web/app run test:e2e` and verify there is no claim that installing desktop upgraded external UI.
- [x] 3.3 Change Windows completion copy to application installation, preserving graceful shutdown and unprivileged launch after the wizard closes. Extend isolated installer assertions to cover an older compatible external fixture remaining alive with unchanged fixture data; run the separate installer suite described in section 4 and verify it never touches the production product registration.
- [x] 3.4 Update `docs/setup/desktop-startup.md` with the new compatibility/recovery decisions, limits for unreviewed older releases, and the fact that the installer does not upgrade Docker. Cross-check every recommended action against actual startup behavior; remove obsolete blanket claims that any different release number must be blocked.

## 4. Integrated regression and evidence

- [x] 4.1 Extend `dev/scripts/test-desktop-startup.mjs` to exercise supported cross-version attachment, the fixed legacy tuple with a changed new-package renderer, incompatible contract recovery, owned-package mismatch, retries, relaunch, and service survival. Build the isolated Windows package and run the commands below; require real main/preload startup assertions and unchanged fixture workspace data.
- [x] 4.2 Build and run the isolated Windows installer test package using the commands below. Verify clean install, reinstall, uninstall, Finish behavior, the legacy external-service scenario, and data preservation. Retain screenshots/logs from the test-only profile; do not use a release installer against the user's application.
- [x] 4.3 Run the applicable existing CI suite: `go test ./...`, the race command below, API drift/type/build, renderer tests, desktop unit tests, and packaged startup/browser/theme checks. Run the shared startup fixtures on Linux, macOS Intel, and macOS Apple Silicon through the existing matrix when that execution is separately available; record unavailable checks explicitly and do not claim full cross-platform qualification.
- [x] 4.4 Record actual commands, fixture identities, results, and remaining limitations in `docs/advanced/product-reliability-plan.md`; run `openspec validate desktop-service-compatibility --strict --no-interactive` and `git diff --check`. Confirm only intended changes are present. Keep commits, pushes, publication, live app/container replacement, and real native-control testing outside this implementation unless separately requested.

### Windows integration commands

Use existing dependencies/approved dependency setup and isolated build/test paths. Build with the package's actual version, not an invented release number:

```powershell
npm --prefix web/app run build
$desktopTestVersion = (Get-Content desktop/package.json | ConvertFrom-Json).version
go build -trimpath -ldflags "-X main.Version=v$desktopTestVersion" -o build/windows/offgrid.exe ./cmd/offgrid
Push-Location desktop
try {
  npx electron-builder --dir --win --x64 --publish never
  node ../dev/scripts/test-desktop-startup.mjs 'dist/win-unpacked/OffGrid LLM Desktop.exe'
  node ../dev/scripts/test-packaged-browser.mjs 'dist/win-unpacked/OffGrid LLM Desktop.exe'
  node ../dev/scripts/test-desktop-theme.mjs 'dist/win-unpacked/OffGrid LLM Desktop.exe'
  npx electron-builder --config installer-test.cjs --win nsis --x64 --publish never
} finally { Pop-Location }
./dev/scripts/test-windows-installer-probe.ps1
./dev/scripts/test-windows-installer.ps1 -InstallerPath "build/windows-installer-smoke/OffGrid Desktop Install Test-Setup-$desktopTestVersion.exe"
```

Check every command's exit status; PowerShell's `try/finally` alone does not fail on native nonzero exit codes. The existing installer harness must continue refusing production installers and conflicting test registrations.

### Shared CI checks

```sh
go test ./...
go test -race ./internal/sessions ./internal/rag ./internal/server ./internal/users ./internal/cache ./internal/agents ./internal/runs ./internal/storage ./internal/serviceclient ./internal/models ./internal/audio
npm --prefix web/app run api:check
npm --prefix web/app run check
npm --prefix web/app run build
npm --prefix web/app run test:e2e
npm --prefix desktop test
```

Use the existing Linux/macOS package build and smoke commands from `.github/workflows/ci.yml`; Linux packaged tests require Xvfb and macOS tests require their actual platform runners. A passing mocked identity test does not replace those checks. No automatic publication or live data migration is part of this task list.
