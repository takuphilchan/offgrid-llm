# Proposal

## Why

Alternative desktop install scripts omit runtime files, trust unverified downloads, and can interrupt unrelated work. Their guidance overstates first-use and platform readiness.

## What Changes

- Verify release checksums before extracting or executing each downloaded payload.
- Install the complete Windows CLI bundle using a user-writable directory and user PATH when unelevated.
- Stop Unix CLI installation with guidance when OffGrid is running, without killing it.
- Lead with desktop downloads and document actual platform capabilities, runtime preparation, GPU prerequisites, compatibility, and manual upgrades.
- Exercise failure paths using disposable fixtures without installing software or stopping services.

Scope agreed with the user: installer scripts and download/platform guidance first. Onboarding implementation, GPU provisioning, signing infrastructure, and automatic updates are excluded.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `installer-entrypoint-compatibility`: safe, verified alternative desktop entry points and complete Windows payload installation.

## Impact

`installers/desktop.ps1`, `installers/desktop.sh`, installer tests, and setup documentation. No API, user-data migration, publication, or native-control changes.
