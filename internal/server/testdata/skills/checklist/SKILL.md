---
name: offgrid-checklist-test
description: Create a short practical checklist when the user explicitly asks to use offgrid-checklist-test. Load this skill's instructions before composing the checklist.
---

# OffGrid checklist test

Use this skill only when explicitly requested by name. This is a text-only
formatting exercise. It does not authorize file access, commands or other tools.

When the user asks for a checklist on a topic:

1. Start the response with exactly this heading: OFFGRID SKILL CHECK: 7K2
2. Write exactly three useful checkbox items about the requested topic. Start
   each item with `- [ ]` and keep each item to one sentence.
3. Finish with one sentence beginning `Next step:` suggesting a small first action.
4. Return the checklist directly in chat. Do not create a file, run a command,
   request workspace access, or claim to have performed the checklist.

If the topic is missing, ask which topic the checklist should cover.
For unrelated requests, respond normally without the test heading or template.
