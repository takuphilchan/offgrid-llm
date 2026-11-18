## Implementation

- [x] Add guarded `source_ref` and `force_rebuild` manual-dispatch inputs and
  validate that forced source tags match the requested version.
- [x] Apply reviewed CPU/GPU Dockerfile packaging to explicit tag dispatches,
  preserve source and packaging revision labels, and normalize manual image tags.
- [x] Add focused workflow contract tests for valid and invalid forced inputs,
  forced reuse bypass, and packaging overlay selection.
- [x] Document the exact-tag forced container rebuild procedure.

## Validation and publication

- [x] Run `node scripts/release/test-release-containers.mjs` (passed in WSL).
- [ ] Run the repository CI actionlint command against active workflows.
- [x] Run `openspec validate force-rebuild-release-containers --strict --no-interactive`.
- [ ] Dispatch the v0.4.32 CPU/GPU rebuild and verify both CPU architectures, the
  NVIDIA image, the multi-platform manifest, and source identity.

Local actionlint could not download its pinned Go module because the Go proxy
connection was unavailable; CI will provide the workflow lint result.
