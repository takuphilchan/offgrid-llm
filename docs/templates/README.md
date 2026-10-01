# Writing and maintaining documentation

User documentation belongs in this repository alongside the implementation.
Start with the reader's task, not the internal module layout. Existing guides
remain authoritative; change proposals belong in [OpenSpec](../../openspec/README.md).

## Choose the right home

| Content | Location |
| --- | --- |
| First install, upgrade, platform setup | `docs/setup/` |
| A user task, including failures and recovery | `docs/guides/` |
| API/CLI contracts | `docs/reference/` |
| Architecture, operations, qualification | `docs/advanced/` |
| Historical release facts | `docs/releases/` |
| Contributor commands | [Contributing](../../dev/CONTRIBUTING.md) |

Use lowercase-kebab filenames for new guides. Preserve established filenames
and exact Git casing when linking: for example,
[ARCHITECTURE.md](../advanced/ARCHITECTURE.md). Windows may accept a wrong-case
link that fails on Linux or GitHub.

Templates are optional starting points, not required section counts:

- [Guide template](guide-template.md): a walkthrough.
- [Feature template](feature-template.md): capability and limitations.
- [API template](api-template.md): contract-oriented documentation.

Remove placeholder sections and illustrative links before publishing a guide.

## Make instructions usable

- State who the guide is for and what success looks like.
- Give prerequisites, working directory, shell, and expected service state.
- Distinguish PowerShell from Bash; do not give Unix environment syntax as a
  Windows command. Use `npm.cmd` when PowerShell policy blocks `npm.ps1`.
- Explain placeholders such as `YOUR_MODEL_ID`; use IDs from `offgrid list`.
- Put warnings before commands that affect installed data, credentials, or services.
- Explain expected output and the next step if it fails. Never invent benchmark
  numbers, timing, signing, or hardware evidence.
- Link to one canonical installation/recovery procedure rather than copying it
  into every feature guide.
- Distinguish current implementation, preview limitations, and planned work.
  An installed model or successful compilation is not workflow qualification.
- Keep headings short, include blank lines before lists, and label code fences.
  Add a contents list only when it improves navigation.

Use screenshots only when they explain something the text cannot. Remove
credentials and personal data, include useful alt text, and capture the current
monochrome UI. Do not alter product colors to make documentation attractive.

## Verify a documentation change

From the repository root with Node.js 22:

```sh
node --test dev/scripts/check-docs.test.mjs
node dev/scripts/check-docs.mjs
git diff --check
```

The offline checker validates local link targets with exact Git casing and code
fence closure in maintained Markdown. It excludes historical release notes and
illustrative templates. It does **not** check external URL availability, heading
anchors, command behavior, or prose accuracy; review those separately.

Check commands against their actual parser/API and run relevant tests in
disposable state. Record what ran and what could not be tested. Never run a
destructive example against the user's normal workspace just to verify a guide.
