# installer-entrypoint-compatibility Specification

## Purpose

Preserve the public Unix installation entry points and their supported install behavior while consolidating the maintained implementation.

## Requirements

### Requirement: Canonical and legacy entry points remain available
The repository SHALL keep root `install.sh` as the maintained Unix installer and SHALL keep the published `scripts/install.sh` URL functional.

#### Scenario: Root entry point
- **WHEN** a user invokes the documented root installer with a supported option or environment variable
- **THEN** the existing installation target, service identity, and release asset selection are preserved.

#### Scenario: Legacy raw URL
- **WHEN** a user invokes the published `scripts/install.sh` raw URL, including a streamed invocation
- **THEN** the installer enters the canonical implementation without requiring the repository to be checked out locally.

### Requirement: Unique legacy behavior is retained before consolidation
Consolidation SHALL preserve supported legacy GPU, CPU, autostart, and systemd behavior or explicitly leave the legacy implementation in place until equivalence is verified.

#### Scenario: Unique option exists
- **WHEN** comparison finds a supported legacy option absent from the root installer
- **THEN** the legacy implementation is not replaced with an incompatible wrapper until the behavior is ported and tested.

#### Scenario: Installer failure
- **WHEN** download, architecture detection, checksum verification, or service installation fails
- **THEN** the installer reports failure and does not claim a successful installation.

### Requirement: Relocated unit sources preserve installed identity
Maintained systemd unit source files MAY move in the repository, but installation SHALL preserve their existing installed unit names and behavior.

#### Scenario: Unit source relocation
- **WHEN** an installer or maintained helper installs a relocated unit
- **THEN** it installs the same service name and configuration expected by existing deployments.
