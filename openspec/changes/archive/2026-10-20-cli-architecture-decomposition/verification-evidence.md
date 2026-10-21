# CLI decomposition verification (2026-10-20)

## Scope and source review

The pre-extraction source is recorded in `baseline-evidence.md` (HEAD
`68d36b22544f3ce5843cad53fb7f7bff6ea15eb1`, 7,564 lines). The split
reduces `cmd/offgrid/main.go` to 6 lines: the `Version` link and process entry.
Twenty-four responsibility files now contain the extracted declarations. The
new `model_search_legacy.go` holds the former search handler while the existing
`model_search.go` parser stays untouched; this avoids merging concurrent source.

An isolated Go AST comparison against the original committed `main.go` found
110/110 non-`main` top-level declarations preserved exactly after formatting.
`main()` was deliberately split into `runRoot()` and `startDefaultServer()`;
the 47 command case labels and their order match the original exactly. The
default branch still loads configuration and starts the service once. It was
not executed against the installed workspace. Package compilation detects no
duplicate symbols or unresolved declarations.

`run_chat.go` has 911 lines because the existing indivisible `handleRun`
function spans about 806 lines; it was not behaviorally split just to meet
the 600-line guideline. Other sampled larger groups are 535 lines
(`audio_command.go`), 528 (`server_command.go`), and 587
(`model_transfer.go`).

## Checks performed

- `gofmt -l` for all `cmd/offgrid/*.go`: no output.
- `go test ./cmd/offgrid -count=1`: passed.
- `go test ./...`: passed on Windows/amd64 (other packages may use Go cache).
- `go build -trimpath -o build/dev/offgrid.exe ./cmd/offgrid`: passed.
- Safe built-CLI captures for `--no-color --help`, `version`, invalid
  `--json model preview`, `chat`, and `--no-color run --help`: all five exit
  codes, output lengths, and SHA-256 hashes exactly matched the baseline.
- Root alias/static route matrix: 47/47 case labels in identical order; no
  default service or personal account route was run.
- Additional safe alias captures `bench --help`, `rm --help`, and `fav --help`
  matched the baseline binary exactly, including exit codes 1, 1, and 0.
- `node scripts/maintenance/check-repository-hygiene.mjs`: clean.
- `node --test scripts/maintenance/check-docs.test.mjs`: 6/6 passed.
- `node scripts/maintenance/check-docs.mjs`: 179 maintained Markdown files,
  zero problems.
- `git diff --check` for affected tracked paths: passed.
- `openspec validate cli-architecture-decomposition --strict --no-interactive`:
  passed; `skip_specs` is intentional for this source-only refactor.

## Ownership and remaining limits

The existing modified `computer_commands.go`, `model_package_commands.go`,
and `model_package_commands_test.go` were not rewritten. Other staged and
unstaged reorganization/provider work remains in the shared worktree. Only
two narrowly scoped hygiene commits were requested and created before this
verification: `242e6ce` adds the checker/tests; `14afccf` stops tracking the
generated CLI binary. The local `bin/offgrid` file was restored in place and
is ignored; no other user files were discarded or committed with those changes.

No installed service, desktop app, saved task, model, or account was touched.
This source verification does not qualify native desktop behavior, hosted CI,
or a release. CLI decomposition itself remains uncommitted for later review
alongside the wider reorganization.
