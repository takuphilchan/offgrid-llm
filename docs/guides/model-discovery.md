# Model discovery and download recovery

OffGrid is not limited to its curated catalog. Larger models are available to
users with suitable hardware; there is no automatic catalog-sized RAM restriction.
Discovery requires internet access to Hugging Face. Ordinary local inference does not.

## Web and desktop

1. Open **Models**, choose **Language** or **Embeddings**, then **Find more models**.
2. Enter a model or publisher and choose **Search**. Typing alone does not send a query.
3. Choose a repository, then **Choose files**. Only that repository's file list is loaded.
4. Read its model card and license. Select the exact GGUF file/quantization, then Download.

The list displays up to 1,000 repository entries, including nested files. Unknown
sizes are labelled, not presented as zero-byte models. Split weights and multimodal
projectors are identified as requiring companion files; this installer flow does
not pretend one shard/projector is a runnable model. Gated/private repositories
are not supported by the public browser discovery flow. Downloads require the
existing model-management permission; discovery does not grant extra privileges.

Repository/file-derived IDs prevent two publishers' identically named files from
sharing a download or installed identity. Catalog IDs remain unchanged. The active
download remains visible on the Models page even when another search is displayed.

File size is **not** total RAM/VRAM use. Runtime support for the model architecture,
context/KV-cache memory, available disk space, license, and model quality still need
checking. A successful download is not a hardware qualification or publisher endorsement.

## CLI

Start or connect to the OffGrid service first. `list` and `download` are
service-backed operations; they do not silently edit model files behind a running
service. This prevents the UI and CLI from disagreeing about downloads or loaded
models. Use `OFFGRID_SERVER_URL` for a non-default address.

CLI `search` currently queries Hugging Face directly from the client process;
web search and service-managed downloads use the server's network environment.
Check both when diagnosing a Windows/container routing difference.

```sh
offgrid search qwen --limit 5
offgrid search qwen --quant Q8_0 --files --limit 5
offgrid search llama --ram 16
offgrid --json search qwen --limit 5
```

`--files` shows the filenames and individual download commands. Use the repository
and filename actually returned by the search:

```sh
offgrid download owner/repository --file model.Q8_0.gguf
```

`--ram` is an optional rough estimate filter, not a runtime guarantee. Omitting it
does not impose a hardware limit. Search has an overall deadline and at most four
concurrent file-list requests. Invalid options exit 2; upstream failure exits 1;
Ctrl+C exits 130. Search progress goes to stderr; `--json` output stays parseable.
`--all` can include gated repository metadata but does not authenticate gated downloads.
For a container-backed CLI, prepend `docker exec -it offgrid` (omit `-it` when scripting).

## Download phases and recovery

### Speech models from Hugging Face

Speech discovery now has a bounded data-only resolver behind the shared model
client. `speech_recognition` and `speech_generation` searches do not apply the
GGUF-only filter. Repository selection resolves an immutable commit before
offering complete variants. Choose **Models → Speech recognition** or **Speech
generation**, then a curated entry's **Review download**, or explicitly search
Hugging Face and choose a repository and variant. The preview shows the full
package, exact source revision, licenses, download size, required free space and
missing runtime. **Download** installs every declared dependency; users do not
assemble manifests or collect files. Advanced offline folder import stays collapsed.

This milestone installs speech models, **not speech runtimes or voice interfaces**.
Installed, integrity checked, runtime compatible, smoke-tested and qualified remain
independent. A download never enables a microphone or puts speech models in Chat.

Supported recipes include English whisper.cpp `ggml-*.en.bin`, Piper ONNX with
its matching JSON configuration and voice model card, streaming Zipformer with
matching encoder/decoder/joiner plus tokens, sherpa-compatible Kokoro with
voices/tokens/license/eSpeak data, and the official Qwen3-ASR/Qwen3-TTS
Transformers layouts. Qwen packages accept only declarative safetensors,
configuration, tokenizer and codec assets; repository Python, pickle and
executable files are never downloaded or run. The initial Qwen profiles expose
English metadata and require their pinned OffGrid runtime adapters. Additional
ASR/TTS families are added through the same reviewed recipe registry rather
than by guessing from a filename. Incomplete, ambiguous or unrecognized layouts
are not installable. Search results are discovery leads, not compatibility or
quality certification.

Resolution downloads bounded metadata and small configuration/token/phonemizer
data only. It checks their immutable Git object identity before recording SHA-256.
Weights require upstream SHA-256/size metadata and are not fetched during search
or resolution. All required artifacts receive a pinned source and digest. The
resolver never executes repository scripts, installs dependencies, or unpickles
files. A missing digest, invalid pagination, changed identity, or missing dependency
is an actionable resolution failure, not a partially installable package.

Limits are 32 metadata pages, 20,000 repository entries, 16 MiB of metadata,
8 MiB per small resolved data file, 64 MiB aggregate data, and three minutes per
resolution. Installation still requires an explicit Download action. Public,
ungated Hugging Face repositories are supported; URL credentials/private access
are not. HTTPS redirects are limited to reviewed Hugging Face distribution hosts.
Existing proxy environment settings are respected; resolution does not restart
WSL, disable a VPN, or alter system routing.

Package downloads use durable operation IDs rather than simulated `.gguf` filenames.
The Models page restores their snapshots after navigation or reconnect without
submitting again. **Downloading → Verifying → Installing → Installed** separates
received bytes from publication. Expand **Package files** for individual progress.
Cancellation first enters **Stopping**, then settles; partial bytes are kept and
reported. **Resume** rechecks complete artifacts and continues the original pinned
source. A changed upstream default branch cannot replace the approved revision.
Restart never automatically downloads. **Discard partial data** confirms cleanup
of only that operation's staging/recovery data; installed models remain.

**Repair** downloads the exact source recorded during installation into separate
staging, verifies it, and preserves the previous package until publication is
durable. Leased/in-use packages cannot be replaced. Imported packages without a
trusted installation receipt require re-import. **Delete** removes only the chosen
revision and refuses competing/unfinished operations; cancel and discard those
first. User recordings, documents and original import folders are never removed.

### Service-backed package CLI and Python

```sh
offgrid model catalog --category asr --json
offgrid model search whisper --category asr --json
offgrid model discover ggerganov/whisper.cpp --json
offgrid model preview <catalog-id> --json
offgrid model install <catalog-id> --yes --detach --request-id <unique-id> --json
offgrid model status <operation-id> --json
offgrid model cancel <operation-id> --json
offgrid model resume <operation-id> --detach --json
offgrid model repair <completed-operation-id> --yes --detach --json
offgrid model discard <operation-id> --yes --json
offgrid model remove <package-id> --revision <revision> --yes --json
```

Use actual IDs returned by catalog/discovery. Repository preview/install additionally
requires `--revision`, `--variant` and `--architecture` from discovery. `asr`/`tts`
alias `speech_recognition`/`speech_generation`; `language` and `embeddings` expose
the typed legacy inventory too. All `model` commands use the authenticated service
client, so they share its network, storage, permissions and operations. Existing
`list`, `search`, `download` and audio commands retain their older behavior.

Without `--detach`, install/repair/resume poll status. Progress and license notices
go to stderr; stdout is JSON. Ctrl+C requests cancellation and exits 130; inspect
status if the service could not acknowledge it. Usage errors exit 2 and operational
failures 1. Identical request IDs deduplicate accepted source-bound work; changed
sources conflict. A fresh preview of the same immutable package can recover the
original operation, but never restarts it. See [Python examples](../../python/README.md#speech-model-packages).

- **Downloading**: transfer bytes and speed; Cancel keeps the partial file.
- **Preparing model**: bytes have arrived; OffGrid closes the file, promotes it,
  and refreshes its registry. 100% transferred does not mean the model is ready yet.
- **Ready**: the installed list has been refreshed.
- **Download interrupted**: an error and, when bytes were retained, Resume.

Download identity is the local model ID plus its repository and exact source file.
Resume preserves that identity, including a pending knowledge-enablement request.
OffGrid rejects reuse of retained partial bytes for a different source. Cancel
waits for the download worker to release its file before Resume becomes available.

On Windows the Hugging Face downloader now flushes and closes its own handle before
renaming `.gguf.tmp`. Brief sharing/lock violations receive bounded retries. A
persistent lock or permission/storage failure leaves the partial data available
for inspection/retry and never deletes the existing destination to force a rename.
Close applications using the file and choose Resume (or repeat the CLI command).
Do not disable antivirus or delete all model data to work around a file lock.

Source changes are not installed updates: already-running desktop apps/containers
need a separately built and explicitly applied package/image before these fixes appear.
