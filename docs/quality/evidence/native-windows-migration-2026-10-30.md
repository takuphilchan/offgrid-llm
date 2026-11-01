# Native Windows workspace migration — 2026-10-30

Status: migrated to the desktop-owned native Windows service; original container
and backups retained. Native GPU replies, streaming, embeddings, workspace
inventory and desktop quit/relaunch were verified. No laptop reboot was performed.

## Approved scope

The user selected migration from the existing Ubuntu/Docker workspace to native
Windows, with backup and verification, and explicitly required GPU acceleration
to be retained. The user then approved launching Microsoft's signed C++ Build
Tools installer and personally handling the Windows administrator prompt.
No driver replacement, automatic reboot, model download, Git publication or
release is included. Existing container and data remain rollback assets.

## Preservation evidence

- Source container: `b7b9951de2ff`, `offgrid-llm:local-ui-compact-20261030`.
- Source volumes: `offgrid-data` and `offgrid-models`.
- Verified idle before snapshot: no active/queued inference, no connected file
  executors or desktop sessions, and no running/queued tasks. No agent schedules.
- Stopped container for consistent data copy, then restarted the same container.
- Source snapshot and private container configuration:
  `/home/phil/offgrid-native-migration-20261030`.
- Windows staging: `%LOCALAPPDATA%/OffGrid-migrations/20261030-native`.
- Workspace archive SHA-256:
  `58030d51681d9831f5dea5b30aa9508fde8890d2687101059dbd6f701ffef876`.
- Windows `offgrid workspace verify` and `workspace restore`: passed, 423 files,
  13,804,788 bytes, application 0.4.21. Restore checked SQLite integrity and left
  original data unchanged.
- All 26 model-tree files copied and independently hashed on source and Windows:
  **17,059,596,685 bytes**, exact path/size/SHA-256 match. No source file removed.
- Baseline API inventory retained privately: 5 models, 77 conversations, 284
  tasks, 2 profiles. An isolated Windows service on port 11612 matched workspace
  identity, every model/conversation/task/profile identity, profile revisions and
  saved task statuses. Document inventory and Knowledge status also matched
  (enabled `bge-m3`, zero documents/chunks). This catalog-only preview ran against
  restored data, performed no inference or desktop actions, and was stopped after
  inspection. Production cutover and GPU checks remain pending.

## Native runtime preparation

The published Windows CPU bundle checksum matched the release checksum
`09a4e9c98ec1552e9c86b86eebf0770c7cee9bb26d332a41147ae7d71167ea68`.
Its `llama-server.exe` failed with Windows status `0xc0000135`: imports included
`libssl-3-x64.dll` and `libcrypto-3-x64.dll`, absent from the bundle. Supplying
the installed Git OpenSSL 3 DLL pair in the staging directory allowed `--version`
to report `b95502b-offgrid-seq2`. This is diagnostic evidence, not activation of
CPU inference or a published packaging fix.

GPU runtime build uses exact upstream commit
`b95502ba9aa0eb73a2f4fc8878d7fbe6a847a0b9` from the locally cached reviewed source,
cloned into `build/native-windows-gpu-cached` and patched with the repository's
`runtimes/llama-cpp/apply-patches.sh`. The main repository is not used as a build
checkout. Build output: `build/native-windows-gpu-build`.

Microsoft Build Tools 2022 installation completed with exit 0. The bootstrapper
had a valid Microsoft Authenticode signature. Selected compiler, Windows SDK
and CMake components only. The detected compiler is MSVC 19.44.35229.0.

NVIDIA CUDA 12.9.1 redistributable components (`cuda_nvcc`, `cuda_cudart`,
`cuda_cccl`, `libcublas`) were obtained from NVIDIA and verified against the
official redistributable manifest's SHA-256 and sizes. They are extracted under
the migration staging directory; no GPU driver was installed. NVCC 12.9.86 runs.
Existing Windows driver 610.60 detects the RTX 5060 Laptop GPU, 8151 MiB.

Build configuration selects `120a-real`, CUDA, the OffGrid sequencing patch,
static ggml/llama libraries, and disables worker HTTPS/OpenSSL and worker CURL
dependencies. OffGrid's normal authenticated service/network handling is not
changed. Build targets are `llama-server` and `test-chat-auto-parser`, parallel 3.

The first MSVC build exposed ambiguous `autoparser` namespace/class lookup in
OffGrid's regression header. Explicit global namespace qualification fixed the
test compilation; runtime/parser behavior was not changed. Rebuild succeeded.
Full native `test-chat-auto-parser.exe` from the source checkout passed **117
tests, 583 assertions, zero failures/exceptions/skips**. Patch admission tests
passed 3/3 under WSL. Upstream's optional embedded UI download timed out; the
worker built without that UI. OffGrid uses its own installed renderer.

Staged and deployed runtime reports `b95502b-offgrid-seq2`, MSVC 19.44.35229.0,
and CUDA0 RTX 5060 Laptop GPU (8150 MiB). CUDA and MSVC runtime DLLs are app-local;
the NVIDIA driver remains system-owned. `dumpbin /dependents` confirms no OpenSSL
DLL dependency. Runtime/package file digests are retained privately in
`gpu-runtime/verified-runtime.json`. This GPU binary targets this machine's
Blackwell architecture; it is not a generally qualified Windows GPU release.

## Native model checks

Isolated Windows service used restored data on port 11612, then was stopped.
All model files were independently rehashed after testing: all 26 source/native
files still matched. Actual native CUDA worker processes were observed through
`nvidia-smi`; chat replies were required, not just successful model discovery.

| Check | Observed result |
| --- | --- |
| Phi 3.5 Mini reply | Passed, 11.487 s including load |
| Qwen 2.5 3B reply | Passed, 11.392 s including load |
| Meta-Llama 3.1 8B reply | Passed, 26.036 s including load |
| Qwen 2.5 7B reply | Passed, 16.386 s including load |
| Resident Qwen streaming | Passed, 13 HTTP body chunks, terminal DONE, 0.254 s total |
| BGE-M3 embeddings | Passed, 1024 dimensions |

These are individual local smoke checks, not latency guarantees or full agent
workflow qualification. After cutover, Meta-Llama and Qwen replies passed again
at their final installed paths (17.673 s and 11.137 s including load), followed
by streaming and embedding checks. Private records are
`gpu-replies-private.json` and `gpu-cutover-checks-private.json`.

## Cutover and restart

- Rechecked idle inference, tasks and host/computer sessions before stopping the
  original container. Took a fresh stopped snapshot so the first snapshot was
  not blindly reused.
- Final archive `final-workspace.zip`: 423 files, 13,804,787 bytes; Windows archive
  verification and SQLite restore passed. SHA-256:
  `c534fc5b8589dda0c438a4ae8a79fe92c7cd1f92f7f2727a5568e0330d119c60`.
- Preserved previous Windows data/models and settings in
  `%LOCALAPPDATA%/OffGrid-migrations/20261030-native/previous-native-workspace`.
  Every move is recorded in `cutover-moves.json`. No original container, volume
  or backup was deleted.
- Active data/models now live in `%USERPROFILE%/.offgrid-llm/data` and `models`.
  Verified GPU runtime and dependencies live in `bin`. Configuration preserves
  GPU operation, q8_0 cache, the 65536 maximum and 8192 chat context setting.
  Eager whole-library warming is disabled to avoid consuming startup resources.
- Only the known external-ownership record for port 11611 was moved into backup.
  Desktop presentation, account and computer permission files were retained.
  User environment pins the installed runtime and the existing opted-in
  `llama31-optional-tools-v1` adapter; previous values were backed up.
- The installed desktop started its own bundled Windows `offgrid.exe`, confirmed
  by parent PID and port ownership. Native inventory matched all 5 legacy
  model IDs, 77 conversations, 284 task IDs/statuses and both profile revisions.
- Automatic approval review blocked the final normal quit command. The user
  quit through the app menu. Port 11611, desktop service and inference workers
  disappeared. Relaunch started one desktop-owned native service and the same
  inventory was verified again.
- The Windows service remained available during an approved WSL shutdown. The
  old container is `exited`, with its original `unless-stopped` restart policy.

## WSL access and rollback

The user separately approved mirrored networking and a WSL restart to preserve
the existing WSL CLI's localhost endpoint. The previous `.wslconfig` is retained
as `previous-wslconfig` in the migration backup. Only `networkingMode=mirrored`
was added under `[wsl2]`; resource settings were retained. No listener was opened
on `0.0.0.0`, no firewall exception was added and no agent credentials/settings
were rewritten. Windows 11 build 26300 and WSL 3.0.3 report mirrored mode; WSL
health and `offgrid model list` reach the native service. The catalog includes
the 5 legacy models and the 2 preserved speech model packages.
A fresh request from WSL after desktop relaunch returned Qwen's greeting through
the native Windows service in 11.981 s including model load; the workspace ID
matched. Results are retained in `wsl-native-verified.json`. The temporary WSL
keepalive ended during shutdown and is no longer needed by the Windows desktop.
Microsoft documents this localhost behavior in its
[WSL networking guide](https://learn.microsoft.com/en-us/windows/wsl/networking).

Rollback is deliberate: quit desktop, back up any **new** native work first, then
restore the saved Windows folders/config/environment and external-ownership
record before starting the original container. The container's snapshot will
not include work created after migration. Restore `previous-wslconfig` and restart
WSL only if reverting the network change too. Never run two services against one
writable data directory or treat rollback as a merge.

No real laptop reboot, macOS/Linux package qualification, speech workflow,
desktop-control workflow or full Hermes/OpenClaw task suite was performed here.
Speech package files are preserved, but their separate runtime/qualification
requirements remain. The published CPU bundle's missing dependency remains a
separate release-packaging issue; this installation uses the verified native GPU
runtime. No commit, push, release or model download was performed.
