# Agents refinement — 2026-11-15

This resumes `workspace-outcome-experience` tasks 44.36–44.37 after the document
reader work. It changes the shared renderer, not the installed desktop binary,
service permissions, model runtime or user workspace. Earlier Library, streaming
and permanent-deletion implementation evidence remains in
[the reader/Library record](reader-library-flow-2026-11-15.md).

## Implemented behavior

- Profile search and archived visibility sit with the profile list. Save and Use
  in Chat sit with the editor heading. Instructions, a labelled compact model
  control, optional resources and secondary maintenance actions form distinct
  groups. Examples edit only the unsaved instructions.
- Archived profiles are read-only and identified in the list. They are excluded
  from new delegate selections; previously selected unavailable delegates remain
  removable. Draft preservation, revision conflicts and reviewed permanent
  deletion retain their existing contracts.
- Tools and MCP connections use bounded cards and semantic icons. Exact tool
  identities remain inspectable, and dynamic service descriptions retain their
  own text direction. Icons do not classify risk or grant access.
- Tool/connection mutations are serialized. Failed tool changes leave the old
  value visible; connection failures retain the name and URL. Successful setup
  closes the form and returns focus to its trigger.
- External-agent cards distinguish model compatibility, unchecked host setup and
  unverified workflows. Setup remains copy-only. A clipboard failure is reported
  inside the open dialog rather than behind it; Escape restores trigger focus.
- Fixed a stretched archive checkbox, an inherited last-card border removal and
  a narrow-layout rule that placed connection deletion on its own row. Reduced
  motion follows the shared application and OS preference.

## Verification

Windows, isolated Playwright Chromium fixtures on port 5187, two workers. No
installed service fallthrough or native desktop control was used. Temporary files
and npm cache were directed to the existing D: build directories.

The scoped renderer command was:

```powershell
npm exec --prefix apps/web -- playwright test --config apps/web/playwright.config.ts tests/agent-profiles.spec.ts tests/agent-management-layout.spec.ts tests/agent-management-refinement.spec.ts tests/mcp-connections.spec.ts tests/external-integrations.spec.ts tests/agent-availability.spec.ts --workers=2
```

There are **99 unique scoped tests with passing results across the main run and
corrected reruns**, not a claimed single all-green full renderer suite:

| Run / log under `build/` | Result |
| --- | --- |
| `agents-refinement-tests.log` | Initial narrower run: 92 passed, one new test had an incorrectly nested locator. |
| `agents-verified-tests.log` | Expanded run: 95 passed, four new assertions failed. |
| `agents-final-regressions.log` | All four corrected assertions passed. |
| `agents-setup-tests.log` | Four existing layout tests, extended with setup-dialog screenshots and focus/overflow checks, passed. |
| `agents-refinement-build.log` | TypeScript and production Vite build passed; existing large-chunk warning remains. |
| `agents-i18n-check.log` | Translation structure/coverage check passed. |
| `agents-i18n-complete.log` | 1,551/1,551 authored app messages for every preview language; zero missing. Native review remains pending. |
| `agents-i18n-consistency.log` | Passed; no shared-navigation conflicts. Contextual wording variants remain review items. |
| `agents-spec-validation.log` | Strict OpenSpec validation passed. |

The four corrected assertions concerned a disappearing checkbox (use a click
then assert removal, rather than waiting for the removed node to become unchecked)
and three motion checks (the shared policy uses a near-zero duration to retain
transition events). They did not require weakening application behavior.

Coverage includes profile optional-resource/draft flows and purpose examples in
all 29 locales, archived delegate removal, save conflicts, profile deletion
review/block/conflict, MCP removal/retry, failed tool mutation, failed connection
setup/retry, clipboard failure/retry, identity/storage loss and unsupported-service
recovery. New visual/keyboard cases cover desktop light/dark and narrow Arabic.

Rendered screenshots were inspected in:

- `build/ui-craft/agents-verified`: tools, connection cards, external providers,
  profile editing, light/dark and narrow layouts.
- `build/ui-craft/agents-final-regressions`: final new-profile controls, compact
  model selection and narrow RTL.
- `build/ui-craft/agents-setup-verified`: setup dialogs at 390 and 1440 pixels in
  light/dark, including wrapped commands and keyboard focus restoration.

## Boundaries

These are renderer fixtures, not real-model, external-provider or native-control
qualification. Server-provided tool descriptions, provider notes and user/model
names are data, not newly authored interface translations. No native-language
approval, physical 8 GB/16 GB qualification, new packaged/offline installation,
commit or publication occurred. The earlier installed preview is unchanged.
