# Surviving supervisor recovery — 2026-11-23

Status: verified local Windows/Linux recovery increment. Complete owner loss,
native macOS and unrestricted process containment remain open.

## Implemented boundary

An updated local execution worker hands its original supervisor a bounded private
startup envelope. It binds recovery authority to the existing host-protection
reservation, approved grant digest, exact workspace/journal locations and native
journal file identity. The worker waits for a private acknowledgement before
dispatch. Missing legacy handoff retains legacy behavior; malformed present
handoffs fail. The capability is excluded from public worker identity, inspection
and launched-child environments.

The original `StdioEndpoint` can publish reservation release only after its
retained native owner confirms descendant cleanup. `RetryProtectionSettlement`
retries this publication; it does not accept caller-supplied cleanup proof,
reconstruct a supervisor, reopen/create a journal or replay an operation. Missing
or replaced journals retain the fence. Forced worker failure remains an error
even after successful cleanup. The operation's uncertain receipt remains
uncertain across an explicit matching restart.

Implementation: `internal/execution/host_protection_stdio.go`,
`host_protection_startup.go`, platform journal-identity helpers and the existing
worker/stdio lifetime owners. This continues the existing execution architecture.

## Actual evidence

Frozen 4,320-file source archive:
`build/continuous-voice-checkpoint-20261123.tar`, SHA-256
`92bb7cb8cb3f0d2ee7aaf58243333a1dc26ce82c192e8bccad46586b93858317`.
Every extracted source file was checked against its manifest before execution.

Full Linux `go test -race -p 1 -json -count=1` passed:

| Package | Seconds |
|---|---:|
| `internal/storage` | 6.476 |
| `internal/execution` | 123.571 |
| `cmd/offgrid-computer` | 2.123 |
| `cmd/offgrid-executor` | 7.757 |
| `internal/computer` | 3.925 |
| `internal/server` | 320.269 |

Log: `build/survivor-linux-race-20261123.jsonl`; zero failed test cases,
41 platform/opt-in skips. The real forced-worker/descendant cleanup case passed
in 4.66 seconds. It checks cessation of descendant writes, preservation of a
foreign reservation, release of the owned reservation, retained uncertainty and
no repeated effect after matching restart. All processes/state are disposable.

The same six full frozen Windows packages passed in 8.275, 55.506, 8.123,
7.173, 2.072 and 210.216 seconds respectively. Log:
`build/survivor-windows-20261123.jsonl`; zero failed test cases, 73
platform/opt-in skips. Its real forced-worker/descendant cleanup case passed
in 4.03 seconds. This Windows run does not use the race detector; Linux does.
Earlier focused Windows native execution also passed in 4.249 seconds
(`build/host-protection-survivor-native-windows-20261122.log`).

Handoff checks additionally cover rejected/malformed startup, foreign authority,
release without native proof, publication retry, replaced/missing journals and
private-field exposure. Actual native cleanup tests supplement these fixtures.

## Remaining limits

No settlement is established when all original supervisors are lost or before a
complete handoff. There is no cleanup inference from a missing PID, stale lease
or caller boolean. This increment does not qualify isolated-backend handoff,
macOS native ownership, real daemon containment, power loss, installed-user
recovery, browser-profile retirement or unrestricted commands/MCP/plugins.
Independent final-source review remains outstanding; passing tests do not close
the full roadmap requirement.
