# Design

## Context

See `proposal.md` for the reported failure. Current code has three distinct boundaries:

- `internal/server/system_identity.go` reports the product/API/capabilities and the SHA-256 of newline-normalized `index.html`. It does not describe the preload contract. The index references content-hashed assets; this fingerprint is not a signature.
- `desktop/backend.js` requires release-version equality before renderer equality. `DesktopRuntime` uses that same check for an existing external service and its own bundled child. Only the latter is expected to be an exact package match.
- `desktop/main.js` loads the service's `/ui/`, not a separate packaged renderer, and exposes a limited, exact-main-frame preload bridge. Computer runtime admission separately checks workspace identity, capabilities, and consent.

The observed external container is `offgrid-llm:workspace-refresh-20261008`, version 0.4.14, API 2. Its index fingerprint is `a7336324e4ea09e8a9f5e25aabf2a53c2f9cb39b81eb990a081ba9a84e66f617`, matching the current 0.4.15 renderer build. This is diagnostic evidence of the rejected combination, not proof that all cross-version operations work.

`desktop/assets/installer.nsh` upgrades the application and gracefully closes its existing desktop process. It does not administer Docker. Its "OffGrid is ready" completion title overstates what has been checked. Startup then repeats a generic paragraph while hiding the actual mismatch reason under technical details.

Authoritative boundaries remain in `docs/advanced/client-contracts.md`, `docs/setup/desktop-startup.md`, and `desktop/README.md`. The outcome-experience change intentionally excludes this compatibility redesign.

## Goals / Non-Goals

**Goals:** Separate package consistency from protocol compatibility; support independent release cadence; make the reported legacy case a durable regression; retain single-owner startup and understandable recovery.

**Non-Goals:** No automatic container orchestration, self-update service, new database, remote attachment, broader IPC, or global promise that startup cannot fail. No changes to native protocol or action-approval semantics.

## Decisions

### 1. Publish the contract of the actual renderer

Generate `desktop-compatibility.json` alongside the built renderer, containing a manifest schema version, an integer bridge protocol, and its normalized index fingerprint. Define bridge protocol 1 from the existing `window.electron` surface and corresponding main-process handlers. Maintain the supported protocol set in the desktop application, independently of its package version. Breaking argument/result/permission semantics require a new protocol; retaining an older protocol requires explicit tests.

The Go identity handler reads bounded metadata from the same resolved UI root as the index. It validates the manifest schema and fingerprint before returning additive `desktop_bridge` metadata (status, schema version, protocol, and UI build ID). It returns a distinct invalid status when a present manifest is malformed or mismatched; it does not turn that failure into a legacy absence. Missing legacy metadata remains distinguishable. No local paths or private data are exposed.

The metadata describes compatibility, not authenticity. Keep existing loopback, authentication, exact-origin, main-frame, and installed-package verification boundaries. A metadata claim alone must never grant host authority.

Use the existing Vite packaging path and Go identity endpoint instead of a second discovery endpoint or runtime listener. Include the generated file in desktop/container/reused-runtime packs and validate it against the packaged index. Add the field to OpenAPI and generated client types. A custom or stale `OFFGRID_UI_DIR` cannot inherit the compatibility declaration of a different packaged renderer.

**Rejected:** Inferring bridge support from semver, API version 2 alone, or a Go release number. None describes the renderer's use of privileged desktop APIs.

### 2. Use separate owned and external policy inputs

Pass explicit attachment context from `DesktopRuntime` to the pure compatibility assessor:

- **External:** Require OffGrid product identity, supported API and required capabilities, identifiable UI, and a supported declared bridge protocol or reviewed legacy mapping. Release numbers and renderer differences are diagnostic only after these checks pass.
- **Owned:** Also require exact package version and packaged index fingerprint, plus valid matching bridge metadata. Never reinterpret an owned child's package mismatch as an external service.

Keep the existing bounded probe, response size limit, no redirects, request cancellation, single-flight connection, and owned-child cleanup. Do not spawn when a listener is incompatible or unresponsive. Explicit isolated recovery stays session-scoped and user-selected.

A ready snapshot includes the decision basis (declared contract or reviewed legacy mapping), component versions, UI identity, and managed/external state. Existing Settings can display this in its current diagnostics group rather than adding another configuration screen. Serving old external UI is not called an app-wide upgrade.

**Rejected:** Always start the bundled service on another port. That strands users in empty history/model storage and creates the two-workspace problem the user has explicitly rejected. Also reject terminating processes identified only by port/name.

### 3. Keep a narrow transition for existing releases

Add a checked-in legacy compatibility table of exact service-version and renderer-fingerprint combinations, mapped to the bridge protocol whose handlers have been reviewed and tested. Seed the reported 0.4.14/current-renderer combination and the corresponding 0.4.15 build only after validating their contracts. Required API capabilities still apply.

The mapping records the source revision/fixture and rationale. It is not generated from whatever live endpoint happens to answer, downloaded dynamically, or expressed as `0.4.*`. Changes to the new UI must not erase the previous fingerprint. Tests deliberately give the new desktop a different fingerprint while preserving legacy attachment.

Only absent metadata enters this path. Explicit invalid, unknown, or unsupported metadata is an error even if the legacy tuple matches. Unknown older renderers retain browser-only recovery and update guidance. New desktop compatibility cannot retroactively change already installed older desktops; document this one-way migration honestly.

**Rejected:** Accept any service with the current desktop's index hash. That breaks again on the next renderer change and ignores service/bridge contract differences.

### 4. Preserve privileged IPC boundaries

Keep startup-only methods limited to the startup frame and workspace methods limited to the trusted main frame with an accepted attachment. Expose protocol information read-only; do not expose commands, filesystem access, tokens, or a compatibility override.

Reassess identity for explicit reconnect/reload before accepting changed service metadata. An observed workspace or bridge change invalidates the prior attachment and triggers existing computer-session stop/reconsent behavior, not transfer of authority. Cover this with injected fixtures; do not run native actions on a user's desktop to test it.

No full-desktop or native authority follows from base UI compatibility. Computer runtime's independent protocol, workspace, local consent, and action authorization remain enforced.

### 5. Use reason-specific recovery without another wizard

Return stable, bounded reason codes from identity assessment instead of requiring the UI to parse English strings: unsupported service contract, invalid compatibility metadata, unreviewed legacy build, inconsistent bundled components, non-OffGrid listener, and unavailable/timeout. Preserve safe technical details separately.

Map these to existing localized presentation resources. External incompatibility points to updating the workspace service and optionally opening its identified web UI. Bundle inconsistency points to desktop repair. Unrelated listeners never get a trusted-workspace button. Show one explanation, both component versions, the configured address, and relevant actions; do not repeat the same generic paragraph twice.

Change the installer finish title to "OffGrid installed" and explain that first launch checks workspace compatibility. Keep the existing launch-after-wizard-close callback and graceful shutdown behavior. No Docker detection or update button is added to NSIS.

## Risks / Trade-offs

- **Incorrect protocol claims:** A declaration can drift from code. Mitigate with packaged tests exercising the real main/preload boundary, generated manifest checks, explicit protocol-change review, and retained legacy fixtures.
- **Old renderer has a host-bridge vulnerability:** Compatibility is not trust. Keep all existing origin/IPC/consent enforcement and revoke an affected legacy mapping if review finds it unsafe; provide actionable recovery instead of a bypass.
- **Legacy transition becomes an indefinite version matrix:** Limit it to reviewed tuples, keep declared contracts as the normal path, and document removal before withdrawing support.
- **External UI remains older than desktop:** Display both versions and service ownership. Do not automatically substitute packaged UI against an older backend.
- **New contract data makes old clients incompatible:** Additive identity fields preserve API readers, but old desktop's strict policy still applies. This fix requires installing the updated desktop; no claim that an old installer acquires new behavior.
- **Windows-only evidence misses shared-host regressions:** Run existing Linux/macOS packaged startup fixtures as well. Record unavailable platform evidence rather than checking off qualification.

## Migration Plan

1. Implement additive build/identity metadata, explicit policy modes, and the narrowly reviewed legacy table; preserve existing storage and endpoint fields.
2. Exercise old identity fixtures, new compatible/incompatible contracts, actual packaged startup, and the isolated Windows installation/reinstall/uninstall flow. No production installation is modified during these tests.
3. Update the existing documentation and reliability evidence. Only then mark implementation tasks complete. No release number, commit, deployment, or publication is implied by this change.
4. On a separately authorized release, desktop installation fixes the tested legacy combination without requiring container replacement. Updating both components remains necessary for genuinely incompatible contracts or new service features.
5. Rollback is a binary/package choice, not a storage migration. An older desktop may again require matching versions; preserve its existing browser recovery option. Do not downgrade an external service or restore user data automatically.
