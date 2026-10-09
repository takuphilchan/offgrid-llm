# Spec Delta

## Purpose

Let independently updated desktop applications open compatible existing OffGrid workspaces without confusing installation success with service readiness or taking ownership of external services.

## ADDED Requirements

### Requirement: Explicit external-service compatibility

The desktop SHALL assess an external service using supported product, API, renderer/desktop bridge, and required feature contracts rather than release-version equality. Identity metadata SHALL describe the renderer actually served and remain public, uncached, and free of private workspace contents or credentials.

#### Scenario: Different releases with compatible contracts
- **WHEN** a newer desktop discovers an external OffGrid service with a supported bridge contract and required API capabilities
- **THEN** it opens that service's existing workspace even when release versions and renderer fingerprints differ
- **AND** Settings shows the actual desktop and service versions without suggesting that the desktop installer upgraded the service UI.

#### Scenario: Unsupported or invalid contract
- **WHEN** bridge metadata is malformed, references a different served renderer, or requires an unsupported protocol
- **THEN** desktop attachment remains blocked with a specific compatibility reason
- **AND** equal product versions or a matching fingerprint do not bypass that failure.

#### Scenario: Missing required capability
- **WHEN** a service advertises a supported bridge protocol but lacks a required API contract
- **THEN** the desktop refuses attachment and identifies the missing contract without creating a replacement workspace.

### Requirement: Bounded legacy transition

The desktop SHALL support legacy identities only through explicit, reviewed combinations of service version, renderer fingerprint, and required capabilities. The reported 0.4.14/0.4.15 renderer SHALL receive a regression fixture. Adding current metadata or changing the packaged renderer SHALL NOT silently remove an accepted legacy mapping.

#### Scenario: Reported legacy installation
- **WHEN** the fixed desktop encounters legacy service 0.4.14 with the reviewed renderer fingerprint and required capabilities from the reported installation
- **THEN** it opens the existing workspace without requiring container replacement, a separate workspace, or another confirmation
- **AND** it remains externally managed after retry, quit, and relaunch.

#### Scenario: Legacy mapping survives a desktop UI rebuild
- **WHEN** the new desktop renderer changes but the reviewed legacy bridge mapping remains supported
- **THEN** the same legacy service still attaches using the explicit mapping rather than equality with the newly packaged renderer.

#### Scenario: Unknown legacy combination
- **WHEN** the service omits bridge metadata but its version/fingerprint combination has no reviewed mapping
- **THEN** the desktop offers actionable compatibility recovery and does not infer support from a patch-version range.

#### Scenario: Invalid new metadata cannot use legacy fallback
- **WHEN** a service has a reviewed legacy version/fingerprint but explicitly reports invalid or unsupported bridge metadata
- **THEN** attachment is refused rather than falling back to the legacy mapping.

### Requirement: Bundled service consistency

A service launched by the desktop SHALL match the desktop package's expected version, renderer fingerprint, and bridge contract. External-service compatibility rules SHALL NOT conceal mixed or damaged bundled components.

#### Scenario: Mixed installed package
- **WHEN** the owned child reports an unexpected version or renderer fingerprint despite a supported bridge protocol
- **THEN** startup reports that the installed desktop components need repair
- **AND** it does not treat that child as an independently managed compatible service.

#### Scenario: Matching bundled startup
- **WHEN** the configured port is free and all bundled components match
- **THEN** the existing desktop-owned startup path opens the existing desktop workspace without creating another data location.

### Requirement: External ownership and user data preservation

Installation, startup, retry, quit, and removal SHALL NOT stop or replace external services, alter their storage, or silently select an isolated workspace. A timeout or occupied port SHALL NOT be treated as permission to spawn a competing service.

#### Scenario: Retry and uninstall with external service
- **WHEN** a desktop is retried, closed, reinstalled, or uninstalled while an external service owns the configured address
- **THEN** that service remains running and its data is unchanged
- **AND** no additional OffGrid service is started by these actions.

#### Scenario: Unresponsive or unrelated listener
- **WHEN** the configured address times out, redirects, serves invalid identity data, or belongs to another product
- **THEN** startup remains bounded and responsive, explains the failure, and neither kills the listener nor silently chooses another workspace.

### Requirement: Compatibility does not authorize host access

Compatibility metadata SHALL NOT substitute for authentication, trusted-main-frame IPC validation, native protocol checks, local consent, or scoped action authorization. A compatibility decision SHALL be refreshed before an explicit reconnect or loading a changed service identity.

#### Scenario: Compatible renderer without computer consent
- **WHEN** an accepted renderer requests computer access without the existing required authorization or local consent
- **THEN** the same native access checks apply and attachment itself grants no control authority.

#### Scenario: Untrusted frame
- **WHEN** a subframe, another window, or a different origin calls the desktop bridge despite advertising compatible metadata
- **THEN** the request is rejected under the existing IPC trust policy.

#### Scenario: Service changes during recovery
- **WHEN** Retry reaches a service whose workspace, renderer, or contract identity differs from the prior probe
- **THEN** compatibility is evaluated again and previous computer grants are not reused as authority for the changed identity.

### Requirement: Actionable startup recovery

Startup SHALL show one concise, localized explanation and a relevant next action for contract incompatibility, bundle inconsistency, unrelated listeners, and unavailable services. Recovery SHALL preserve monochrome styling, all nine locales, keyboard use, and actual component-version diagnostics.

#### Scenario: External service requires update
- **WHEN** an identified external OffGrid service has an unsupported contract
- **THEN** recovery identifies the service as the component needing an update, displays both versions and the address, and offers the existing web workspace and update guidance
- **AND** it does not recommend repeatedly reinstalling the desktop or duplicate the same generic paragraph.

#### Scenario: Unrelated listener is not an OffGrid workspace
- **WHEN** the address belongs to an unrelated application
- **THEN** recovery explains the address conflict and does not offer to open it as a trusted OffGrid web workspace.

### Requirement: Truthful installer completion

The Windows installer SHALL distinguish successful application installation from workspace readiness. It SHALL retain graceful owned-desktop shutdown, unprivileged launch after Finish, and preservation of installed-user data.

#### Scenario: Installation completes with older external service
- **WHEN** installation succeeds while a separately managed service remains running
- **THEN** Finish reports that the application is installed, not that the workspace is ready, and explains that separate services are not upgraded
- **AND** launch occurs only after the wizard closes, followed by the normal compatibility check.
