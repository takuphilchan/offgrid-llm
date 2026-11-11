# Desktop distribution verification

Installer checksums identify bytes. They do not establish publisher identity,
Windows reputation, Apple notarization, hardware compatibility or a first answer.
Keep those claims separate in release evidence.

## Local Windows preview

After installing the locked web, desktop and computer build dependencies and
preparing the pinned PDF/OCR assets, run from the repository root:

```powershell
node scripts/build/desktop-preview.mjs
```

This builds the current renderer and service, then the NSIS and portable packages.
It uses the next patch version with a `preview` timestamp and Git revision; it
does not edit source package versions, install the app, or publish anything.
Desktop, service, executor and terminal package checks use that same version.
The version remains visible through the desktop's About dialog and service
identity. The output directory contains `preview.json` (including whether the
workspace was dirty) and `artifact-trust.json`. A local preview is never labelled
release-qualified, even if a signing identity happens to be available.

An unsigned local build can be inspected without pretending it is signed:

```powershell
node scripts/release/verify-desktop-trust.mjs --file PATH_TO_SETUP.exe --platform windows --mode inspect --report trust.json
```

`inspect` records native signature results and SHA-256. Release mode is the
default and exits unsuccessfully unless all required trust checks pass. Do not
disable OS security settings to turn a failing report into a passing one.

## Public release credentials and gates

The release workflow verifies the **downloaded draft assets**, including reused
assets on reruns, before final publication. It checks the GitHub asset digest
and then runs native trust checks on both Windows installers or both macOS
architecture ZIPs. A reused asset is not trusted because it has a matching name.

- Windows: configure repository secrets `WINDOWS_CSC_LINK` and
  `WINDOWS_CSC_KEY_PASSWORD` for electron-builder, and repository variable
  `OFFGRID_WINDOWS_SIGNER_THUMBPRINT` with the intended signing certificate's
  40-character SHA-1 thumbprint. Both Authenticode status `Valid` and this exact
  publisher pin are required. Update the pin deliberately when renewing the
  certificate. A valid signature does not promise SmartScreen reputation.
- macOS: configure secrets `MACOS_CSC_LINK`, `MACOS_CSC_KEY_PASSWORD`, `APPLE_ID`
  and `APPLE_APP_SPECIFIC_PASSWORD`, plus variable `APPLE_TEAM_ID`. The extracted
  app must pass strict/deep code signature verification, match the intended Team
  ID, pass Gatekeeper assessment, and have a valid stapled notarization ticket.
  Ad-hoc signatures do not pass. Native macOS build and verification hosts are
  required; Windows fixture tests cannot qualify this path.

The workflow preserves the trust report as a CI artifact. Missing credentials,
wrong publisher, unsigned/modified artifacts, or failed notarization prevent
publication. Credential availability and successful native release runs remain
external qualification requirements; implementing this gate does not satisfy
them. Linux continues to use the release integrity manifest and its native
package qualification; this verifier does not claim Linux publisher signing.

Run verifier fixtures with:

```sh
node --test scripts/release/verify-desktop-trust.test.mjs scripts/build/desktop-preview.test.mjs
```

Also run the [project checks](../../openspec/README.md#terminal-checks), package
verification, isolated installer/first-response tests and an upgrade preservation
check. The [production-readiness contract](../quality/production-readiness.md)
remains authoritative for release qualification.

## Isolated native qualification

Use a separate app ID, **package name** (`extraMetadata.name`), executable name
and installation directory for disposable installer tests. Electron-builder's
updater cache follows the package name, so a different app ID alone does not
isolate its cached installer. Disable protocol registration, shortcuts and launch
after installation; retain `deleteAppDataOnUninstall: false`. Verify the intended
cache directory before running the installer, compare every installed payload
file, and check that the ordinary app's installer cache stays unchanged during
both install and uninstall.

The native reply harness copies a local model and verified runtime into a new
temporary profile, then checks explicit engine preparation, a saved reply,
onboarding completion and restart preservation:

```sh
node scripts/test/test-desktop-native-reply.mjs DESKTOP_EXE MODEL_GGUF VERIFIED_RUNTIME_DIRECTORY
```

It launches a test window and owned service and consumes model memory; keep other
installation tasks separate. The default is Windows CPU. Add `--gpu` only with a
verified CUDA runtime manifest whose platform is `windows-amd64-cuda` and enough
free GPU memory. The CPU manifest uses `windows-amd64-cpu`. Both modes check every
supplied binary/DLL digest and retain the original model bytes. GPU qualification
does not qualify CPU performance or imply a GPU runtime is automatically bundled.

The native process-guard fixture compiles disposable NSIS executables and checks
absent/running processes and inspection failures. It uses one harmless owned
process, writes only temporary marker files, and creates no application or
uninstall registration:

```sh
node scripts/test/test-installer-process-guard.mjs PATH_TO_MAKENSIS.exe PATH_TO_X86_UNICODE_PLUGINS
```

Use the NSIS compiler and plugin directory provisioned by electron-builder.
The fixture prints the path to its result report. This checks the guard itself;
also qualify the complete installer to prove refusal precedes payload, registry
and updater-cache changes.
