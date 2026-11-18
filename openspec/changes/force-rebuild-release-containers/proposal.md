# Proposal: Force-rebuild release containers

## Why

The container release workflow reuses CPU and NVIDIA images when their versioned
tags already verify against the source commit. That is correct for normal release
publication, but maintainers cannot request a deliberate rebuild after a release
has been published without changing the immutable Git tag or moving application
source to a repair branch.

## Scope

- Add a manual container workflow option to force rebuilding images from an
  explicitly selected immutable `vX.Y.Z` tag matching the requested version.
- Reuse reviewed Dockerfile packaging from the workflow revision while building
  the application source from that tag.
- Build CPU AMD64 and ARM64 images, publish their multi-platform manifest, and
  build the NVIDIA AMD64 image when GPU publication is enabled.
- Verify the resulting image platforms and source revision.

## Exclusions

- Do not move or recreate a Git tag, modify GitHub release assets, or change
  application source.
- Do not force rebuilds during ordinary publication or change normal reuse
  behavior.
- Do not move latest or minor aliases backward for an older release.

## Acceptance criteria

- A manual forced rebuild with `version=0.4.32` and `source_ref=v0.4.32` uses
  that tag's commit as application source and bypasses reuse for CPU and GPU.
- A forced manual rebuild without a matching semantic-version tag fails before
  attempting any image publication.
- The CPU manifest is published only after both architecture builds succeed;
  final verification checks CPU AMD64/ARM64 and NVIDIA AMD64 source identity.
- Existing manual preview and normal release reuse behavior remains intact.
