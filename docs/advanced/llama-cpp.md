# Local llama.cpp runtime

The default OffGrid service supervises a native `llama-server` process and communicates with it over local HTTP. It is not a mock-response default, and ordinary builds do not require linking the Go program directly against llama.cpp.

## Choose a runtime

Packaged containers include their matching inference runtime. For native installations, runtime selection is implemented by [binary_manager.go](../../internal/inference/binary_manager.go): an explicit executable override, a compatible executable on `PATH`, or the managed runtime installation path. Native CPU/Metal fallback downloads require network access unless already installed. A missing compatible GPU runtime fails with guidance rather than silently qualifying a CPU build as GPU-capable.

To select a custom executable before starting the service:

```powershell
$env:OFFGRID_LLAMA_SERVER_PATH = 'C:\offgrid-runtime\llama-server.exe'
offgrid serve
```

```bash
export OFFGRID_LLAMA_SERVER_PATH=/path/to/llama-server
offgrid serve
```

Use a real absolute path. `OFFGRID_BIN_DIR` selects the managed binary directory. Run the chosen executable's `--version` and `--help` to inspect it; required runtime flags and the model/template must be compatible. Preserve the revision and digest when recording results.

OffGrid owns the workers it starts. Do not also autostart a separate inference server for the same service or terminate an unrelated process because it occupies a port. See [runtime lifecycle](../../internal/inference/lifecycle.go) and [autostart](../setup/autostart.md).

## Validate an installation

1. Check `/api/v2/system` for the actual service version/revision and UI identity.
2. Install a chat model and request a short response.
3. Inspect runtime/model loading failures rather than treating `/health` as proof of inference.
4. For GPU use, check actual offload and device activity during inference; detecting a GPU is not proof that the model uses it.
5. Test embeddings, structured tool calling, and vision separately when those workflows are needed.

[Performance](PERFORMANCE.md) explains context, memory, placement, and latency. [Models](../guides/models.md) explains GGUF discovery; [embeddings](../guides/embeddings.md) covers document retrieval.

## Separate capabilities

Embeddings need a compatible embedding model with supported pooling and dimensions. Hash-derived placeholder vectors from old builds are not semantic retrieval and cannot qualify as a usable fallback. Preserve sources and follow [index recovery](workspace-recovery.md#recovering-indexes-created-with-placeholder-embeddings).

Tool calling depends on the model, template, and effective runtime settings. A model's filename or a successful chat response does not prove it supports the expected tool protocol. Browser/native computer tasks have their own checks and limits.

Vision additionally needs compatible model/projector components and typed image handling. Managed-browser image support does not imply native desktop capture or qualified computer control. See [computer tasks](../guides/computer-tasks.md).

## Development and qualification

The repository retains explicit mock and alternate build-tag implementations for development/testing. Their presence is not proof of the production runtime path. Never use mock output as evidence of inference quality or successful model loading.

The default adapter is [llama_stub.go](../../internal/inference/llama_stub.go), despite its legacy filename; it forwards to the native HTTP engine. [server.go](../../internal/server/server.go) composes the configured engine. Build the ordinary service using the [build guide](BUILDING.md); old `make build-llama` instructions are not the current setup path.

Use the pinned runtime revisions in release build configuration for repeatable builds. Custom upstream builds need their own tests. Consult the [reliability plan](product-reliability-plan.md) for measured profiles and remaining evidence rather than assuming upstream model support qualifies an OffGrid workflow.
