# Interface audit: dividers, controls, Settings and localization

Scope: the owner's reported stacked/tinted rules, cramped native dropdowns,
oversized mobile navigation button, nested Settings entry, mixed-language routing
and fixed-color in-app logo. Work uses isolated HTTP fixtures and disposable
desktop profiles. No installed-user replacement, model import/download, commit,
push or publication is included.

## Findings and implementation

| Area | Finding | Result |
| --- | --- | --- |
| Models | Installed package rows closed with a rule, then the Advanced disclosure opened with another. Its full-width keyboard outline looked like another enclosure. | Rules remain between rows; the last row has no closing rule. Section headings/disclosures use spacing. Disclosure focus wraps the label, including in RTL. |
| Empty model inventory | An empty language-package section could inherit a top border. | No empty section is rendered after loading completes successfully. Loading and error states remain available. |
| Theme colors | Decorative rules reused control-border colors; Alucard's pale lavender border stood out against cream. | A separate `--separator` blends the current text color into the actual containing surface. Contrast palettes and forced colors retain stronger separation. |
| Settings forms | Decorative rules immediately under language, theme and text-size controls added clutter without identifying a useful boundary. | Form sections and appearance preferences use headings and spacing, with no bottom rules. |
| Knowledge, tools, connections, skills, schedules, activity and execution rows | Repeated closing rules could compete with the next section. | Row separators divide siblings; final-row rules are removed where the next section owns separation. |
| Markdown | Thematic breaks inherited the browser's inset border. | One explicit theme-aware rule. Tables, code blocks, blockquotes, selected tabs, progress indicators and input boundaries retain their own roles. |
| Permission panel | Its border used an unregistered token and hard-coded fallback. | It uses the existing control-boundary token. Permission behavior is unchanged. |
| Dropdowns | OS-dependent arrows were cramped; text did not follow interface text scaling. | Shared renderer/terminal CSS provides a theme-colored chevron and logical padding. Native options, type-ahead and keyboard behavior remain. Forced colors restore the system arrow. |
| Mobile navigation | A long translated navigation label occupied header space as a pill. | Compact icon button, localized accessible name and tooltip, expanded state, focus return and touch target preserved. |
| Settings entry | Clicking the gear opened a second preferences menu containing another Settings link. | Gear navigates directly to Settings and closes the drawer. A separate overflow menu contains status, Refresh and account actions. Language/appearance have one UI location. |
| In-app brand | A fixed black image tile ignored the chosen palette. | The existing layered vector inherits the current text color in navigation and login. |
| Localization | Features received the fallback language before looking up the actual selection. Native terminal/menu/error copy also contained English-only strings. | Selected-language routing reaches contextual feature catalogs with per-message fallback. The nine original locales have inventoried key parity including new terminal/menu/error drafts. |

## Audit method and boundaries

The placement pass removes 28 further rules around headings, disclosures,
follow-up forms, profile capability/resource fields, provider setup/readiness,
knowledge recovery/results/passages, comparison/evidence blocks and the start of
tool/model/document inventories. It preserves spacing rather than replacing
lines with boxes. Profile Save/Cancel has a separator only when its toolbar is
sticky. Repeated rows, tab baselines, genuine toolbar boundaries and Markdown
thematic breaks retain intentional separators. The first artifact/help row has
no opening rule. Source decisions are recorded in
`build/divider-placement-removals.json`.

`node scripts/quality/audit-dividers.mjs build/divider-source-audit.json` inventories
31 renderer/desktop style sources (including startup HTML). The current report
classifies 32 retained decorative declarations separately from control/enclosure,
focus/selection, indicator/shape and removed declarations. The classification is
a review aid, not proof that every visual state has been inspected. Generated
bundles, third-party terminal CSS and shell/program output are excluded from
source editing.

Browser checks exercise the actual populated speech-package list and Advanced
import, including keyboard-expanded and focused states, across all 23 palette
variants. An additional geometry check looks for pairs of decorative rules around
empty strips; it intentionally excludes enclosed controls, code and tables.
Main routes are captured in Swahili desktop, German at 640px/150% text and Arabic
at 390px/150% text. Empty libraries, management routes and all Settings sections
are checked separately. Screenshots inspected include cream and dark Solarized
Models, RTL Settings/Chat, enlarged Knowledge, and the separated Settings/menu UI.
No fixtures import models or write workspace data.

## Validation

- Web type-check/build passed; the build retains the existing large-chunk warning.
- Targeted divider/control/layout run: 37 passed; three reload assertions exposed
  a test fixture reseeding its preferences on reload. Corrected that fixture;
  all three dropdown/keyboard/persistence/forced-color cases then passed.
- Desktop unit tests: 195 passed, one skipped. Package verification now also
  rejects missing/mismatched shared select CSS and terminal translation assets.
- Terminal renderer harness passed locale changes, narrow/zoom geometry, forced
  colors, settings persistence and unchanged output. Evidence:
  `C:/Users/phil/AppData/Local/Temp/offgrid-terminal-display-wCY6A1`.
  This harness does not claim a real PTY test.
- Final complete browser suite against the final built renderer: **1,507 passed,
  32 skipped, zero failures** (`build/interface-final-browser.log`, 5.7 minutes).
  This run includes the broader separator placement pass, direct Settings
  navigation, native dropdowns, drawer focus fix and missing-bridge guard.
  A separate login-brand check passed explicit warm mode, RTL and forced colors
  (`build/login-brand-verified.log`).
- Packaged Windows theme/locale checks passed, including 13 native palette
  variants, all 29 dropdown options, Help/chooser and routes at native 200% zoom,
  and saved theme/language after restart. Evidence:
  `C:/Users/phil/AppData/Local/Temp/offgrid-desktop-theme-k7YKAA`.
- Packaged Windows startup/recovery checks passed with a disposable profile.
  Evidence: `C:/Users/phil/AppData/Local/Temp/offgrid-desktop-startup-5WybwA`.
- Source whitespace checks and strict OpenSpec validation passed. Translation
  validation passed with the incomplete-preview counts stated below.

## Verified local build

`build/interface-final-20261111/win-unpacked/OffGrid LLM Desktop.exe`

Archive SHA-256:
`59ce52f38265944b33c2586d59759c81e51ef3b1d0f66ee340e06020d65598fb`.
The copied archive's five presentation assets match source, and the package
verification hooks passed. Native checks above used this executable. Older
personalization installers predate this audit. This candidate has not been
installed over the user's application or published.

## Translation completion remains open

The quality gate covers 135 catalogs / 1,319 messages. Each of the 20 global
preview packs currently translates 90 unique messages, with 1,229 still falling
back to English. The inventory includes core/features, voice/computer, desktop
presentation, folder-trust and terminal copy; it does not inventory all CLI
output, server diagnostics or OS-owned menus. Core-only coverage is no longer
presented as app-wide coverage. Source snapshots reject stale translation source
text and removal of previously translated keys. User content is never translated.

Neither key parity nor these checks certifies language quality. Contextual and
native-speaker review remains pending, including the original-language additions.
Preview packs must not be promoted until complete translation and review. Human
screen-reader evaluation and macOS/Linux native checks are also unrun.
