# Design

## Context

See [the proposal](proposal.md) and the authoritative
[recovery guide](../../../docs/advanced/workspace-recovery.md). Current
`internal/storage/backup.go` streams format-1 ZIP into a private temporary file,
checks its inventory/hashes, then publishes with a no-replace hard link. It requires
exclusive ownership of a stopped workspace and rejects unsafe paths and special
files. Limits are 100,000 files, 100 GiB uncompressed data and a 32 MiB manifest.
Restore verifies an isolated fresh directory, including SQLite/WAL and foreign
keys, before platform-specific no-replace publication. Legacy archives contain
credentials in plaintext. Cleanup errors are presently ignored by deferred removal.

`cmd/offgrid/workspace_commands.go` is the only public maintenance surface. Its
Knowledge rebuild first makes a plaintext backup. There is no existing archive
HTTP API, SDK wrapper or administrator recovery UI. The existing distribution
change addresses installer/trust/first-run behavior, not encrypted archives; this
change supplies the missing cohesive M1/M7 archive contract without replacing it.

## Goals / Non-Goals

**Goals:** Preserve one storage implementation, retain legacy compatibility, add
confidential interoperable backups with bounded authentication, and carry the
same behavior through CLI and later trusted administrator controls. Every stage
must be cancellable where the underlying primitive permits it and truthful about
published or retained state.

**Non-Goals:** No new backup database, remote storage provider, network key lookup,
automatic live-service snapshot, automatic migration/update/rollback or task replay.
No promise that encryption authenticates the archive publisher or securely erases
storage. Signed offline installation packs and wider M1/M7 qualification stay with
their program owners. These boundaries do not remove required administrator UI,
cross-platform recovery or real installed-product validation from this change.

## Decisions

### 1. Standard age envelope around the existing ZIP

Use binary age v1 containing the unchanged format-1 ZIP; recommend `.zip.age`.
Pin `filippo.io/age` v1.3.2 after dependency review. Its Go API provides
`Encrypt` and `DecryptReaderAt`, the latter explicitly supports ZIP readers and
authenticates the final chunk/size. Preserve native Hybrid and X25519 key support;
exclude SSH/plugin execution. This reuses a maintained interoperable format rather
than designing an envelope from the repository's existing cryptographic primitives.
[Official API](https://pkg.go.dev/filippo.io/age@v1.3.2)

Do not shell out to an installed age executable in production. Use independently
produced age CLI fixtures for interoperability evidence. The chosen release adds
header/recipient parsing bounds; application limits remain stricter where needed.
[Pinned release](https://github.com/FiloSottile/age/releases/tag/v1.3.2)

### 2. Stream encryption and verify before publication

Add explicit storage options, preserving existing plaintext wrappers. Route ZIP
output through encryption directly into the private ciphertext temporary file.
Close ZIP, close encryption to emit its final tag, sync and close the file, then
perform complete cryptographic and inner-archive validation before no-replace
publication. Any earlier error prevents publication; join cleanup errors rather
than replacing the original cause.

Public-key backup must verify even when the operator holds no decryption key.
Use a small reviewed recipient wrapper which captures the library-generated
ephemeral file key in memory and forwards both ordinary wrapping and any labels
unchanged. Use the library's injected-file-key identity only for this process's
post-write verification. Never persist that key, add an invisible recovery
recipient, alter encryption algorithms or bypass label compatibility. Tests must
cover passphrase-only labels and incompatible recipient types. Clear retained
buffers best effort; Go does not guarantee elimination of every memory copy.
[Recipient interfaces and key handling](https://github.com/FiloSottile/age/blob/v1.3.2/age.go)

### 3. Authenticate all bytes without a decrypted archive file

Open bounded regular input once. For encrypted input, derive a random-access
plaintext reader and authenticate its complete logical byte range before parsing
or staging restore output; random access alone authenticates only visited chunks.
The format authenticates chunk order and final length, so truncation is a failure.
Then construct the shared ZIP reader, retaining strict manifest/hash/CRC and
expanded-size validation. Reject unexpected trailing/concatenated encrypted data.
[Format specification](https://c2sp.org/age)

Refactor archive loading and `readBackupManifest` to accept the common ZIP reader
instead of owning a plaintext file. A closeable archive handle owns its underlying
file and encrypted/plain reader. Context checks wrap reads and copies, including
random-access reads. Do not spool decrypted ZIP to disk as a fallback. Existing
100 GiB data, 100,000 entry and 32 MiB manifest limits remain. Add explicit raw
archive and central-directory metadata limits before potentially large parser
allocations; prove the chosen bounds admit archives produced by the writer and
reject malicious claimed counts, integer overflows and metadata expansion.

Restore extracts into its necessary private plaintext stage only after complete
authentication and inventory validation. Run existing database/version checks
before publication. Retain exclusive creation and platform no-replace behavior;
flush parent-directory metadata where supported. If publication occurred but
durability confirmation failed, report that state and retain the output for
inspection, without deleting it or blindly retrying.

### 4. Explicit CLI modes and bounded secret input

Add repeatable `--recipient`, `--recipients-file`, `--identity`, `--passphrase`
and `--passphrase-stdin` options with command-specific validation. `--passphrase`
is a boolean requesting a no-echo terminal prompt, confirmed for backup/rebuild.
Standard-input mode is explicit and never prompts. No secret-valued argument or
environment variable is accepted. Plaintext existing syntax stays supported and
clearly labeled; specifying encryption parameters selects encryption regardless
of extension. Malformed parameters and secret acquisition fail before mutation.

Set initial limits of 1 MiB per key file, 32 identities or recipients total and
4096 UTF-8 passphrase bytes. Reject excess input without leaking its contents.
For standard input, accept one bounded passphrase with at most a final line ending;
reject NUL/embedded line endings, excess bytes and missing input. Preserve other
whitespace exactly. Interactive readers need an actual bounded no-echo input
implementation, not an unbounded helper followed by a length check. Document safe
dedicated identity storage outside backed-up data; no automatic key generation
or placement in the workspace.

Use scrypt logN18 for new passphrase archives and cap accepted work at18 initially;
larger external archives fail with a safe explanatory error rather than causing
the library's higher default allocation. This is a compatibility limit, not a
weaker fallback. KDF invocation is not context-cancellable: check cancellation
before/after and bound its resource use; never claim instantaneous interruption
or leak an untracked worker after cancellation. Automated backups should use
recipient keys. [Pinned scrypt implementation](https://github.com/FiloSottile/age/blob/v1.3.2/scrypt.go)

Return one machine-readable result retaining existing fields with additive
encryption/verification metadata. Errors must not include secret text, decrypted
content or private-key parser echoes. Never save secret inputs in progress or
support bundles. Verification distinguishes authenticated encrypted archives from
legacy checksum-only ZIP. Public-key encryption permits anyone with a public key
to create ciphertext; retain trusted-source guidance and separate signing claims.

### 5. Share encryption with pre-rebuild recovery

Build backup options once and invoke the same operation for ordinary backup and
`rebuild-knowledge`. Only begin rebuild after verified archive publication. Retain
its recovery archive on rebuild failure; surface the path without secrets. Existing
rebuild ownership and staged-index behavior remain unchanged. Tests inject the
rebuild boundary to prove no mutation follows backup failure, and run existing
isolated rebuild tests to retain actual storage behavior.

### 6. Temporary-state lifecycle and trusted controls

Own only newly created temporary files/stages. On normal failure, remove those
exact paths and report cleanup failures. A crash can leave ciphertext archive
temps or private plaintext restore stages. Recovery inspection must identify
operation-owned state and require an explicit scoped cleanup choice; never
enumerate and delete arbitrary matching directories or claim secure erasure.
Verify Windows ACL and Unix mode behavior on real filesystem fixtures.

After the storage/CLI gate passes, add narrow versioned administrator maintenance
operations to the existing trusted desktop host/IPC boundary. Native destination
selection, ownership handoff, secret acquisition and maintenance process lifetime
belong to the trusted host, not arbitrary renderer commands. The process can
continue while the owned service is stopped and report bounded progress/results.
Use ephemeral operation-scoped authorization, reject untrusted frames and path
substitution, and retain Stop/cancellation outcomes. Externally managed services
are never stopped or mapped to guessed desktop paths. Their UI must truthfully
show the host-local procedure until an authorized maintenance host is available;
unsupported execution and installed-edition qualification remain open tasks.

Renderer work must use the existing design, accessibility, reduced-motion,
keyboard and locale infrastructure. Keep secrets out of retained state, telemetry,
screenshots and support bundles. No generic filesystem bridge or live unaudited
HTTP backup route is introduced. Any later public host API/SDK contract is reviewed
and versioned as part of that integration, not inferred from this CLI addition.

### 7. Owned service drain and private maintenance protocol

The existing desktop owner sends `child.kill('SIGTERM')` on shutdown. On Windows,
Node forcefully terminates a child for that signal; it cannot establish a drained
workspace. Extend that same owner with explicit `serve --desktop-owned-stdio`:
its private stdin remaining open keeps the parent lifetime attached, and EOF
cancels the existing application context. `app.Runtime.Run` remains the only
drain/cleanup owner. Exit zero acknowledges that normal transport/worker/resource
cleanup completed; nonzero/signal/timeout is unconfirmed and prevents maintenance.
No new HTTP shutdown endpoint, forced-kill fallback followed by backup, or second
shutdown executor is added. Ordinary `serve` retains its current behavior.
[Node child-process signal semantics](https://nodejs.org/api/child_process.html#subprocesskillsignal)

Use a separate explicit `workspace --desktop-stdio` maintenance invocation with
version-1 length-prefixed JSON frames over private stdin/stdout. A start frame is
limited to 2 MiB, contains a unique operation ID, one backup/verify/restore request
and bounded ephemeral native-key/passphrase inputs. Subsequent input accepts only
a bounded matching cancellation frame; EOF also cancels. Unknown fields/versions,
duplicate starts, mismatched IDs and oversized frames fail closed. The maintenance
process invokes existing storage options and ownership, never shell commands.
Context-aware reads terminate with the operation rather than leaking a blocked
reader goroutine. Cancellation waits for bounded noninterruptible KDF work when
necessary; an unacknowledged process stop stays uncertain.

Output frames are compact and bounded: operation/phase, terminal outcome, encryption
and authentication state, file count/byte count/application version, publication
state, and retained-path diagnostics when necessary. Do not send the complete
manifest or contents to the renderer. An error after publication retains the
published summary and reports that state. Phase progress never invents a percentage
or physical durability result. Raw errors, input frames, private keys and secrets
are neither logged nor persisted. Full manifests remain inside the archive.

### 8. Operation-bound desktop authority

The existing main-frame sender check gates new recovery IPC. Before selecting any
paths, require a ready owned child, matching service/workspace/build identity and
current administrator principal. Native dialogs choose the archive, destination
and native key files; an isolated host-owned dialog obtains passphrases and confirms
new ones. Service-rendered content receives no secret input API. Renderer requests
contain only documented operation/encryption enums and issued operation IDs.

Retain one operation at a time. Pin canonical filesystem identities, the owned
service process/binary and actor/workspace identity across asynchronous pickers and
confirmation. Recheck before drain and before dispatch; navigation, account changes,
lock/suspend, shutdown and stale tokens cancel pending authority. Stop computer,
execution and terminal owners before service drain, and refuse a failed handoff.
Never stop external services or infer their data paths from reported metadata.
[Electron IPC sender guidance](https://www.electronjs.org/docs/latest/tutorial/security#17-validate-the-sender-of-all-ipc-messages)

The currently loaded UI can observe/cancel an already admitted maintenance operation
while its owned service is stopped. These narrow status/cancel channels do not
grant new offline work. Resume is an explicit action for that original owned
workspace, guarded against replacement owners. Restore publishes only a new checked
directory; it never activates that directory, restarts it or replays tasks. Crashes
leave no reusable consent/secret state; retained archive/stage inspection stays
explicit. Added optional bridge capability is versioned without widening legacy
methods or exposing generic filesystem/process operations.

## Risks / Trade-offs

- Extra authentication/read passes cost I/O -> measure fixture and representative
  workspace performance; bounded memory and confidentiality take precedence over
  eliminating this verification pass.
- KDF memory and cancellation latency -> fixed reviewed ceiling, key-based
  automation and explicit measured limits; no background abandoned derivations.
- Random-access decryption caches plaintext in memory -> bounded chunk lifetime,
  no disk spool, whole-stream authentication and no perfect-zeroization claim.
- Source filesystem races and interrupted cleanup -> retain ownership plus existing
  per-file change checks, expand fault tests and document exact retained state.
- Dependency changes -> review module/sum differences and licenses, preserve the
  current Go toolchain unless compatibility actually requires a separate decision,
  and run project checks rather than assuming release metadata proves support.
- Native host permissions and installed recovery differ by platform -> qualify
  Windows, Linux and macOS separately using isolated state; unavailable machines
  remain explicit gates.

## Migration Plan

This adds archive transport capability without a workspace schema migration.
Existing ZIP archives and callers remain usable. New encrypted archives require
a compatible decrypting version and the selected key/passphrase. Rolling back the
binary never rewrites encrypted archives, discards keys or restores data over newer
work. Update maintained recovery guidance only as each behavior becomes implemented,
and preserve original plaintext and failure evidence. No live installation or
personal-workspace qualification runs are part of source implementation authorization.
