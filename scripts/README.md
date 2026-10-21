# Maintained scripts

Run scripts from the repository root unless their usage says otherwise. The root
[Makefile](../Makefile) exposes safe contributor build/test shortcuts; Windows
users can run direct commands in [Building](../docs/advanced/BUILDING.md).
Install, download, deployment, live-service, and native desktop scripts are
**not** part of `make test`. Use isolated state and review side effects first.

| Path | Purpose and owner | Prerequisites / side effects |
| --- | --- | --- |
| `../install.sh`, `install.sh` | Public Unix release installer and preserved legacy URL wrapper; release | Downloads and installs published components. |
| `build-all.sh`, `sync-ui.sh` | Build binaries and copy the renderer; contributor/release | Go/Node; writes generated output. |
| `start-server.sh` | Start a local service; contributor | Binds a port and uses configured workspace. |
| `clean-repo.sh` | Remove generated local output; contributor, explicit only | Destructive to build output/logs; inspect targets first. |

## `build/` — explicit build and source-install helpers

| Script | Purpose | Prerequisites / side effects |
| --- | --- | --- |
| `build-static-bundle.sh` | Experimental static llama.cpp bundle | Go/native toolchain; may download/build dependencies. |
| `build-all.sh`, `sync-ui.sh` | Build release variants or the shared UI | Go/Node; writes generated output. `build-all` may install npm dependencies and replaces its own output directory, so review before use. |
| `build-whisper-binaries.sh` | Compile Whisper binaries | Native toolchain/source; writes binaries. |
| `create-usb-package.sh` | Assemble USB distribution | Existing artifacts; writes output tree. |
| `install-from-source.sh` | Preserve older GPU/source/systemd install flow | Linux, sudo, network, native toolchain; installs software/services. Not public default. |
| `source-install-build.sh` | Sourced functions for source installer | Never run directly; may install/build when called. |
| `install-llama-service.sh`, `llama-server-start.sh` | Legacy llama service setup/start | Linux/systemd; changes service or launches inference. |

## `test/` — fixtures and opt-in integration checks

| Scripts | Purpose | Prerequisites / side effects |
| --- | --- | --- |
| `check-python-package.py` | Isolated SDK wheel/import check | Python/wheel; temporary install only. CI-owned. |
| `desktop-external-fixture.mjs`, `mcp_test_server.py` | Disposable service fixtures | Node/Python; temporary local listeners. CI-owned. |
| `test-agent-cross-clients.mjs`, `test-task-jobs.mjs`, `test-web-workspace.mjs` | Isolated job/browser fixtures | Built service, Node/Playwright; disposable data/ports. CI-owned. |
| `test-computer-desktop.cjs`, `test-desktop-access.mjs`, `test-desktop-startup.mjs`, `test-desktop-theme.mjs`, `test-packaged-browser.mjs` | Desktop packaging/startup checks | Packaged app/Node; may launch isolated applications. CI/explicit platform owner. |
| `test-native-document-workflow.mjs`, `test-native-document-workflow.test.mjs`, `test-native-linux.sh`, `test-native-macos.sh`, `test-owned-edge-companion.mjs` | Native contract and platform checks | Disposable account/desktop and OS permissions for live variants; never personal documents. |
| `score-provider-workflows.mjs`, `score-provider-workflows.test.mjs` | Score recorded trials against the frozen provider-workflow manifest | Node; reads result/evidence files only. Does not execute workflows or qualify hardware by itself. |
| `test-p2p-local.sh`, `test_embeddings.sh`, `test_streaming.sh`, `test_embeddings_client.go`, `test_voice_input.py`, `quick_voice_test.py` | Network/model/audio probes | May contact a service or use models/microphone; opt-in only. |

## `release/` — release and installer verification

`test-finalize-release.mjs`, `test-release-asset-access.mjs`, `test-release-ci.mjs`,
`test-release-containers.mjs`, and `test-release-context.mjs` inspect release
contracts with fixtures; CI-owned, no publication. `test-windows-installer.ps1`,
`test-windows-installer-probe.ps1`, `windows-installer-capture.ps1`,
`windows-installer-hang.ps1`, and `windows-installer-ui.ps1` exercise only
isolated test installer identities; they can install/uninstall.
`deploy-computer-preview.sh` performs deployment and requires a separate decision.

## `maintenance/` — checks and generated assets

`check-docs.mjs`, `check-docs.test.mjs`, `check-application-layout.mjs`,
`check-application-layout.test.mjs`, `check-repository-hygiene.mjs`,
`check-repository-hygiene.test.mjs`, `check-community-templates.test.mjs`,
`check-developer-layout.mjs`, `check-developer-layout.test.mjs`,
`check-source-size.mjs`, `check-source-size.test.mjs`,
and `test-installer-entrypoints.sh` are CI-owned checks. The checkers read source;
their fixture tests use temporary files and local stubs, never installed-user
data or real installer/service actions.
`source-size-allowlist.json` records the owner, responsibility, next review date,
and exact no-growth line baseline for each existing handwritten production file
above 600 lines. New oversized files require explicit review; `cmd/*/main.go`
has a non-exempt 150-line limit. Historical/generated content is excluded.
`generate-icons.mjs` and `generate-installer-branding.mjs` require desktop
dependencies and write generated assets. `fetch_hashes.go` fetches hashes over
the network; review output before use.
`get-version.sh` reads `VERSION`; `update-version.sh` rewrites release metadata
and npm package/lock files. Both are release-owned; the SDK version stays
independent.

Maintained client demos live in [`examples/clients`](../examples/clients/),
the metadata tool in [`tools/model-package-metadata`](../tools/model-package-metadata/),
and native test image in [`docker/test`](../docker/test/).
