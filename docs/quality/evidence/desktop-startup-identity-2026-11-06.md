# Desktop startup identity and presentation

The owner reported `Desktop files need repair` after installing the November 6
speech/composer preview. The normal installed service reported version `43.7.9`
instead of OffGrid `0.4.26`. Its UI fingerprint matched the desktop. The service
build metadata contained a build revision but no `main.Version` linker value.
Windows resolved the development fallback `VERSION` against Electron's lowercase
`version` file in the installation directory. Earlier isolated tests launched
from the repository and therefore concealed this defect.

The replacement service embeds `main.Version=0.4.26`. Executing it from the same
installed directory reports OffGrid `0.4.26`, while the adjacent Electron file
still contains `43.7.9`. No compatibility check or user data was changed to make
the mismatched build appear compatible.

Packaging now inspects Go build metadata for every bundled service architecture
and rejects absent, duplicate or mismatched embedded versions. This works without
executing cross-compiled services. The packaged startup test launches from the
application directory and verifies the owned service version explicitly.

Startup presentation uses one brand and one status, without repeated headings,
marketing copy, a large bordered card or footer. Busy states show a small spinner;
recovery controls and collapsed technical details appear only when needed. The
window title remains simply `OffGrid` in every state and locale. Existing local
recovery, external-workspace ownership and compatibility checks are unchanged.

The owner's final review further reduced recovery to one primary action. Bundle
mismatches offer the repair guide instead of an ineffective connection retry;
connection failures retain Retry. Web access, separate recovery and additional
help are inside the closed technical disclosure, not competing primary buttons.

## Verification

- Desktop Node suite: 134 passed, one platform-dependent skip.
- New packaging tests reject unversioned, incorrectly versioned and duplicate
  version flags; accept matching embedded versions with or without a tag prefix.
- Browser rendering inspected for dark loading/recovery and narrow light Arabic;
  no horizontal overflow, one heading, technical details hidden while loading.
- Packaged startup suite passed on Windows from the application directory,
  including owned-service startup, mismatched bundle rejection, external-service
  recovery, timeout handling and shutdown. First window: 900 ms; recovery: 1085 ms
  in this isolated run. These are observations, not a general performance claim.
- Private logs: `build/startup-desktop-full-20261106.log`,
  `build/startup-packaged-repair-20261106.log`.

The final minimal-recovery candidate also passed the packaged startup suite:
first window 903 ms, recovery 1153 ms in the isolated run. Evidence is in
`build/startup-packaged-minimal-20261106.log`. This candidate includes both the
fixed title and the single primary recovery action. Other native platforms and
independent human usability checks remain unverified here.

## Installed replacement

The minimal startup build was installed successfully on November 6. All five
checked application payloads match the tested candidate. All 393 workspace data
files retained their hashes; model inventory and native inference executables
were unchanged. Rollback is retained at
`D:\OffGrid-install-backups\v0.4.26-before-startup-minimal-20261106-184028`.
The private receipt is
`build/desktop-startup-minimal-20261106/install-result.json`.
The normal user workspace was not launched as part of installation verification.
