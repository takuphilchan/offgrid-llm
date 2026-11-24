# Disposable attachment network diagnostic

This is a test-only extension, not an OffGrid product extension. It has no native
host registration, service credentials or attachment authority. The test loads it
only into a newly created, headless Playwright Chromium profile and invokes its
fixed probe helpers from the test runner. Its fixture-origin host permission is
not evidence of the proposed production selection or permission contract.

From the repository root in PowerShell:

```powershell
$env:OFFGRID_ATTACHMENT_NETWORK_PROBE = '1'
try {
  node --test --test-concurrency=1 apps/computer/test/attachment-network-feasibility.test.mjs
  $probeExit = $LASTEXITCODE
} finally {
  Remove-Item Env:OFFGRID_ATTACHMENT_NETWORK_PROBE
}
exit $probeExit
```

The installed full Playwright Chromium is required; this command does not install
a browser or dependency. Without the environment flag the two tests are skipped.
The required zero-effect assertions are intentionally strict: a counterexample
fails the command and leaves the proposed network boundary unqualified.

Only loopback fixture servers receive traffic. The isolated browser maps
`*.offgrid-fixture.example` to `127.0.0.1` with an owned launch argument and ignores
the existing self-signed fixture certificate. This models a hostname resolving
to a private destination; it is not a real DNS-rebinding or public-to-private
browser-permission qualification. No live proxy, DNS settings, browser profile,
extension installation or native-host registration is changed.

The selected page opens a WebSocket before DNR session rules and debugger Fetch
interception are enabled. Controls confirm actual Fetch interception, denied new
foreign traffic and an unaffected sentinel tab. Button input then tests an effect
over that established socket and a POST to an allowed hostname resolving locally.
Independent server counters, rather than page claims, decide containment. Raw
credentials and personal content are absent from this fixture.
