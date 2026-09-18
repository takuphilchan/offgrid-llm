# Model discovery and download recovery

OffGrid is not limited to its curated catalog. Larger models are available to
users with suitable hardware; there is no automatic catalog-sized RAM restriction.
Discovery requires internet access to Hugging Face. Ordinary local inference does not.

## Web and desktop

1. Open **Models → Find more models**.
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
