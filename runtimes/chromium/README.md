# Metadata-only Fetch patch preparation

**Uncompiled, unqualified, and not used by production.** This artifact continues
`browser-file-transfers` M2.07; it does not complete its 512 MiB upload requirement.
It changes no installed browser, runtime manifest, service capability or companion
negotiation. Stock attached Chrome/Edge remain outside this managed-build patch.

The pinned upstream source is Chromium `151.0.7922.34`, commit
`782af9cb30a53f54487e5d2e44738645a8ec457c`. [source.lock.json](source.lock.json)
records exact upstream URLs, byte counts, SHA-256 preimages, patch digest and all
postimages. Only nine small reference/source files were fetched to author it;
there is no full checkout, generated protocol, compiler result or browser binary.

## Behavior

[metadata-only-fetch.patch](metadata-only-fetch.patch) adds experimental
`Fetch.enable.includePostData`, defaulting to `true`. With `false`, newly created
interception jobs do not clone, drain, buffer or serialize the request body for
`Fetch.requestPaused`/`Fetch.authRequired`. They emit ordinary request metadata
and native `hasPostData`, omitting `postData` and `postDataEntries`.

The original `ResourceRequestBody` remains untouched. Continue without a body
override retains Chromium's native upload; explicit overrides and abort remain
supported. The setting is captured per job and retained through redirects/auth
retries, including when `Fetch.enable` is called again. Other DevTools clients
or Network-domain observers are not disabled or promised bounded memory by this
patch. Existing Network interception calls retain the default collection mode.

This removes an interception diagnostic copy, not browser origin policy. A future
qualified integration must keep the existing route owner, origin enforcement,
WebSocket denial and pinned CONNECT relay. It must never rewrite raw File to
multipart, lower 512 MiB, proxy an upload independently, or trust page bytes.

## Apply to a dedicated source checkout

The build-only helper downloads nothing. It rejects a different Git HEAD,
changed preimages, occupied new test paths and symlinked source ancestry. It does
not commit, build, install or update packs. Keep the checkout quiescent while
checking/applying. The source-only authoring mirror cannot pass the Git HEAD
requirement and must not be represented as a complete checkout.

```sh
node /path/to/offgrid/runtimes/chromium/apply-patch.mjs --check /path/to/chromium/src
node /path/to/offgrid/runtimes/chromium/apply-patch.mjs --apply /path/to/chromium/src
```

Application is explicit and one-time; repeated application rejects. On a
post-application verification error, inspect that dedicated checkout instead of
assuming it is pristine. The script does not automatically discard any work.

## Native build prerequisites and qualification procedure

A separately authorized, isolated full checkout at the exact commit, matching
DEPS, depot_tools, GN/Ninja, compiler/sysroot and platform dependencies are needed.
The [upstream Linux instructions](https://chromium.googlesource.com/chromium/src/+/main/docs/linux/build_instructions.md)
specify at least 100 GB free disk and 8 GB RAM, with more than 16 GB recommended;
reserve additional capacity for test outputs, symbols and upload fixtures. The
[Windows instructions](https://chromium.googlesource.com/chromium/src/+/main/docs/windows_build_instructions.md)
also require at least 100 GB on NTFS, the appropriate Visual Studio C++/MFC/ATL
toolchain and Windows SDK. Confirm toolchain versions against the pinned checkout
before provisioning; current-main instructions are not a version lock. These
downloads/installations/builds have not been performed or authorized by this
artifact. The Windows authoring shell has no callable Python; the receiver-only
fixture checks used already available WSL Python.

After source/dependencies/toolchains are available, a candidate Linux build is:

```sh
cd /path/to/chromium/src
gn gen out/OffGridMetadata --args='is_debug=false is_component_build=true symbol_level=1'
autoninja -C out/OffGridMetadata chrome blink_tests
python3 third_party/blink/tools/run_web_tests.py -t OffGridMetadata \
  http/tests/inspector-protocol/fetch/offgrid-metadata-only.js
python3 third_party/blink/tools/run_web_tests.py -t OffGridMetadata \
  http/tests/inspector-protocol/fetch
```

The build generates Fetch bindings from the changed PDL; generated files must
not be hand-edited. Upstream [web-test instructions](https://chromium.googlesource.com/chromium/src/+/main/docs/testing/web_tests.md)
require `blink_tests` and use `run_web_tests.bat` on Windows. Preserve unexpected
results; the authored expected output has not been confirmed by a native run.

The new native test uses a bounded HTTP receiver to check exact bytes/digests
for raw byte/Blob/File bodies, omitted/explicit legacy collection, empty request
metadata, explicit body override, abort, page cancellation, 307 mode retention
and response-stage continuation. The receiver limit is 8 MiB for this small
regression fixture, not the product upload limit. Abort assertions check fetch
settlement; they do not independently prove zero remote effects or resource
cleanup. Existing native fetch/auth/worker/frame tests and independent receiver
effect/cleanup checks remain required. There is no native allocator-spy test yet.

Next, build a separately identified managed runtime compatible with the pinned
Playwright 1.62.1 recipe/patches, and verify those assumptions rather than merely
renaming a stock binary. Production `apps/computer/pack.cjs` currently hashes the
complete browser payload and pins Playwright; it has no metadata-only capability.
An ignored unknown CDP parameter is not capability proof. Any future handshake
must bind a distinct proven capability to the verified binary/pack identity and
apply the setting through the same interception owner on every covered target.

Only after that separately reviewed integration should the unchanged
`apps/computer/test/browser-file-transport-probe.test.mjs` and
`apps/computer/test/browser-file-max.test.mjs` be run against the exact candidate.
Their current launch paths still use the installed unpatched runtime. Require
all 536,870,912 bytes and the independent digest, original-source replacement,
raw and multipart controls, production HTTPS relay, deny-before-effect cases,
Stop and confirmed teardown. Record browser/network-process memory separately
from Node over increasing sizes and slow receivers. Check other active DevTools
body collectors before claiming bounded process memory. Stock attached-browser,
platform, real-site and installed qualification remain separate gates.

See [actual preparation evidence](../../docs/quality/evidence/chromium-metadata-patch-2026-11-22.md).
