# Browser file transfer evidence — 2026-11-20

Status: **in progress** under [browser-file-transfers](../../../openspec/changes/browser-file-transfers/tasks.md).
All eight tasks remain open. This record does not qualify complete transfers,
attached browsers, installed platforms, models or human usability.

## Reproduced upload defect

The original companion checked a selected path's size/digest, then supplied that
same path to Chromium. The new real loopback HTTP/Chromium regression replaces
the original file immediately before `setInputFiles`. The independent recipient
received a different SHA-256 while the legacy result claimed verification.
`node --test --test-concurrency=1 test/browser-file-transfer.test.mjs` initially
reported **one passed, one failed, 1.289 seconds**. The first rejection case
confirmed that changes detected before preparation still cause zero assignment.
Log: `build/browser-file-transfer-baseline-20261120.log`.

The implementation now captures and hashes the exact bytes copied into an
exclusive private snapshot. It validates source regular-file identity, size,
mtime and digest, bounds reads, supports cancellation and keeps cleanup ownership
after a failure. A context-owned upload store shares the snapshot across its
tabs. Actual context closure precedes snapshot deletion: Chromium can read a
selected File lazily after the assignment call returns. Manual takeover clears
the grant but does not prematurely delete already selected file content.

The first integration passed the new defect test but failed an existing repeat
upload assertion: reusing captured bytes had omitted revalidation of the original
selection on a new action. That failure was **18 passed / one failed, 7.094
seconds**, retained in `build/browser-file-transfer-first-fix-20261120.log`.
Revalidation now uses bounded regular-file reads on every later action while
assignment continues to use the owned snapshot. The same combined files then
passed **19/19, 6.916 seconds**; log:
`build/browser-file-transfer-repeat-fixed-20261120.log`.

The installed Playwright 1.62.1 implementation rejects in-memory payloads at
50 MiB. OffGrid's 512 MiB contract was preserved. An actual **51 MiB** selected
file was read and posted by Chromium only after the tool returned and the original
source was replaced. The independent server streamed its size/digest and matched
the approved bytes. The three real transfer tests passed **3/3, 3.001 seconds**;
log: `build/browser-file-transfer-large-20261120.log`. This proves that exercised
size and lifecycle; it is not peak-resource or complete 512 MiB qualification.

## Implementation and remaining proof

Files: `apps/computer/browser-file-snapshot.mjs`, shared driver/workspace ownership,
runtime pack inventory, snapshot unit fixtures and real browser transfer fixtures.
The snapshot unit cases cover changed content, path/name rejection, pre-cancelled
and racing Stop, cleanup identity replacement and explicit retry. A sampled
independent review flagged Windows alternate streams/device names; explicit name
validation and targeted cases were added before any such path is opened.

Still required: special-file/substitution cases on supported platforms; protected
host storage boundaries against every application-owned writable path; durable
snapshot recovery; full desktop grant integration and renewal; all context/Stop cleanup
and full companion regressions after integration; peak/maximum-size/resource
qualification. Private random directories/read-only mode do not defend against a
process already controlling the user's full OS identity. The current result
format remains legacy; no stronger negotiated transfer feature is advertised.

Downloads still need in-flight budget enforcement, exclusive durable publication,
source/time/content evidence and explicit local inspection/export. Existing
post-completion size checks cannot establish bounded transfer. The new OpenSpec
scenarios retain these requirements; they are not satisfied by the upload fix.

All files, pages, bytes and HTTP effects were disposable fixtures. No personal
file/session, live installation, model weight, remote account or publication was
accessed. Overall M2 and M0–M8 remain incomplete.

The final snapshot-specific Windows run passed **7/7, 0.117 seconds**, including
explicit alternate-stream/device-name rejection before opening a proposed path.
Subsequent complete companion command
`node --test --test-concurrency=2 test/*.test.mjs` passed **169 tests with two
opt-in attachment containment probes skipped, 31.838 seconds**. Those two probes
were run separately and failed as documented in the attachment research; their
default skip is not a containment pass. Log:
`build/browser-file-transfer-companion-20261120.log`. This source checkpoint
includes upload ownership and actual transfer cases, all manual lifecycle cases,
pack integrity and the independent framing suite. It does not erase previous
default-concurrency/composed cleanup failures recorded under manual takeover.

## Actual selected-content verification

The isolated native File read-back is now integrated into `BrowserDriver` and the
runtime pack. It checks every selected byte against bounded snapshot chunks,
rehashes the complete snapshot against the grant, and retains the exact observed
input handle. Page-world getters, File/Blob methods and crypto cannot supply the
verification result. The receipt explicitly identifies `file_input_selection`;
legacy `uploaded` remains a compatibility field, not evidence of remote receipt.
No stronger negotiated transfer capability is advertised yet.

A driver-level regression installs a page change handler that substitutes wrong
bytes with the same name and size. Before integration the command falsely
succeeded: **one failed, 0.612 seconds**, log
`build/browser-file-readback-driver-baseline-20261120.log`. With the isolated check
it rejects after the one assignment; the exception remains an uncertain effect,
never a safe precondition for replay. The combined read-back, transfer, snapshot,
legacy browser and pack command passed **32/32, 9.490 seconds**, log
`build/browser-file-readback-integrated-20261120.log`. This includes the latest
cached-grant filename comparison. See [read-back evidence](browser-file-readback-2026-11-20.md)
for the independent engine tests and the preserved fixture correction.

## Download budget feasibility

Actual Chromium 151.0.7922.34 native progress callbacks are too coarse to establish
the hard staging budget. With a 256 KiB limit and a streamed 4 MiB response, the
first positive callback reported 1,835,008 received bytes; the fixture had sent
1,900,544 bytes by cancellation. This occurred both with declared length and with
chunked transfer/no length. Cancellation removed the native partial file, but
that is not a pre-write size bound. Logs:
`build/browser-download-progress-probe-20261120.log` and
`build/browser-download-progress-chunked-20261120.log`.

A subsequent prototype combines the existing request-scope routing with a separate
native response interceptor and denies the browser's automatic download sink.
It takes the original response body as a bounded CDP stream; it never issues a
second HTTP request. For a chunked 4 MiB fixture and 256 KiB budget, it read
262,145 bytes and wrote exactly 262,144, then closed/cancelled. The server saw one
request and sent 327,680 bytes, including transport buffering. A 128 KiB fixture
completed with the independently expected SHA-256. Both diagnostic runs exited
zero; no native download event or file was produced. Logs:
`build/browser-download-stream-probe-20261120.log` and
`build/browser-download-stream-small-probe-20261120.log`. This bounds owned staging,
not network buffers or all renderer resource use. The diagnostic is not yet a
production download path and does not cover blob/data URLs, ambiguous actions,
frames, durable artifact publication, cancellation faults or platform qualification.

The [CDP Fetch contract](https://raw.githubusercontent.com/ChromeDevTools/devtools-protocol/master/pdl/domains/Fetch.pdl)
requires cancellation or an explicit replacement response after taking a body
stream. [IO reads](https://raw.githubusercontent.com/ChromeDevTools/devtools-protocol/master/pdl/domains/IO.pdl)
accept a maximum chunk size. Source for the installed
[Chromium pipe stream](https://raw.githubusercontent.com/chromium/chromium/151.0.7922.34/content/browser/devtools/devtools_stream_pipe.cc)
shows reads consuming its bounded pipe on demand; this differs from accumulating
the complete body through `getResponseBody`. These primary-source details inform
the implementation choice and do not replace the executable lifecycle checks.

Further probes retained two important limitations. For both a generated `blob:`
download and a `data:` download, the response interceptor received no request;
the native download event still identified the initiating frame. With native
downloads denied, both ended cancelled at zero received bytes. Logs:
`build/browser-download-blob-probe-20261120.log` and
`build/browser-download-data-probe-20261120.log`. This demonstrates a safe denial
in those fixtures, not implemented local-URL download support.

A deliberately interrupted chunked response exposed a second defect in the
candidate stream path. The server sent 65,536 bytes of a planned 131,072-byte
response and destroyed the connection without completing chunked framing. The
stream nevertheless reported `eof:true`; no original `Network.loadingFailed`
event distinguished this from success before the replacement response. The
strict diagnostic failed (**exit 1, 0.782 seconds**):
`build/browser-download-stream-truncated-20261120.log`. The pinned
[interceptor source](https://raw.githubusercontent.com/chromium/chromium/151.0.7922.34/content/browser/devtools/devtools_url_loader_interceptor.cc)
retains original completion status while waiting for interception resolution.
Consequently, an IO end-of-stream plus local digest is insufficient proof of a
complete HTTP download. This candidate is not adopted as the production sink.

## Full 512 MiB acceptance — failing transport

The new opt-in `browser-file-max.test.mjs` generates the literal approved 512 MiB
contract using bounded chunks. After the driver verifies the complete native
selection, it replaces the original source and asks the page to send the retained
File to an independent streaming HTTP receiver. The first complete run failed:
**one failed, 10.585 seconds**. The fixture's response and submission promises
were then observed together to avoid obscuring an earlier failure during cleanup;
the strict rerun still failed (**10.097 seconds**).

A diagnostic run again failed (**10.267 seconds**). The selection step reported
the complete expected 512 MiB digest, with Node RSS 177,999,872 bytes at that
checkpoint. The independent receiver got **zero bytes**, and Chromium exited with
code **3758096392**. The initial free physical memory observation was about
5.1 GiB on this 16 GiB Windows host. These facts do not alone establish the cause.
Logs are `build/browser-file-max-20261120.log`,
`build/browser-file-max-observed-20261120.log` and
`build/browser-file-max-diagnostic-20261120.log`.

The failing acceptance case remains intact. Pinned request-interception source
and isolated with/without-interception controls are being investigated; no size
limit was lowered and no production origin policy was removed to obtain a pass.
Maximum-size end-to-end qualification therefore remains **failed/incomplete**,
despite the successful native selection step and earlier 51 MiB journey.

Runnable maximum-size procedure (requires 2 GiB free temporary disk):

```powershell
$env:OFFGRID_BROWSER_FILE_MAX_TEST='1'
node --test --test-concurrency=1 apps/computer/test/browser-file-max.test.mjs
Remove-Item Env:OFFGRID_BROWSER_FILE_MAX_TEST
```

The default companion suite explicitly skips this disk-intensive case; a default
skip must never be counted as maximum-size acceptance. The fixture uses synthetic
bytes, disposable Chromium and loopback HTTP, not model weights or user files.
