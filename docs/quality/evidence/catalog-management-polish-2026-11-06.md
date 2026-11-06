# Catalog and management presentation follow-up

Local Windows candidate, 2026-11-06. Installed after the checks below; not published.
Existing workspace data, model files and integration configuration were preserved.

## Changes

- Agents now has Agent profiles, Tools & MCP, and External agents. The old
  `#/agents/connections` route still opens MCP setup. Connection drafts survive
  section navigation. External readiness requests are isolated from tools/MCP;
  a failed provider does not hide the other section's results.
- MCP setup appears on request. External provider cards keep technical fields
  under Details and retain service/model versus host/workflow readiness distinctions.
  Cards lead with the integration name and purpose, use plain “Check requirements”
  or “Model compatible” status text, and provide aligned Setup instructions actions.
  Host setup remains explicitly unchecked; compatibility is not a workflow pass.
- Read-aloud preparation uses a compact progress/Stop action instead of growing
  the message action row. Its explanation remains available to assistive technology.
  Response details uses a 224px label/value popover.
- The catalog adds five small-model families: Qwen3.5, SmolLM3, Granite 4.1,
  LFM2.5 and Gemma 4. Three starter suggestions come from different families.
  Old IDs and artifacts remain available as compatibility choices. Exact source
  revisions and weight hashes are pinned; vision bundles resolve their projector.
  MTP draft weights are excluded from companion selection. No weights were downloaded.

## Verification

- `go test ./internal/models ./internal/server ./internal/cli -count=1`: passed.
  Log: `build/catalog-packages-20261106.log`.
- `OFFGRID_CATALOG_NETWORK_CHECK=1 go test ./internal/models -run
  'CurrentCatalog|VisionBundle|Qwen35' -count=1 -v`: passed. This opt-in check reads
  public repository metadata, not model data. All five pinned entries matched
  their source size/hash/revision; both expected vision projectors resolved.
  The Windows symlink test skipped because creating symlinks needs privilege.
  Log: `build/catalog-diversity-check-20261106.log`.
- `npm run check`, `npm run api:check`, `npm run build`: passed. Build retains
  chunk-size/dynamic-import warnings. Log: `build/agent-catalog-ui-build-20261106.log`.
- Playwright: voice-playback, chat-streaming, action-presentation,
  connection-readiness, control-consistency, mcp-connections,
  external-integrations, functional-pages, interaction-contracts,
  agent-availability, task-management and workflow-presentation: **138 passed**.
  Log: `build/agent-response-final-20261106.log`.
- `agent-management-layout.spec.ts`: **4 passed**, covering populated external
  provider cards at 390/1440px in light/dark, hidden-by-default technical fields,
  navigation isolation and zero configuration writes. Log:
  `build/external-agent-layout-20261106.log`. The 1440px/dark and 390px/light
  captures were visually inspected, including readiness warnings and setup actions.
- Inspected Tools & MCP at 1440px/light and 390px/light, plus speech preparation
  at 1440px/dark and 390px/light. No horizontal overflow; speech preparation
  keeps the action-row height stable. Fixtures use isolated state.
- Maintained-document check: 327 files, zero problems. Source-size ratchet clean.
  `openspec validate workspace-outcome-experience --strict`: passed.
- Final card follow-up: 33 targeted browser checks passed; after the final setup
  label change, the build and six layout/integration checks passed again.
  Desktop unit tests: 137 passed, one platform skip. Logs:
  `build/provider-card-final-20261106.log`,
  `build/provider-cards-layout-20261106.log`, and
  `build/provider-cards-desktop-tests-20261106.log`.
  Desktop/dark and narrow/light populated cards were visually inspected.
- The packaged Windows candidate passed startup/recovery, terminal and appearance
  checks. Terminal coverage includes interactive CLI/Agent entry, tabs, Unicode,
  cursor preservation and owned-child cleanup. Appearance coverage includes nine
  language options and 200% zoom. Logs: `build/provider-cards-startup-20261106.log`,
  `build/provider-cards-terminal-20261106.log` and
  `build/provider-cards-theme-20261106.log`.

Packaging initially rejected the service because `go build -trimpath` omitted
the linker flags inspected by the bundle verifier. Rebuilding without trimpath
and with explicit `-X main.Version=0.4.27` passed the same verifier. No verifier
was disabled and the rejected candidate was not installed.

Earlier assertions exposed a stale cancel-label key, collapsed connection drafts
on navigation, old Connections-tab selectors and an old Add document height
assumption. These were corrected before the final 138-test run. The catalog
wrapper initially exceeded the source-size ratchet; it now lives with the new
catalog definitions rather than expanding the legacy catalog file.

These checks do not establish live model quality, GPU memory requirements,
external-agent workflow qualification, native installation success or human
usability. No new model is claimed qualified from repository metadata alone.
Unavailable platform and human-review gates remain open.

## Windows installation

Installed from `build/desktop-provider-cards-20261106` with installer SHA-256
`12901d5cc0e1e339addddc3066fb7d5b3c17cf1cfe4422ba3dfa8e7d4651dba4`.
All five checked application payloads matched the tested candidate. All 392
workspace files, the Electron profile, model inventory and inference-runtime
hashes were preserved. The OffGrid Start shortcut and stable uninstall identity
passed verification. All installed UI assets were compared against the candidate.

Rollback: `D:/OffGrid-install-backups/v0.4.27-before-provider-cards-20261106-214536`.
Receipt: `build/desktop-provider-cards-20261106/install-result.json`.
The normal app remains closed; no live user-workspace walkthrough is claimed.
