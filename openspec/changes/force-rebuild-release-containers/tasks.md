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
- [x] Run the repository CI actionlint command against active workflows (passed
  in CI job `Validate release automation`).
- [x] Run `openspec validate force-rebuild-release-containers --strict --no-interactive`.
- [x] Dispatch the v0.4.32 CPU/GPU rebuild and verify both CPU architectures, the
  NVIDIA image, the multi-platform manifest, and source identity.

## Evidence

- Container workflow run [44251679784](https://github.com/takuphilchan/offgrid-llm/actions/runs/44251679784)
  completed successfully. CPU manifest digest:
  `sha256:8acd7729d1087854e7621dc7290af479b549b86c2e896421ed9ddba13088c1fd`.
  NVIDIA image digest:
  `sha256:6f9a064238089afc42cd0442cc0e9aeca2a1cbad80a8f26a49b008b3b17cf80e`.
- The prior CPU manifest digest `sha256:08145e62cda4881825c83707c26376ae5e3ceaad2f3f176a89293d2f404c48d2`
  was replaced. Both resulting tags verified against application revision
  `230d0ad7b29b16c84ee2fda6ccffae8735351508`.
- CI run [44251653996](https://github.com/takuphilchan/offgrid-llm/actions/runs/44251653996)
  passed actionlint but failed one unrelated browser assertion:
  `tests/typography-consistency.spec.ts` expected `.topbar h1` to be visible on
  the settings screen. No release-workflow validation failed.
