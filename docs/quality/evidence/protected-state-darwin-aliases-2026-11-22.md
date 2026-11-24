# Darwin protected-state case aliases — 2026-11-22

Status: implementation and cross-compilation evidence only. Native macOS
qualification remains incomplete; no Mac was available for a before/after run.

## Finding and implementation

The previous non-Windows helper used Go `filepath.EvalSymlinks`, preserving lookup
spelling, followed by case-sensitive lexical containment. On a case-insensitive
Darwin volume, differently cased paths can identify the same directory. Persisted
folder identity alone does not repair overlap checks based on those path strings.
This finding came from source inspection, not a reproduced macOS test failure.

Darwin now uses a separate `canonicalPrivateLocation`: open a directory handle,
request `F_GETPATH` into a bounded `unix.PathMax` buffer, require a valid absolute
clean path, and compare its device/inode/birth-time identity with the open handle.
The original input must still name that handle's object. The integer-argument
`FcntlInt` wrapper receives an explicitly pinned buffer. Missing open components
retain their native error for pending-root traversal; unsupported canonicalization
fails closed. Windows volume-GUID handling is unchanged. Linux and other systems
retain their existing helper.

[Apple's XNU F_GETPATH implementation](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/kern_descrip.c)
retrieves a bounded vnode path. The implementation also follows the
[x/sys Darwin fcntl wrapper](https://cs.opensource.google/go/x/sys/+/v0.47.0:unix/fcntl_darwin.go).
These sources explain the chosen API; they do not establish runtime qualification.

## Verification and remaining qualification

Cross-compilation uses `CGO_ENABLED=0`, `GOOS=darwin`, and both `GOARCH=arm64` and
`GOARCH=amd64` with `go test -c ./internal/execution`. This proves compilation of
the native-path helper and platform tests, not execution of Darwin syscalls or
qualification of the separate cgo ACL implementation.

Actual results:

- Final arm64 and amd64 cross-compilations passed (exit 0), with logs
  `build/private-state-darwin-alias-{arm64,amd64}-cross-final-20261122.log`.
- Windows existing protected-host regression passed in 0.966s: five top-level
  tests and eleven alias subcases passed; the symlink fixture explicitly skipped
  for unavailable Windows privileges. Log:
  `build/private-state-darwin-change-windows-final-20261122.log`.
- An earlier Windows selection used the Darwin-only test prefix and ran zero
  tests. Its retained `build/private-state-darwin-change-windows-regression-20261122.log`
  is not regression evidence; the corrected selection above was then run.
- Documentation validation checked 503 maintained Markdown files with no
  problems; the source-size ratchet and scoped whitespace check passed.

On a Mac, run:

```sh
go test ./internal/execution -run 'TestPrivateStateDarwin' -count=1 -v
```

Run on both case-insensitive and case-sensitive volumes by selecting an isolated
`TMPDIR` on each volume. Tests prove actual alias identity with `os.SameFile`
before exercising equal, ancestor, descendant, and missing-tail rejection;
they also check registry location identity, disjoint siblings, missing paths,
regular-file rejection, and symlink resolution. The complementary test requires
distinct case-sensitive directories to remain disjoint. Filesystem-specific
skips are explicit and are not passing evidence for the skipped volume behavior.

The pending-reservation regression additionally creates a missing parent using
different capitalization after its location was recorded. Registry admission
must re-resolve that pending path and retain its fence when canonical spelling
changes; validating only the old existing ancestor cannot establish that fact.
That regression also requires native Darwin execution and remains unqualified.

No installed data, personal browser session, native UI, or global filesystem
configuration is modified. Tests create and remove only their own temporary
directories. This increment does not close macOS ACL/confidentiality, storage
durability, or browser integrity qualification requirements.
