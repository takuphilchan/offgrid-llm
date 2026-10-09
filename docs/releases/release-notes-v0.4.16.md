# OffGrid LLM v0.4.16

This patch fixes desktop startup being blocked solely because an independently
managed OffGrid service has a different release version. It preserves existing
workspace data, models, permissions, and the shared monochrome interface.

## Desktop installation and startup

- External services are checked against their API and renderer-to-desktop bridge
  contracts, not release-number equality. Compatible versions can open the same
  existing workspace without starting another service.
- A narrow reviewed mapping supports the reported 0.4.14/0.4.15 workspace-refresh
  renderer. It survives subsequent desktop UI rebuilds. This is not blanket
  compatibility with every older build; unknown legacy renderers remain blocked.
- Bundled services still require matching package versions, renderer identity,
  and bridge metadata. Damaged or mixed packages cannot use the external-service
  policy to bypass repair checks.
- Reconnect and full-page reload reassess compatibility. A changed workspace or
  renderer invalidates prior computer access rather than transferring it. Failed
  local stopping keeps access unavailable. A disconnected external service does
  not silently become a different bundled workspace on Retry.
- Startup explains whether the external service needs updating, the desktop
  needs repair, another application occupies the address, or the service is
  unavailable. Duplicate guidance is removed; all nine interface locales remain.
- Settings reports actual component versions, ownership, and compatibility basis.
  Windows Setup says **OffGrid installed**, then checks workspace readiness at
  launch. It does not update or stop independently managed Docker services.

## Verification and limitations

Local checks include the full Go suite, eleven-package WSL race suite, API drift,
TypeScript/build, 280 browser tests (ten opt-in skips), 58 desktop unit tests,
packaged Windows startup/browser/theme checks, and isolated Windows installation,
repair and uninstall. Installer tests verify that the compatible legacy external
service remains running and its fixture data is unchanged.

Release publication is gated on CI for the exact source revision, all required
release assets and their checksums, and CPU/GPU container verification. CI adds
packaged Linux and macOS Intel/Apple Silicon checks; automated fixtures are not
qualification of every platform, historical upgrade or model. Evidence and
limitations remain in the [reliability plan](../advanced/product-reliability-plan.md).

This patch does not complete the Voice or Computer Tasks programs, add computer
permissions, or bypass authentication and local consent. Existing speech-runtime,
model/hardware qualification, Talk, native/vision, accessibility and pilot limits
remain. Publisher signing and macOS notarization are not claimed; follow the
[desktop trust guidance](../setup/desktop-startup.md#installer-appearance-and-os-warnings).

## Upgrade

Back up workspace data and model storage and stop active work before upgrading.
Install the updated desktop to receive the compatibility fix; source changes or
a container-only update cannot change an older installed desktop's policy.
The reviewed legacy service can remain installed. Update the service separately
when you need newer server/UI features or its contract is genuinely incompatible.
Preserve Docker volumes and any separately installed speech runtime. This patch
introduces no workspace storage migration and never silently creates a replacement
workspace.

CPU image: `takuphilchan/offgrid-llm:0.4.16` (AMD64/ARM64).
NVIDIA image: `takuphilchan/offgrid-llm:0.4.16-gpu` (AMD64).
Verify release assets with `checksums-v0.4.16.sha256`.

[Full changelog](https://github.com/takuphilchan/offgrid-llm/compare/v0.4.15...v0.4.16)
