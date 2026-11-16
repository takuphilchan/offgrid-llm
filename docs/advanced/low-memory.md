# Memory and disk use

Model file size is not total runtime memory. Weights, context/KV caches, compute
buffers, embedding or voice workers, the desktop renderer and other applications
all need space. A smaller quantized model may work on a constrained machine;
installation alone does not qualify it for that machine or for reliable agent work.
There is no fixed response-time guarantee for a RAM size.

## Runtime safeguards

Before a llama.cpp model load, OffGrid rechecks available host RAM and GPU memory.
On Windows it also measures commit headroom: free physical RAM does not guarantee
that Windows can commit another allocation. CPU budgets use the smaller available
pool, and GPU loads must retain host commit headroom. A rejected load preserves the
requested model and context; it does not silently lower quality or discard history.
These are conservative estimates, not a hard process-memory cap or proof against
all allocation failures. Other applications can consume memory after measurement.

Speculative prewarming has one active operation, a 1 MiB read buffer, a shared
512 MiB maximum per batch, a 30-second deadline and at most 32 retained status
records. Its budget also leaves at least 1 GiB or 10% of host RAM as reserve and
uses at most one quarter of the remaining headroom. Pressure is checked between
reads. Model loading pauses speculative reads, including a pause/resume cycle that
happens between reads. Partial reads cannot claim the full model was warmed.
The OS controls page-cache eviction; prewarming does not pin RAM.

System-memory measurement failure is reported as unknown. Go heap statistics are
not substituted for total PC RAM. macOS availability is measured instead of
assuming 80% of installed memory is free.

## Explicit operator choices

`OFFGRID_MAX_MODELS=1` limits llama model-cache residency to one model. Existing
configuration is preserved. This is not a total count of desktop, speech and
embedding processes and is not a total application RAM limit.

`OFFGRID_LOW_MEMORY=true` opts into one cached model, batch size 128, one inference
slot and q4_0 KV cache. Runtime support for that cache format still matters. It does
not silently shorten the configured context. Choose a smaller model or the
Interactive response context explicitly when the current allocation does not fit.
Quit OffGrid from its tray menu to stop its owned service; merely closing its main
window intentionally leaves quick access available.

## Disk cleanup is separate

Old installers, preview builds, development caches and rollback snapshots consume
disk space. They do not themselves stay in model RAM. A nearly full Windows system
drive can prevent a system-managed page file from expanding. Do not disable the
page file to reduce disk usage.

Before relocating rollback snapshots, confirm they are inactive, copy to a chosen
archive drive, verify file hashes, and retain a manifest before removing the source
copy. Keep the matching application/data snapshot for rollback. Do not delete
conversation databases, model files, credentials, originals or the current desktop
profile as cache cleanup. Previously exported files and backups have their own
retention; deleting a live document does not delete those copies.

The repeated local development installations can create several historical backup
folders; ordinary runtime cache clearing is not a backup-retention policy. Build
temporary directories and rollback copies should use an explicitly selected drive
with sufficient capacity. Never automatically prune unique recovery snapshots.

Windows commit semantics: [Microsoft MEMORYSTATUSEX documentation](https://learn.microsoft.com/en-us/windows/win32/api/sysinfoapi/ns-sysinfoapi-memorystatusex).
