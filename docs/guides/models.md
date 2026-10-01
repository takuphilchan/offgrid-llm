# Choose and manage models

Start with [model discovery and download recovery](model-discovery.md) for the
web/desktop installer. This guide explains model roles, storage, and verification;
it is not a ranked list of current model recommendations.

## Choose for the workflow

| Workflow | What the model/runtime must support |
| --- | --- |
| Conversation or drafting | Instruction/chat generation and the correct template |
| Document retrieval | A separate compatible embedding model, pooling, and dimensions |
| Agent tools | Reliable structured tool calls, not just fluent chat |
| Managed-browser images | Compatible model/projector pair, image transport, and preflight |
| Native desktop control | The implemented driver and typed tools; a vision model does not add native capture |

Check the publisher's model card, license, language coverage, and architecture.
A filename, parameter count, or successful download is not compatibility evidence.
Evaluate tool calling and useful outcomes separately from text generation.

## Fit memory and storage

Model file size is only part of memory use. Context, KV cache, image processing,
runtime buffers, and the operating system also need room. Increasing context or
loading embeddings alongside chat can change what fits.

Quantization trades weight size against accuracy; a smaller quantization is not
automatically suitable for tool calling. Start with a modest context and check
actual runtime allocation/offload. See [performance](../advanced/PERFORMANCE.md)
and [CPU support](../advanced/cpu-support.md). Hardware recommendations must stay
within the [recorded qualification profiles](../advanced/product-reliability-plan.md).

## Download into the correct workspace

With the intended service running:

```sh
offgrid list --catalog
offgrid download phi-3.5-mini-instruct
offgrid list
```

The catalog example is not a promise of agent or vision quality. For other models,
use **Models → Find more models** or the [Hugging Face guide](huggingface.md).
Choose an exact supported GGUF file. Split weights/projectors need their companion
files; the installer does not treat a single shard as a complete model.

Use the installed ID returned by the service. Native default storage is
`~/.offgrid-llm/models`; containers normally use `/var/lib/offgrid/models`.
`OFFGRID_MODELS_DIR` selects the service's actual root. A second empty model list
may mean a different workspace, not a failed download.

## Verify what you installed

1. Wait for **Ready**, not merely 100% transferred.
2. Inspect the installed model entry and file identity.
3. Run a short chat request and check model/runtime errors.
4. Test the intended embedding, tool, or image workflow separately.
5. Retain source, license, checksums, runtime revision, template, context, and
   hardware settings when recording reproducible results.

The registry can recognize some legacy file extensions, but recognition is not
a guarantee that the current llama.cpp runtime can load an old GGML/BIN format.
Use a compatible GGUF for current workflows; do not rename an incompatible file.

## Offline transfer and backups

Prepare the matching application, UI, runtime, models, and supporting files on a
connected machine, verify digests after transfer, and test with networking off.
See [offline deployment](../advanced/DEPLOYMENT.md#prepare-an-offline-installation).

Workspace backup covers the data root, not automatically the separate model
directory. Preserve models and licenses separately. Local-file import/export
commands must use the intended host paths; they do not magically copy into a
different container's volumes. Do not overwrite a model currently in use or
delete partial download files while the worker owns them.

## Troubleshoot

| Symptom | Next check |
| --- | --- |
| Model absent | Confirm service address, workspace, installed ID, and completed download |
| Loading fails | Check architecture, runtime revision, file integrity, and available memory |
| Chat works but tools fail | Check template and structured tool calling; reasoning style is not a repair |
| Knowledge will not enable | Select an embedding model, not a chat model; inspect index/runtime compatibility |
| Interrupted download | Resume the same source/file through Models; do not replace partial bytes with another model |
| GPU detected but slow | Verify actual offloaded layers and context allocation, not just device detection |
| File copied manually but not listed | Inspect the service's model root; a deliberate rescan/restart may be needed |

Keep installed files and sources when investigating; deleting the whole models
directory is not routine repair. For CLI/local-path differences, see
[CLI reference](../reference/cli.md#terminal-chat-and-local-model-files).
