# Complete vision model downloads

## Why

The user reported image rejection after downloading NVIDIA Nemotron 3.5 Bolt E4B UD. The installed GGUF is present without its projector. The ordinary download handler installs one file, and registry association uses an older hardcoded name list. Installing weights alone must not be presented as complete vision setup.

## What Changes

- Resolve required GGUF companions from the selected repository at one immutable revision, distinguishing projector alternatives from required files and excluding draft/imatrix artifacts.
- Download and verify weights and the selected projector under one durable operation; retain exact source identity for retries and repair.
- Persist model-specific companion bindings instead of adding a Nemotron-specific runtime rule. Refuse ambiguous or incomplete layouts rather than borrowing a similarly named projector from another model.
- Expose missing dependencies and repair in Models; retain image drafts and existing runtime qualification gates.

## Capabilities

### New Capabilities
- `vision-model-downloads`: complete, source-bound GGUF vision acquisition and explicit repair.

## Impact

Model discovery/download/registry, CLI service clients, shared Models UI, image readiness and recovery tests. Linked to workspace-outcome-experience image input. No automatic model switch, arbitrary code download, acquisition triggered by sending an image, or general vision qualification claim.
