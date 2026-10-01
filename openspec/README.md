# Development with OpenSpec

OpenSpec keeps a proposed change, acceptance scenarios, design decisions, and
implementation tasks together. It does not replace tests or user documentation.
This repository uses **OpenSpec 1.14.0**, with the `spec-driven` schema.

## Start a change in Codex

Open this repository as your Codex project, not a sibling repository. Use these
in **Codex chat**, not a terminal:

```text
$openspec-explore Investigate <problem>; do not implement yet.
$openspec-propose <one-small-change>
$openspec-apply-change <change-name>
$openspec-verify-change <change-name>
$openspec-archive-change <change-name>
```

Review the proposal, scenarios, design, and tasks before requesting implementation.
Verification must report actual test evidence and unresolved findings. Archive
only when completion criteria are met; archiving does not commit, push, publish,
or qualify a release.

Use `$openspec-update-change <change-name>` to revise existing planning artifacts.
Use `$openspec-sync-specs <change-name>` to merge implemented requirement changes
without archiving. Neither operation is implied by an ordinary question.

Upstream skills are installed once in the owner's personal Codex skills directory,
not copied into this repository. Repository-specific constraints live in
[config.yaml](config.yaml) and [AGENTS.md](../AGENTS.md). Skills are available on a
new Codex turn; if discovery has not refreshed, restart Codex in this repository.

## Where things live

- `config.yaml`: project-specific constraints and artifact rules.
- `changes/<name>/`: in-progress change, scaffolded with the OpenSpec CLI.
- `specs/<capability>/spec.md`: maintained requirements for completed changes.
- `changes/archive/`: completed change history, not proof of a release.

Specs and changes intentionally start empty. Existing code has not been
retroactively certified or rewritten into generated requirements.

## Terminal checks

Run these from the repository root:

```sh
openspec --version
openspec list --json
openspec doctor --json
openspec validate --all --strict --no-interactive
```

In PowerShell, `openspec.cmd` is equivalent and avoids script-policy issues.
An empty workspace has no requirements to validate; this is setup status, not
application coverage.

Applicable application checks:

```sh
go test ./...
npm --prefix web/app run api:check
npm --prefix web/app run check
npm --prefix web/app run build
npm --prefix desktop test
```

Run Go checks from the root and UI/desktop checks with their existing dependencies.
Use `npm.cmd` in PowerShell if script policy blocks `npm.ps1`. Streaming, auth,
storage, browser, and native-control changes require additional relevant checks
in `.github/workflows/ci.yml`. Do not enable native-test flags against an ordinary
user desktop or start large inference downloads just to validate specifications.

## Another machine or a future project

Node.js 20.19 or newer is required. In PowerShell use `npm.cmd`; in Linux use
`npm`. Install the pinned CLI:

```sh
npm install --global @fission-ai/openspec@1.14.0 --ignore-scripts --no-audit --no-fund
openspec config set telemetry.enabled false
```

With personal OpenSpec skills already installed, initialize a new repository with
`openspec init --tools none --no-animation`, then tailor its config. Do not add
same-named project-local copies alongside the personal skills.

Without personal skills, use `openspec init --tools codex --no-animation` instead.
That is the upstream project-local integration. Include the optional verify
workflow using `openspec config profile`, then `openspec update`. Review generated
changes before committing.

On the owner's machine, Windows and WSL have separate CLI installations/settings.
Use the Windows repository path or its `/mnt/d/...` equivalent; never run
overlapping edits in both environments.

## Completion and maintenance

- Scope one reviewable increment; preserve unrelated work and compatibility.
- Reproduce defects and add regression tests.
- Record commands, environment, outcomes, and unavailable evidence with the change.
- Require relevant application tests and review. A structural specification check
  cannot authorize merging or releasing; branch protection is configured separately.
- Keep secrets, private prompts, raw private datasets, credentials, and model
  weights out of specs. Telemetry is disabled in this installation.
- Upgrade CLI and personal skills deliberately together. Review differences and
  rerun checks. `openspec update` refreshes project-generated integrations, not
  personally installed GitHub skills.
- No automatic commits, pushes, training, publication, or production approvals.

References: [OpenSpec release](https://github.com/Fission-AI/OpenSpec/tree/v1.14.0)
and [Codex integration](https://github.com/Fission-AI/OpenSpec/blob/v1.14.0/docs/supported-tools.md).
