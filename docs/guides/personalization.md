# Personalize OffGrid

The sidebar gear opens Settings directly. The separate three-dot menu contains
workspace status, Refresh and account actions; language and appearance live in
Settings rather than a second preferences menu.

In Settings > General, keep System, Light or Dark as the appearance mode. Below
it, choose separate light and dark palettes. The collection includes OffGrid,
Slate, Warm, Forest, High contrast, Catppuccin (Latte/Mocha), Tokyo Night
(Day/Night), Gruvbox, Solarized, Rosé Pine (Dawn/Main), Nord (dark), and Dracula
(Alucard/Dark): 12 families, 23 variants. They are OffGrid adaptations with
contrast adjustments, not endorsed upstream ports. Preview applies to the
current view; Apply remembers it. Select either saved-theme summary to open the
searchable chooser, use its Light/Dark tabs, then select a workspace preview.
Cancel or Escape restores the saved selection and returns focus to the summary.
Reset appearance
restores the original palettes, 100% text scale and system motion behavior. It
does not reset language, terminal preferences, conversations, models or grants.

Text size and Reduce motion save as soon as they change. Text size adjusts the
shared interface typography tokens. Browser/desktop zoom
remains available for full-page enlargement. Reduce motion can disable interface
animation even when the operating system allows it; the OS reduced-motion choice
is always respected.

Settings search includes direct links to language, appearance, text size, motion,
responses, voice, terminal, access and service details. The command palette also
opens language, appearance and text-size settings. For example,
`#/settings/text-size` opens and focuses text size. Existing read-only diagnostics
remain read-only.

Preferences apply to this browser/device. Chat response preferences retain their
connection scope; agent profiles keep their own settings. If storage is unavailable,
changes can work for the current session and a save warning appears. Matching
desktop builds also persist normalized appearance preferences and share palette
surfaces, cursor/selection and ANSI colors with the integrated terminal. Native
window backgrounds and Windows caption controls follow the saved palette.
Terminal font/cursor/scrollback preferences
remain separate. No terminal output or user data is sent to a model by these controls.

New interface copy is supplied for all nine existing languages. These additions
are translation drafts pending native-language review. Twenty additional packs
are available in the **Preview** group: Brazilian/European Portuguese, Italian,
Russian, Ukrainian, Turkish, Simplified/Traditional Chinese, Japanese, Korean,
Hindi, Bengali, Tamil, Urdu, Persian, Hebrew, Indonesian, Vietnamese, Thai and
Filipino. These currently have drafts for 139 of 1,319 inventoried messages, including
navigation, main management screens, settings and theme controls. Missing messages fall back individually
to English, so most advanced content remains English. The selector shows a native-language
notice; previews are never enabled automatically from browser language.
They are not complete or independently reviewed translations.

The in-app OffGrid mark follows the selected theme. Decorative dividers blend
with their containing surface; control borders, selected tabs and keyboard focus
remain separate. High contrast and system forced colors preserve stronger rules.
Forms and disclosures use headings and spacing; dividers are reserved for useful
row and toolbar boundaries. Native dropdowns retain keyboard selection and follow
text size and right-to-left layout.

Regional tags preserve Chinese scripts and Portuguese regions; date/number
formatting uses the selected language. Urdu, Persian and Hebrew use RTL layout.
No additional fonts are downloaded; glyph coverage uses installed system fallbacks.
Interface language does
not change an installed model's language capability or automatically select a
speech model. Human screen-reader, installed-platform and full visual-state
qualification remain separate from automated checks.

Maintainers should follow the [translation quality process](../development/localization.md)
before promoting preview packs. Theme sources and license notices are bundled in
`apps/desktop/theme-NOTICE.txt` and the web build's `theme-NOTICE.txt`.
