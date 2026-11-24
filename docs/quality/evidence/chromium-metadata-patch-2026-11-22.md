# Chromium metadata-only Fetch patch preparation — 2026-11-22

Scope: existing `browser-file-transfers`, M2.07. **Native implementation artifact
only: uncompiled and unqualified.** No production browser code, pack negotiation,
capability advertisement, installed state or 512 MiB acceptance assertion changed.
The [original failure and controls](browser-upload-transport-2026-11-21.md) remain
authoritative. No browser process or large-file probe ran during this increment.

## Exact source and patch

Chromium tag `151.0.7922.34` resolved through the primary GitHub mirror tag API to
commit `782af9cb30a53f54487e5d2e44738645a8ec457c`. Nine exact files were fetched
from that immutable revision into ignored
`build/chromium-metadata-source-20261122`; no Chromium checkout, dependencies,
browser binary or model was downloaded. The maintained
[source lock](../../../runtimes/chromium/source.lock.json) contains every source
URL, length and SHA-256, plus all patch preimage/postimage identities.

The [patch](../../../runtimes/chromium/metadata-only-fetch.patch) changes five
native/protocol files and adds three native protocol-test/receiver files. Its
SHA-256 is `728507d858342bcf8997394927b940d0c3db674ec92aaf7d75b90264f8907e88`.
`includePostData` defaults true. The opt-in false setting is captured by each
interception job; it skips body collection without changing the original upload
or its existing continue/abort paths. It preserves body-presence metadata and
does not suppress other DevTools observers. This is not an origin-policy bypass.

## Executed checks

- `node --test scripts/test/chromium-patch.test.mjs`: **5/5 PASS**, no skips,
  74.0122 ms. Exact preimages/absent new paths, changed user work, occupied tests,
  invalid/nonregular paths, actual directory junction denial and explicit CLI
  operation admission. Log: `build/chromium-metadata-admission-20261122.log`.
- Exact upstream sparse-source preimages, `git apply --check --whitespace=error`,
  actual sparse application, all eight postimage SHA-256 values, and reverse
  application restoring all five original files/removing the three new tests:
  **PASS**. This verifies patch mechanics only; the sparse mirror has no matching
  Git HEAD/build tree and cannot pass the production build helper's HEAD check.
  Log: `build/chromium-metadata-patch-roundtrip-20261122.log`.
- New native protocol test and application helper JavaScript syntax: **PASS**.
  Native protocol test execution: **UNRUN**.
- Receiver fixture through available WSL Python: exact 128 KiB digest, empty
  input, 307 response and bounded oversize rejection: **4/4 PASS**. It ran as an
  isolated CGI subprocess with supplied bytes, not as a browser/HTTP journey.
  Log: `build/chromium-metadata-cgi-20261122.log`.

An initial patch check rejected an extra blank line at the expected-output EOF;
that artifact whitespace was corrected before the recorded round trip. An earlier
check used an incorrect relative patch path and did not inspect or modify source.
Neither is presented as a native regression test or browser result.

## Required next evidence

Independent read-only review found no concrete blocker in the native/protocol
control flow or patch admission helper. It checked the default compatibility,
per-job policy snapshot, collector bypass and pinned native serializer behavior.
This review ran no native tests or build and does not establish any of the
qualification results listed below.

The [runtime procedure](../../../runtimes/chromium/README.md) lists dedicated
checkout/toolchain/storage prerequisites, native build/test commands, runtime
identity/Playwright compatibility work and the subsequent unchanged 512 MiB
probes. No full source, dependencies, toolchain provisioning, native build,
generated-protocol compilation, native test execution, memory bound, zero-effect
abort proof, teardown qualification or managed-runtime integration is established.
The authored expected output still requires confirmation; it must not be updated
to hide semantic failures. This patch does not fix or qualify stock attached
Chrome/Edge. All these requirements remain incomplete.

## Local build capacity observed

Read-only inventory on 2026-11-22 found approximately 25 GiB free on Windows
`D:` and 31 GiB on `C:`. Physical memory is 16,866,033,664 bytes with 16 logical
processors; WSL currently reports 9.7 GiB memory and 4 GiB swap. Its filesystem
reports 563 GiB logical free space, which does not establish that its expanding
virtual disk has equivalent free physical backing. Neither observed Windows
volume meets the upstream 100 GB free-storage prerequisite. Available WSL tools
include Git, Python and Ninja; `clang` and `gn` were not on PATH in this check.

A separate disk or build machine with sufficient actual capacity is needed
before provisioning the full isolated checkout and toolchain. The user was
asked for that resource; no existing files were deleted and no full checkout or
toolchain install was started. Voice and local recovery work continue independently.

Follow-up, 2026-11-23: the user authorized cleanup. The
[generated-build cleanup](build-cleanup-2026-11-23.md) reclaimed approximately
71.09 GiB and left 96.02 GiB free on the physical D: volume backing WSL.
An isolated shallow fetch of the exact Chromium commit was attempted at
`/home/phil/.cache/offgrid-chromium-build-20261123/src`. Its owned process group
is stopped if physical free space falls below 20 GiB or a fetch step exceeds
one hour. Source/dependency acquisition and compilation are not yet successful
qualification; no installed browser or production negotiation changed.

The fetch failed with exit 128 after a 300,046 ms connection timeout to
chromium.googlesource.com (`build/chromium-source-fetch-20261123.log`). Only a
116 KiB Git directory exists; no source checkout or dependencies were obtained.
Bounded follow-up probes also failed: Windows HTTPS to the Google source host
timed out after 15 seconds with zero bytes, and WSL HTTPS to GitHub hit its
10-second connection timeout. No global network settings were changed. Storage
cleanup resolved the initial capacity prerequisite; network acquisition and all
native build/512 MiB qualification remain incomplete.
