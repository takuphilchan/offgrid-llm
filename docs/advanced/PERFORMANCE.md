# Inference performance

Separate cold model loading, prompt processing and decoding when measuring
performance. No fixed first-token latency or GPU speedup is guaranteed.

## Streaming saved conversations

Web and Electron share the chat renderer. Saved chat now streams by default,
with queue, retrieval, loading, prompt-processing, generation and saving stages.
Stop cancels generation, including queue waits. Text is provisional until the
server confirms persistence. Broken streams retain the draft and label partial
output as unsaved. Reload before retrying an uncertain final save.

Response settings offer Interactive and Extended context, and output budgets
of 256, 1024 (default) or 4096 tokens. Interactive allocates the smaller of
`OFFGRID_CHAT_CONTEXT` (8192 by default) and the service's effective context.
Extended uses the service context. Profile changes wait for active inference
and may reload the model. No history is silently removed: use Extended or a
new conversation if a prompt exceeds its context.

External agents and `/v1/chat/completions` retain the service context. Set
`OFFGRID_MAX_CONTEXT=65536`, `OFFGRID_CHAT_CONTEXT=8192`,
`OFFGRID_ADAPTIVE_CONTEXT=false` and `OFFGRID_MAX_MODELS=1` to separate
interactive chat from a deliberately configured 65K agent service. Only choose
65K if your model and memory support it. Disabling adaptation does not create
memory. Health's `inference.context_window` shows the admitted profile; public
model discovery describes the context allocated to public agent API requests.

## NVIDIA GPU containers

You need a CUDA-built inference binary, GPU device access, and GPU offloading
enabled. A GPU flag on the CPU image is insufficient.

```bash
docker run --rm --gpus all nvidia/cuda:12.8.1-base-ubuntu24.04 nvidia-smi
docker buildx build --load -f docker/Dockerfile.gpu --build-arg VERSION=local-gpu -t offgrid-llm:local-gpu .
```

Run the image with `--gpus all`, loopback port publishing and your existing
model/data volumes. The GPU Compose example is `docker/docker-compose.gpu.yml`.
Back up data and stop the old service before replacement. Never run two servers
against one writable data volume or delete volumes to update an image.

The GPU image enables `OFFGRID_ENABLE_GPU=true`. `OFFGRID_GPU_LAYERS=0` means
automatic fitting with the requested context as a floor; a positive value
explicitly requests that many layers. Do not publish architecture-specific local
CUDA builds as universal images.

Verify CUDA initialization and offloaded layer counts in container logs during
real generation, plus VRAM usage from `docker exec offgrid nvidia-smi`.
An idle GPU percentage or successful hardware probe alone is not proof of
accelerated inference. Large contexts on small GPUs may require partial offload.

## Memory and tuning

GGUF size is not total memory: account for KV cache, compute buffers, embeddings
and the OS. OffGrid no longer increases residency using average model file size.
Use `OFFGRID_MAX_MODELS=1` on constrained machines; keep one inference slot
until throughput and memory have been measured.

WSL memory is a ceiling shared by processes and containers. Increasing it
blindly on a 16 GB Windows machine can starve Windows. Inspect `free -h`,
`vmstat 1`, and `docker stats`. Persistent swap-in/out indicates pressure;
occupied swap alone does not prove thrashing. Prefer suitable models/context
and verified GPU placement over more swap or forced mlock.

Advanced startup settings include `OFFGRID_NUM_THREADS`,
`OFFGRID_BATCH_SIZE`, `OFFGRID_KV_CACHE_TYPE` (f16, q8_0, q4_0) and
`OFFGRID_FLASH_ATTENTION`. Benchmark changes individually: more threads and
smaller batches are not always faster; aggressive KV quantization affects quality.

## Verification

Chat reports first-text latency, context and throughput when the backend returns
actual token usage. SSE timings are documented in [the API reference](../reference/api.md).
Streaming improves time to visible output, not computation speed itself.

Compare identical models, prompts, output budgets, profiles and cache states.
Record cold loading separately from several warm runs. Test short chat, long
prompts, retrieval, cancellation and agents. Avoid concurrent builds/downloads
while benchmarking. Inspect runtime logs if output stops midway; never treat a
partial answer as completed or blindly resubmit an uncertain final save.
