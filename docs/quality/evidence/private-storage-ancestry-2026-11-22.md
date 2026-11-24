# Private storage ancestor authority — 2026-11-22

This increment addresses the ancestor replacement gap in M2.07 protected browser
storage. It does not complete the browser transfer or platform qualification gates.
The reusable `execution.CheckPrivateAncestry` checks existing directory components
before private leaf creation, without changing existing permissions. The native
storage-only checker invokes it before creating storage.

## Reproduced failure and implementation

The isolated Windows regression creates a new private browser directory, then
grants Everyone `DELETE_CHILD` on its newly created parent without changing the
child's private DACL. The previous native checker accepted that tree. Actual
before result: **FAIL, 0.454 seconds**, retained in
`build/browser-storage-ancestry-before-2-20261122.log`.

Windows now checks owners and effective DACL allow entries along the path.
Untrusted delete-self, delete-child, ACL/owner rewriting and relevant write rights
reject admission. Read/traverse entries and inherit-only entries do not themselves
grant mutation of the existing ancestor. Current-account, SYSTEM, Administrators
and the specific Windows TrustedInstaller identity are trusted OS authorities;
other service identities are not inferred trustworthy. Unknown effective ACE
forms fail closed. Deny entries are never used to excuse an unsafe allow entry.

Linux checks current-account/root ownership and group/world write mode bits.
Trusted-owner sticky directories are accepted because their removal rules protect
the child owner; all existing child owners are also checked. POSIX access ACL
write grants are bounded by the group-class mask represented in those mode bits.
Symlink ancestors, non-directory components and invalid paths reject admission.

Darwin additionally has a bounded native extended-ACL inspector using libc and
current/root user UUIDs. It distinguishes absent ACLs through a successful
`lstatx_np` and explicit security-property query. Untrusted mutation grants,
including inherit-only grants that could affect newly created children, reject
admission. Builds without the native ACL inspector fail closed. This Darwin
implementation has **not** been compiled with a macOS SDK or executed on a Mac.

## Actual checks

Windows x64, Go 1.26.6:

```text
go test ./internal/execution ./cmd/offgrid-computer -run 'TestPrivateAncestry|TestBrowserStorage' -count=1 -v
```

**PASS:** execution **0.604 seconds**, native computer **1.952 seconds**.
Log: `build/browser-storage-ancestry-windows-final-20261122.log`. Actual DACL
fixtures cover read-only access, delete-self, delete-child, WRITE_DAC, WRITE_OWNER,
generic write/all, inherit-only entries and conservative deny/allow handling.
The native regression checks that rejection leaves the parent's actual security
descriptor unchanged. Windows symlink cases explicitly skip because the test
process lacks symlink privilege.

WSL Ubuntu, Go 1.26.6, CGO enabled, `GOPROXY=off`, `GOMAXPROCS=4`:

```text
go test -p 1 ./internal/execution -run TestPrivateAncestry -count=1 -v
```

**PASS, 0.016 seconds**, including actual symlink and mode/sticky fixtures.
The foreign-owner fixture explicitly skips because this test process is not root.
Log: `build/private-ancestry-linux-final-20261122.log`.

Darwin arm64, `CGO_ENABLED=0`, `go test -c ./internal/execution`: **compile PASS**,
recorded in `build/private-ancestry-darwin-cross-20261122.log`. This only checks
the deliberate unavailable-native-inspector fallback, not the cgo implementation.

Earlier development failures remain recorded: a temporary missing-helper build
while registry integration was concurrent; an unavailable Windows constant name;
and a test comparing a literal SDDL spelling instead of canonical descriptors.
The final test compares the actual descriptor before and after rejection.

Positive native fixtures now choose a canonical, checked temporary parent or a
new isolated random directory below a checked home directory. Only newly created
fixture permissions change. Cleanup checks the owned parent and filename prefix.
Successful worker subprocesses receive `testhost.Account(t)` isolated account
configuration. No installed-user ACLs or live registry data are changed.

## Remaining evidence and limitations

- These are focused working-tree checks; they are not results from the earlier
  4209-file frozen snapshot or a replacement for the next full frozen suite.
- Native macOS ACL compilation, execution and filesystem qualification remain
  required. Run the same focused command with CGO enabled on a Mac; it includes
  real disposable `chmod +a` ACL fixtures.
- Linux foreign-owner/sticky behavior still needs the isolated root-capable
  fixture. Tests do not impersonate another Windows account or establish a
  cross-account browser race; the actual DACL admission defect is reproduced.
- The existing Darwin private-leaf helpers inspect POSIX modes, not read grants
  in extended ACLs. This ancestry check addresses mutation, not that separate
  private-data confidentiality gap. Full Darwin protected storage remains open.
- Privileged OS administrators and same-account unrestricted processes remain
  outside this permission boundary. Cross-client host admission is separate
  registry work. No transfer, recovery or platform milestone is checked complete.

## Primary sources

Microsoft's [file security and access rights](https://learn.microsoft.com/en-us/windows/win32/fileio/file-security-and-access-rights)
distinguishes effective and inherit-only ACEs and documents directory deletion
rights. Its [access-check semantics](https://learn.microsoft.com/en-us/windows/win32/secauthz/how-dacls-control-access-to-an-object)
explain why this conservative inspection does not infer safety from a deny entry.
[Windows Resource Protection](https://learn.microsoft.com/en-us/windows/win32/wfp/about-windows-file-protection)
describes TrustedInstaller's privileged ownership role.

The Linux [rename interface](https://man7.org/linux/man-pages/man2/rename.2.html)
documents parent permissions and sticky-directory restrictions. Apple's
[ACL declarations](https://github.com/apple-oss-distributions/Libc/blob/main/include/sys/acl.h),
[native ACL entry implementation](https://github.com/apple-oss-distributions/Libc/blob/main/posix1e/acl_entry.c),
[file-security property implementation](https://github.com/apple-oss-distributions/Libc/blob/main/gen/filesec.c)
and [membership declarations](https://github.com/apple-oss-distributions/Libinfo/blob/main/membership.subproj/membership.h)
inform the Darwin inspector. Sources consulted 2026-11-22; code inspection is not
native platform qualification.
