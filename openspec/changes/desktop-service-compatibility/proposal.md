# Proposal

## Why

Installing desktop 0.4.15 leaves the independently managed 0.4.14 container running, then `desktop/backend.js` rejects it because release versions differ before checking renderer compatibility. This causes a recurring post-installation dead end, even in the observed case where both builds have the same renderer fingerprint, while the installer and recovery copy do not explain the ownership boundary clearly.

## What Changes

- Separate release versions from desktop/service compatibility. Add an explicit, versioned renderer-to-desktop bridge contract to the existing public system identity, tied to the renderer actually served; continue checking API and feature contracts independently.
- Attach to a compatible, externally managed workspace across release versions without starting another service or changing its data. Keep exact binary/version/renderer checks for a service launched from the desktop package.
- Provide a narrow, tested legacy transition for the reported 0.4.14/0.4.15 identity format with explicitly reviewed renderer fingerprints. Retain those fingerprints when the new desktop renderer changes. Do not assume all patch releases, all older renderers, or self-reported hashes are compatible or authentic.
- Show actionable, localized startup reasons: incompatible workspace contract, mismatched bundled files, unrelated occupied port, or unavailable service. Show desktop and service versions separately; remove duplicate generic guidance.
- Make installer completion mean "application installed," not "workspace ready." Explain that independently managed services are not upgraded by the desktop installer. Preserve the existing graceful shutdown and launch-after-Finish behavior.
- Add regression tests at the Go identity, desktop policy, real packaged startup, and isolated Windows installer layers. Document how future releases maintain or deliberately change the bridge contract.

### User-visible acceptance

1. The reported same-renderer legacy fixture opens its existing workspace after installing the fixed desktop, without container replacement, a second workspace, or repeated recovery choices.
2. Future external desktop/service releases with compatible contracts attach despite different release numbers and renderer builds. The service remains the source of the displayed web UI, and Settings identifies both versions honestly.
3. An incompatible service or broken desktop package remains blocked with a specific reason and useful next action. Reinstalling desktop is not presented as a remedy for an independently managed service.
4. Quitting, retrying, reinstalling, and uninstalling never stop an external service, reset user data, silently select another workspace, or grant computer access.

### Exclusions

No Docker/WSL administration, automatic service update, occupied-port process termination, data migration, authentication bypass, relaxed native consent, new installer configuration wizard, or live installation replacement. This change prevents version-only false conflicts; it cannot promise that corrupted packages, incompatible services, or network failures never occur. Publication and deployment remain separate actions.

## Capabilities

### New Capabilities

- `desktop-service-compatibility`: Verified compatibility decisions, owned-versus-external startup, and understandable installation/recovery behavior.

### Modified Capabilities

None. There are no maintained capability specs yet. The in-flight `workspace-outcome-experience` deliberately leaves attachment contracts unchanged; this separate change proposes that boundary change rather than silently expanding its scope.

## Impact

- `internal/server/system_identity.go` and tests; additive public identity metadata and applicable API types/docs.
- Shared renderer build metadata in `web/app/vite.config.ts` and packaging validation, without redesigning the workspace UI.
- `desktop/backend.js`, `runtime.js`, main/preload compatibility handling, presentation copy, startup UI, installer copy, and their tests.
- Existing Settings backend diagnostics, packaged startup/installer fixtures, and CI commands; no new runtime dependency or database.
- Authoritative documentation: `docs/advanced/client-contracts.md`, `docs/setup/desktop-startup.md`, `desktop/README.md`; evidence in `docs/advanced/product-reliability-plan.md` without changing production-readiness gates.
