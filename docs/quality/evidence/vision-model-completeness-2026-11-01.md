# Complete GGUF vision downloads — 2026-11-01

Implemented in source; installed-user inference qualification is still pending.
No installed model, active inference process, desktop package or user draft was
changed during these tests.

## Cause and actual source inspection

The shared service downloaded only the selected GGUF. Registry association used
legacy known-family projector names; it could not associate a generic companion
with the reported Nemotron model. The installed directory contained the selected
5,060,482,240-byte weights and no matching projector.

A metadata-only resolver check against
[the publisher repository](https://huggingface.co/unsloth/NVIDIA-Nemotron-3.5-Bolt-E4B-GGUF/tree/b738967d6da3441a7d1c71fcec15e49e32e16ecd)
resolved:

- Revision: `b738967d6da3441a7d1c71fcec15e49e32e16ecd`.
- Weights: `NVIDIA-Nemotron-3.5-Bolt-E4B-UD-Q4_K_XL.gguf`, 5,060,482,240 bytes,
  SHA-256 `84ff34f5834b929043f5ae74799e8d562d4056998f6a5923ceb44831e8ad84d6`.
- Projector: `mmproj-F16.gguf`, 990,373,568 bytes,
  SHA-256 `6bc77b7ac2b62cf61bc86757605e45e39f6345373f0a6b76cc5b1ed97f2e233e`.

The source response is retained in `build/vision-source-probe.json`. This checked
upstream metadata, not the installed model's full digest or image accuracy.
Other quantizations, projector precisions, dflash and imatrix files are not extra
dependencies to download indiscriminately.

## Implemented behavior

Downloads resolve and persist one immutable bundle, verify sizes and hashes, and
associate a model-specific companion only after both files pass. Existing weights
are never overwritten. Explicit repair handles a missing companion and retains
a damaged managed companion before replacing it. Cancel/restart retains the
revision and digest-specific partial identity. Registry validation rejects altered
file identities and invalid receipts; companion files are not chat-model choices.
This local integrity cache is not a signed trust grant.

Web/desktop discovery and recorded-source installed-model menus offer **Check and
repair files** on services advertising `gguf-bundles-v1`. Older services retain
their existing UI. The service-backed CLI shares acquisition; the local init
wizard now uses the same resolver/acquisition code. Image submission never starts
a download, and draft-retention behavior remains unchanged.

## Validation

- Go regression tests cover Nemotron-shaped F16/BF16/Q8 alternatives, missing or
  ambiguous source layouts, corrupt data, cancellation, repair, preserved existing
  weights, receipt traversal/identity rejection and generic-name collisions.
- Shared-service test verifies aggregate completion, source persistence across
  restart and companion repair without redownloading unchanged weights.
- Metadata-only live source resolution passed; no real model assets were fetched.
- `go test ./...` passed on Windows.
- API generation/check and web TypeScript/build passed. The build retains its
  large-chunk advisory; this change does not claim bundle-size optimization.
- Two discovery browser tests passed, including explicit repair and narrow layout.
- 47 model download/selection/library and Chat image browser tests passed,
  including failed-send image retention and nine-locale composer checks.
- Desktop tests: 99 passed, one skipped, zero failures.
- WSL/Linux race tests passed for models, RAG, server and CLI; final vision
  source-binding/symlink tests also passed with the race detector.
- Final image recovery tests: 20 passed. Missing-projector errors now open Models
  instead of offering an unrelated request-status refresh. Images remain retained.
  An initial test used the wrong button label; the corrected test verifies the
  actual Models action and absence of the old status action.
- Source-size, repository-hygiene, maintained-doc checks and strict change
  validation passed after correcting one source-size increment and removing the
  temporary probe's Go package from the build-output tree.

## Remaining gates

Windows race testing initially could not start with CGO disabled; the passing
Linux race run uses WSL Go 1.26.6/GCC, not native Windows race coverage.
The full advertised cross-platform CI matrix and a fresh installed Nemotron image
response have not been qualified. Two native inference workers were running during
inspection; they were left untouched. Companion download/repair must be activated
through the updated service before testing this installed model. The runtime may
still reject an unsupported architecture despite complete assets.

GGUF preflight does not reserve free disk space. Damaged companions and orphaned
companion data remain recoverable; automatic deletion/garbage collection is not
implemented. New translated copy has fixture coverage, not human linguistic review.
