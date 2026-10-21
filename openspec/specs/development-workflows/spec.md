# development-workflows Specification

## Purpose

Keep contributor build and test workflows discoverable while separating the public Python client from executable service workers and preserving existing product interfaces.

## Requirements

### Requirement: Python client packaging remains stable
The repository SHALL package the Python client independently under `sdk/python` with the existing `offgrid` distribution name, version policy, import path, public methods, and examples.

#### Scenario: Existing SDK consumer
- **WHEN** the client is installed from the relocated SDK project
- **THEN** `import offgrid` and its existing public client operations remain available without importing or bundling a service worker.

#### Scenario: Independent version policy
- **WHEN** the product version changes
- **THEN** the Python distribution version is not silently rewritten to match it.

### Requirement: Python runtime workers stay separate and trusted
Executable Python service workers SHALL live outside the public SDK project, retain their existing protocol and local-only process boundaries, and remain subject to reviewed runtime identity checks before execution.

#### Scenario: Provider worker is launched
- **WHEN** an installed provider selects its pinned Python worker
- **THEN** the service verifies the expected worker identity and receives the same versioned protocol events as before the move.

#### Scenario: Worker is missing or altered
- **WHEN** the configured worker path is absent or its bytes do not match the reviewed identity
- **THEN** activation fails closed without loading an arbitrary SDK file or executing a different worker.

### Requirement: Contributor commands have stable entry points
The repository SHALL provide root build, test, documentation, and lint targets that delegate to maintained direct commands without installing software or touching a contributor's running service as a hidden side effect.

#### Scenario: Contributor runs a root target
- **WHEN** a contributor runs a documented root target
- **THEN** it invokes the corresponding maintained direct command and reports its failure status.

#### Scenario: Contributor lacks Make
- **WHEN** a Windows contributor cannot use `make`
- **THEN** equivalent direct PowerShell-compatible commands remain documented.

### Requirement: Development files have maintained ownership
Maintained tests, build helpers, release checks, documentation checks, tools, native test containers, and examples SHALL have responsibility-specific locations and active references SHALL resolve to those locations.

#### Scenario: Obsolete active path
- **WHEN** an active CI, packaging, source, or contributor instruction still references a removed `dev/` or `python/` source path
- **THEN** the repository path check fails with the offending file and location.

#### Scenario: Historical evidence
- **WHEN** an archived change or release note records a former path
- **THEN** it remains unchanged and is not treated as an active source reference.

### Requirement: Python SDK checks remain isolated
CI SHALL run the relocated Python SDK tests on Linux and Windows without connecting to a user's live OffGrid service; live-service tests require explicit invocation and isolated state.

#### Scenario: Ordinary CI test run
- **WHEN** CI runs the Python SDK test command
- **THEN** client and worker protocol fixtures run without using an installed user's workspace or default service endpoint.
