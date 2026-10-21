# CLI pre-extraction baseline (2026-10-19)

The authoritative worktree is `D:/offgrid-llm`, HEAD `68d36b22544f3ce5843cad53fb7f7bff6ea15eb1`. `cmd/offgrid/main.go` was clean at capture time, 7,564 physical lines, SHA-256 `F4443231CCD9A5168949D84F22F9B64F683AB95B1C9631F01B115D4AA0E0629F`. Other CLI files already modified by concurrent work were `computer_commands.go`, `model_package_commands.go`, and `model_package_commands_test.go`; this change must preserve them. `go test ./cmd/offgrid` passed before extraction.

The declaration inventory found 111 top-level `func`, `type`, `var`, or `const` starts in `main.go` (from `Version` at line 41 through `isSystemdService` at line 7557). They cover versioning, two shared HTTP clients, presentation/progress types, startup helpers, root dispatch, model operations, run/chat, diagnostics, administration, audio, legacy agent/integration, and logs. The complete one-to-one destination map is recorded below before code movement.

`go build -trimpath -o build/dev/offgrid-cli-baseline.exe ./cmd/offgrid` passed from this source tree. The ignored baseline binary SHA-256 is `75CC4951D6C3E6A2FDA0591F3B21A91845F3608C62CAA8416B5503EA5159A325`. The following captures used that binary and PowerShell `Out-String`, then SHA-256 of its UTF-8 bytes. They are comparison fixtures for this worktree, not release binaries.

| Safe invocation | Exit | Characters | Captured output SHA-256 | Observation |
| --- | ---: | ---: | --- | --- |
| `--no-color --help` | 0 | 2355 | `5E75624B1C34E1766BC865B8B3CAC007066D8034E34C81CE89DF4AEECD0044AE` | Full root help; no service request. |
| `version` | 0 | 113 | `BC4065A9B1DF08C113F28F7D614FDAB73A1745DBB4BB2151CEA69C4ECB2AFC26` | `0.4.18`, `windows/amd64`, CPU only. |
| `--json model preview --category nonsense` | 2 | 116 | `B76ED794A0C1A7CB63909DE2FB3C3251C65D3E97EF1A0805B1529C7D10E715EC` | `invalid_usage` for unsupported category, `retryable:false`. |
| `chat` | 0 | 421 | `7D38CE1DA5364C52DFFD0BD80B96EABD116D871EA47BDEE363C7E1C5CB0B1D8D` | Existing informational placeholder, not a live chat. |
| `--no-color run --help` | 0 | 774 | `7A8B1B69CD1986BD1D2F794B29DD295B0908E34730E3E3E4D221683679A51B6A` | Usage exits before model lookup or download. |

The root switch in `main.go` routes aliases `rm`, `bench`, `fav`, `rag`, `server`, and `diagnose` to their existing handlers; existing process-level `os.Exit` behavior remains part of the baseline. The no-command path loads configuration and starts the HTTP server. These potentially stateful routes were inspected rather than executed against the user's service. `cmd/offgrid/command_result_test.go` exercises CLI subprocess exit and JSON behavior with an isolated `httptest` service and temporary data/model directories. No installed-user data, service, or desktop app was modified for this capture.

## One-to-one declaration destination map

Original physical-line intervals below are inclusive and cover all 111 top-level declaration starts. Each interval ends immediately before the next interval begins; attached comments move with their declaration. Existing sibling command files remain in place. The `httpClient`/`httpClientLong` group at lines 68–97 stays together in `root.go` to avoid initialization drift.

| Original `main.go` lines | Responsibility | Destination |
| --- | --- | --- |
| 41–43 | Version link | `main.go` |
| 44–67 | Version resolution | `version.go` |
| 68–97 | Shared HTTP client globals | `root.go` |
| 98–132 | Basic terminal messages | `terminal_output.go` |
| 133–182 | Text progress bar | `progress.go` |
| 183–316 | Terminal errors, options, boxes, sections | `terminal_output.go` |
| 317–567 | Service/llama start and readiness helpers | `server_command.go` |
| 568–709 | Spinner and structured progress bar | `progress.go` |
| 710–769 | Terminal section/card helpers | `terminal_output.go` |
| 770–984 | Llama reload and readiness helpers | `server_command.go` |
| 985–1200 | Root command dispatch and default server branch | `root.go`, with process entry in `main.go` and server branch in `server_command.go` |
| 1201–1775 | Download/import/remove/export helpers | `model_transfer.go` |
| 1776–1800 | Existing informational `chat` placeholder | `run_chat.go` |
| 1801–1900 | Doctor diagnostics | `diagnostics.go` |
| 1901–2096 | Initialization command | `init_command.go` |
| 2097–2202 | Auto-select diagnostics | `diagnostics.go` |
| 2203–2419 | Benchmark | `benchmark.go` |
| 2420–2546 | Test/diagnostic command | `diagnostics.go` |
| 2547–2796 | List, quantization and quantize | `model_local.go` |
| 2797–2906 | Root help | `terminal_output.go` |
| 2907–2946 | Version command | `version.go` |
| 2947–3180 | Peers and info diagnostics | `diagnostics.go` |
| 3181–3306 | Configuration command | `config_command.go` |
| 3307–3465 | Search command and service-backed search | existing `model_search.go` |
| 3466–3572 | Hugging Face, projector and quant-name helpers | `model_remote.go` |
| 3573–4438 | Run/chat and request/response types | `run_chat.go` |
| 4439–4448 | Numeric terminal formatting | `terminal_output.go` |
| 4449–4497 | Model RAM estimation | `model_local.go` |
| 4498–4576 | Disk/size/wrap/duration formatting | `terminal_output.go` |
| 4577–4595 | Server health probe | `server_command.go` |
| 4596–4909 | Alias, favorite and template commands | `model_preferences.go` |
| 4910–4992 | Batch command | `batch_command.go` |
| 4993–5021 | Relative-time formatting | `terminal_output.go` |
| 5022–5175 | Completion command | `model_remote.go` |
| 5176–5256 | Model verification | `model_local.go` |
| 5257–5285 | Shell completion command | `terminal_output.go` |
| 5286–5326 | Legacy session export formatting | `session_legacy.go` |
| 5327–5615 | Benchmark comparison | `benchmark.go` |
| 5616–5882 | Audit command | `audit_command.go` |
| 5883–6105 | Users command | `users_command.go` |
| 6106–6159 | Metrics command | `metrics_command.go` |
| 6160–6289 | LoRA command | `model_local.go` |
| 6290–6646 | Legacy agent command | `agent_legacy.go` |
| 6647–6929 | Legacy MCP command | `agent_mcp_legacy.go` |
| 6930–7449 | Audio and multipart helpers | `audio_command.go` |
| 7450–7564 | Logs and systemd detection | `logs_command.go` |

The root dispatch interval contains no other top-level declaration. Its branch statements will be moved intact rather than dispatched twice. The default server code remains an explicit, no-argument branch reached once. The map intentionally does not copy functions already located in separate sibling files.

The map was mechanically checked against `rg -n '^func |^type |^var |^const ' cmd/offgrid/main.go`: 111 declaration starts, 46 contiguous intervals from line 41 through 7564, zero missing or multiply assigned declarations, and zero interval gaps. The package clause and imports are outside the declaration intervals and will be regenerated per file. Splitting init, model preferences, and legacy MCP responsibilities keeps all target groups near or below 600 lines except the indivisible `handleRun` function (about 806 lines), documented in `design.md`.
