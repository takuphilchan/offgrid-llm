# Proposal

## Why

The packaging audit found remaining unsafe root installer behavior, incomplete CLI runtime discovery, delayed inference setup, uncertain publisher trust, and inconsistent hardware guidance. The user requested thorough implementation and installation of the new Windows app.

## What Changes

- Apply verified downloads and ownership-preserving upgrade behavior to the root installer and archive helpers.
- Preserve and discover complete UI, speech, inference and execution payloads across installation paths.
- Make first-run inference preparation and CPU/GPU readiness visible; require an actual response before declaring readiness.
- Add explicit artifact trust verification and truthful signing status; retain unsigned local preview support without security bypasses.
- Correct hardware/platform requirements and simplify desktop download and manual upgrade guidance.
- Distinguish the local preview build from public 0.4.28, qualify its installer in isolation, then install it on this Windows machine with workspace preservation and startup/response verification.

The user subsequently authorized installation, commits and publication of the new version. Publisher credentials and other OS machines are external prerequisites if unavailable; their absence must remain visible, not be replaced with a claim of qualification. Publication must retain the existing release checks and source/artifact identity requirements.

On 2026-11-12 the user explicitly requested release without waiting for unavailable
credentials, superseding the earlier signed-only choice. Publish 0.4.29 through
an explicit unsigned-preview channel with truthful OS warnings, native trust
reports, complete assets and exact-source CI. Keep signed publication as the
default and leave signed/notarized qualification incomplete.

## Capabilities

### New Capabilities

- `distribution-readiness`: complete runtime discovery, first-response readiness, artifact trust and installed build identity.

### Modified Capabilities

- `installer-entrypoint-compatibility`: all maintained installer paths verify payloads before use and preserve running work.

## Impact

Installers, release packaging/helpers, runtime discovery, first-run UI/API, setup documentation and isolated installation tests. Continue the completed installer-script-reliability work without rewriting its evidence. Preserve unrelated UI work and installed user state.
