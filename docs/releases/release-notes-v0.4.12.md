# OffGrid LLM v0.4.12

This release brings a task-first Agents workspace, durable workflow controls,
MCP connection removal, and shared browser/desktop usability repairs. Computer
Tasks remains a preview; this is not a claim of universal desktop automation.

## Changes

- Describe an outcome and start a saved task without configuring computer access
  first. When access is needed, continue that same task in the matching desktop
  app and approve the actual application or managed browser locally. Ordinary
  desktop setup does not require terminal commands or manual pairing codes.
- Resume work from saved snapshots and ordered events. Actor-scoped request IDs
  deduplicate submissions. Pause, take over, stop, reconnect access, and update a
  safely paused instruction through the shared service, web, desktop, and CLI.
  Uncertain effects remain protected against automatic repetition.
- Support bounded work plans, exact archived context, and linked read-only
  subtasks with dependencies. Subtasks do not inherit computer access or approval.
  This is not arbitrary recursive multi-agent or concurrent desktop control.
- Save private downloadable text, Markdown, JSON and CSV artifacts, with
  independent read-back, digest and format checks. These checks verify storage,
  not the factual accuracy of model-authored content.
- Show task and Activity deletion controls with confirmation. Protect active
  work, unresolved outcomes and referenced child history. Keep tools and
  connections in separate management views, and improve task-card spacing.
- Remove saved MCP connections, including offline entries. Removal persists
  before unregistering the exact connection's tools and closing its transport;
  failure leaves the connection intact. History and remote data are not deleted.
- Keep native dropdown menus readable in dark and light desktop themes, including
  an application appearance that differs from the operating system. Retain the
  established monochrome presentation, nine interface locales and RTL support.

## Computer Tasks preview

Since v0.4.11, packaged native workers add selected-window structured-control
paths for Windows UI Automation, macOS Accessibility and Linux AT-SPI, alongside
the dedicated browser worker. Available operations depend on driver permissions
and exposed controls. Existing browser windows can be selected as native targets;
this does not import personal browser profiles into the managed browser.

Local consent binds the target and approval mode: ask every time, approve scoped
changes, or full task access within that scope. Protected operations remain
blocked; full task access is not unrestricted shell, credential, payment or
whole-desktop authority. Stop revokes the selected task's access without stopping
another task's application. Stale state and rejected input have explicit recovery.

Managed browsing supports real permitted HTTPS page addresses, explicitly
consented trusted-proxy routing, and bounded file-transfer tools. Proxy mode
trusts routing hidden by the selected VPN/proxy; it is not direct-IP isolation.
Uploads use locally selected file grants; downloads are staged and checked.
Vision remains gated by a compatible installed profile and successful checks;
structured operation does not imply vision or real-model task qualification.

## Upgrade

Back up the workspace and stop active tasks before upgrading. Update the desktop
host and service together; replacing Docker alone cannot update host components.
Existing models do not need to be deleted or downloaded again. Web/container
users still need a matching desktop host for local application input.

Agent storage migrates transactionally to schema 4, validating legacy records
and retaining migration backups. Older binaries must not open the migrated
store. If recovery is necessary, stop the service and follow the documented
backup/restore procedure; do not roll back the companion duplicate-action journal.
Restart requires fresh local computer consent, not automatic task resumption.

See the [agent guide](../guides/agents.md) for CLI commands, ownership,
reconciliation and history-deletion behavior.

## Evidence and limitations

The [reliability plan](../advanced/product-reliability-plan.md) records tests,
local installed-package checks and deployment evidence. Full native/vision
platform/model qualification, independent security review, signing and the
soak/pilot program remain incomplete. Passing synthetic tool-call checks is not
proof of model-planning reliability. Linux Wayland and application-specific
behavior must not be inferred from AT-SPI/X11 tests. Unsigned packages may show
operating-system reputation warnings; verified-publisher distribution is not
claimed.

CPU container: `takuphilchan/offgrid-llm:0.4.12` (AMD64/ARM64).
NVIDIA container: `takuphilchan/offgrid-llm:0.4.12-gpu` (AMD64).
Verify downloaded assets using `checksums-v0.4.12.sha256`.

[Full changelog](https://github.com/takuphilchan/offgrid-llm/compare/v0.4.11...v0.4.12)
