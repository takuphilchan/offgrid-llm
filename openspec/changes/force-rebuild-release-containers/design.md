# Design: Force-rebuild release containers

The `workflow_dispatch` interface will gain an optional immutable `source_ref`
and a `force_rebuild` boolean. A forced dispatch must provide a `vX.Y.Z` tag
whose version exactly matches the normalized image version. The resolver will
reject invalid or mismatched input before image jobs run.

The application checkout remains pinned to `source_ref`. For an explicit
workflow-dispatch tag build, CPU and GPU Dockerfiles are overlaid from the
reviewed automation revision (`github.sha`), matching the existing repair-branch
boundary. The image revision label remains the source tag commit; the packaging
revision label records the workflow revision.

The existing reuse check already has a force-rebuild switch. Exposing it for
manual dispatch bypasses reuse, allowing the CPU architecture matrix and NVIDIA
job to run. The existing manifest dependency and final source/platform verifier
remain authoritative. A failed architecture build cannot publish a manifest.

Ordinary dispatches without a forced tag continue to target the current ref and
retain their existing reuse behavior. The versioned Docker tags and eligible
latest/minor aliases may be republished, but the Git tag and GitHub release are
not modified.
