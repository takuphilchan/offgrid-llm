## ADDED Requirements

### Requirement: Forced container rebuilds use immutable release source
The manual container workflow SHALL allow a maintainer to force-rebuild CPU and
NVIDIA images from an immutable semantic-version tag without moving the Git tag
or changing the corresponding GitHub release.

#### Scenario: Matching release tag is forced
- **WHEN** a maintainer dispatches a forced rebuild with version `X.Y.Z` and
  source tag `vX.Y.Z`
- **THEN** the workflow bypasses image reuse, builds CPU AMD64 and ARM64 plus the
  NVIDIA AMD64 image when enabled, and labels application source with the tag's
  commit.
- **AND** the multi-platform CPU manifest is published only after both CPU
  architecture builds succeed, followed by source and platform verification.

#### Scenario: Forced tag is missing or mismatched
- **WHEN** a maintainer requests a forced rebuild without a semantic-version
  source tag matching the requested version
- **THEN** source resolution fails before any container image is published.

#### Scenario: Ordinary publication or preview
- **WHEN** a workflow runs without the explicit forced-tag request
- **THEN** existing image reuse and publication behavior remains unchanged.
