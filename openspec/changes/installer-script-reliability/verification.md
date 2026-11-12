# Verification: installer-script-reliability

Verified on 2026-11-11 using Windows PowerShell 5.1, Node, and Git Bash.

| Dimension | Result |
| --- | --- |
| Completeness | Installer code, guidance, and CI regression wiring delivered |
| Correctness | All three added requirements have implementation and fixture coverage; native installations not verified |
| Coherence | Standalone scripts, existing asset names, same-release checksums, scoped Windows targets, and Unix refusal follow the design |

Checks run successfully:

- `node --test scripts/maintenance/desktop-installers.test.mjs`: 5 tests. Covers exact valid/missing/duplicate/malformed/mismatched manifest entries, binary-marker checksums, complete nested payload copies, missing runtime rejection, user/machine target selection, running/absent/error process checks, real script entry-point checksum failures and simulated setup exit failure. All downloads and process launches in entry-point tests are mocked.
- `node --test scripts/maintenance/check-docs.test.mjs`: 7 tests.
- `node scripts/maintenance/check-docs.mjs`: 345 maintained Markdown files, zero problems (before this evidence file; OpenSpec artifacts are not user guides).
- `bash scripts/maintenance/test-installer-entrypoints.sh`: passed existing local/streamed, release selection, checksum and service fixtures.
- `bash scripts/maintenance/test-executor-install.sh`: passed existing executor installation fixtures.
- `bash -n installers/desktop.sh`: passed; Windows AST parsing also passes in the fixture suite.
- `openspec validate installer-script-reliability --strict --no-interactive`: passed.
- Scoped `git diff --check`: passed.

The new fixture suite is wired into Linux installer checks and Windows native desktop CI. CI itself has not been run remotely. Existing unrelated edits in ci.yml and application sources were preserved.

No critical issues found in the checks that ran. Native Windows elevated/unelevated installation, registry PATH effects, actual downloads/signatures, Linux/macOS installation, model inference, and full Go/web/desktop suites were not run. Fixture selection verifies PATH scope but does not modify the user's PATH. Cross-platform native qualification and publisher trust remain release gates, not claims of this patch. No installed application or user workspace was modified, and nothing was committed or published.

## Follow-up checkpoint: verified installers and archive discovery

The subsequent distribution-first-run-readiness work extends this checkpoint with
root/archive helper verification and process refusal, exact ARM archive selection,
unsupported CPU/desktop refusal, and executable-relative CLI runtime discovery.
The unverified optional Piper download was removed; speech/UI copy failures now
propagate. These changes do not complete the separate onboarding/signing/install
work in that active change.

Checks on an isolated copy of the exact staged snapshot on 2026-11-11:

- All nine `desktop-installers.test.mjs` tests passed, including ARM/x64 selection,
  Windows AVX2 refusal, checksum failures, generated helper payload preservation,
  and speech copy failure without any fallback download.
- Root installer and executor-install Bash fixtures passed.
- Full `go test ./internal/audio ./internal/inference ./internal/server ./internal/execution`
  passed. This snapshot excludes unfinished runtime-preparation/API/UI changes.
- Documentation tests passed; the staged-tree link check inspected 342 maintained
  documents with zero problems. The source-size ratchet and staged whitespace
  check passed. The self-contained root installer has a reviewed 676-line baseline.
- Native Windows architecture/AVX2 read-only preflight passed on this host.
- Release branding/asset naming and immutable release-context fixtures passed.
- Release finalization fixtures passed using Git Bash and a verified jq 1.8.2
  test dependency with binary/LF output. The first attempt selected the Windows
  WSL alias; it was stopped, and native jq CRLF output was normalized in the test
  environment to match the Linux release runner. No release action was performed.

Snapshot extraction exposed CRLF conversion of systemd units; `.service` files
now explicitly retain LF and the exact-byte fixture passed. These are automated
fixtures and discovery checks, not fresh installation or inference qualification.
Full project CI, signing, and production installation remain outside this checkpoint.
