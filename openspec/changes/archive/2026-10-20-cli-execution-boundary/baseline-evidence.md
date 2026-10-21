# Phase 5 CLI baseline (2026-10-20)

The ignored `build/dev/offgrid-phase5-baseline.exe` was built in this worktree
with `go build -trimpath -o build/dev/offgrid-phase5-baseline.exe ./cmd/offgrid`.
Its SHA-256 is
`4BD4F9B7B8CA666C21E6545B97F187B672A219F586D6E55A0DC738D6CA283AE0`.
It is a comparison fixture, not a release artifact. The current source includes
the completed Phase 4 decomposition, which remains uncommitted amid the broader
reorganization. The prior Phase 4 `baseline-evidence.md` and
`verification-evidence.md` are archived under
`openspec/changes/archive/2026-10-20-cli-architecture-decomposition/`.

The current command package has 91 production `os.Exit` calls and 53 production
`os.Args` references. `root.go` has 47 `case` labels, identical in content and
order to the pre-decomposition `main.go`. With no command, and with `serve` or
`server`, it loads `OFFGRID_CONFIG` through `config.LoadWithPriority`, sets
`OFFGRID_VERSION` if absent, then calls `server.NewWithConfig(cfg).Start()`.
This stateful path was inspected, not run against the installed service.

The existing `command_result_test.go` runs a subprocess against an isolated
`httptest` service with temporary data/model directories; its table covers
authenticated service errors, invalid usage and JSON results. The concurrent
worktree already modified `computer_commands.go`,
`model_package_commands.go`, and `model_package_commands_test.go` before this
phase; those edits are not part of the source extraction. Other staged
reorganization and provider changes remain user work.

Safe current-output oracles from the Phase 4 verified build (PowerShell
`Out-String`, SHA-256 over UTF-8) are retained:

| Invocation | Exit | Characters | SHA-256 |
| --- | ---: | ---: | --- |
| `--no-color --help` | 0 | 2355 | `5E75624B1C34E1766BC865B8B3CAC007066D8034E34C81CE89DF4AEECD0044AE` |
| `version` | 0 | 113 | `BC4065A9B1DF08C113F28F7D614FDAB73A1745DBB4BB2151CEA69C4ECB2AFC26` |
| `--json model preview --category nonsense` | 2 | 116 | `B76ED794A0C1A7CB63909DE2FB3C3251C65D3E97EF1A0805B1529C7D10E715EC` |
| `chat` | 0 | 421 | `7D38CE1DA5364C52DFFD0BD80B96EABD116D871EA47BDEE363C7E1C5CB0B1D8D` |
| `--no-color run --help` | 0 | 774 | `7A8B1B69CD1986BD1D2F794B29DD295B0908E34730E3E3E4D221683679A51B6A` |
| `bench --help` | 1 | 438 | `A06221BAEA3475FF3F4EEA702805FDD50C194B5054F3C1951C183398AF384285` |
| `rm --help` | 1 | 345 | `853BEA96EF8324ED659A9EA0F36F52DA6E8A5C26845479AF0856B0A2494FC66E` |
| `fav --help` | 0 | 323 | `194FD1A59AFF367F0CA8D0674D0DCA0938545CD17E5B472DD28B3BC4CEC7B4B3` |

These commands do not access the personal service or mutate model files.
Stateful alias routes and true platform-specific Ctrl+C delivery need isolated
fixtures before changing their execution paths.

`command_result_test.go` now has isolated subprocess fixtures for JSON flag
placement, `NO_COLOR`, safe aliases, invalid usage, and the existing
authenticated service errors. The focused Windows test run passed. It also
contains a Unix child-process interrupt fixture, but Go cannot deliver
`os.Interrupt` to a Windows child process, so that case is skipped locally;
Windows cancellation mapping is separately covered by `TestCommandErrorContract`.
The Unix signal path still needs a Linux/macOS CI run before platform claims.
