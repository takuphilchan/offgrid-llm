# Rate-limiter lifecycle correction — 2026-11-20

Investigation of the failed full server race runs found cleanup goroutines retained
after fixture shutdown. `RateLimiter` had no stop path. Eviction also read a
bucket's refill timestamp without the lock used by refill, and used a stale count
when enforcing the bucket bound.

The limiter now joins its cleanup worker on idempotent Close. Application resource
ownership and fixture cleanup call it, including failed application construction.
Handlers already draining retain rate-limit enforcement. Eviction reads timestamps
under the bucket lock and checks the current map size under the map lock.

- Linux WSL `CGO_ENABLED=1 go test -race ./internal/server -run '^TestRateLimiter'
  -count=1` passed, 2.990 s. Regressions cover concurrent idempotent close,
  continued enforcement while draining and concurrent eviction/refill at capacity.
- The first attempt did not run tests: fetching x/crypto v0.55.0 failed with a
  TLS handshake timeout. The verified Windows module download cache populated
  WSL's module cache; the successful test used `GOPROXY=off`.
- This finding does not explain away the aggregate timeouts. Both failed full
  runs remain in [workspace evidence](browser-workspaces-2026-11-20.md), and the
  later full race recheck passed on a verified Linux-native source snapshot in
  534.658 s. Filesystem environment and intervening changes differ, so this fix
  alone is not credited with eliminating the aggregate timeout.

No deadline was increased, test assertion removed or installed service changed.

## Isolated commit verification — 2026-11-21

Local commit `93c1c322` on `main` was created at 23:14 Asia/Shanghai, before the
requested 23:30 deadline. It contains the limiter correction, application cleanup
registration, transport-fixture cleanup and tests, including a deterministic
oldest-empty-key eviction regression. Unrelated program changes were excluded.

The exact staged source tree `09668dd4ddef96a2d05c17306c505d534a13a21f` was
archived and tested independently of the dirty working tree. Windows focused
server tests passed in 0.782 s and the complete application package passed in
4.854 s. Linux race equivalents passed in 4.796 s and 5.157 s respectively:

```sh
go test -race ./internal/server -run '^(TestRateLimiter|TestClientIP)' -count=1 -timeout=3m
go test -race ./internal/app -count=1 -timeout=5m
```

The Windows commands omit `-race`. Linux output is retained in
`build/rate-limiter-commit-final-linux-20261121.log`. An earlier runner passed
the focused server check but rejected the application command because of a CRLF
argument; the corrected runner executed both commands successfully. No hooks
were bypassed and no push was performed. This focused commit is not a full
server-suite or whole-program qualification.
