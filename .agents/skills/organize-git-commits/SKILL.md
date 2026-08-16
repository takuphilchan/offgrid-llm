---
name: organize-git-commits
description: Inspect a Git working tree, separate changes into coherent reviewable commits, propose commit messages, stage only intended files or hunks, run proportional validation, and create commits when explicitly requested. Use for dirty-worktree cleanup, splitting mixed changes, preparing commits, committing completed work, or reviewing how changes should be grouped before a pull request.
---

# Organize Git Commits

Create a clear history in which each commit represents one complete intent and can be reviewed or reverted independently. Preserve all user work and keep unrelated changes out of commits.

## Establish scope

1. Read repository instructions such as `AGENTS.md` and contribution guidance.
2. Inspect without mutating:

   ```bash
   git status --short --branch
   git diff --stat
   git diff
   git diff --cached
   git log -10 --pretty=format:%s
   ```

3. Treat every existing modification and untracked file as user-owned. Do not discard, overwrite, restore, or reformat unrelated work.
4. Infer commit-message conventions from recent history. Use Conventional Commits only when the repository uses them or no clear convention exists.
5. Check `git config --get user.name` and `git config --get user.email` before committing. If either is missing, ask the user for the values; never invent an identity.

## Decide whether mutation is authorized

- For requests to review, organize, suggest, or plan commits, return a commit plan only. Do not stage or commit.
- For requests that explicitly say to stage, commit, create commits, or execute the plan, staging and `git commit` are authorized.
- Do not amend, rebase, squash, reset, push, create tags, or change branches unless the user explicitly requests that operation.

## Build the commit plan

Group by intent, not merely by directory or file type:

- Keep implementation, its focused tests, and directly corresponding documentation together.
- Keep dependency manifests with their lockfiles.
- Separate independent security fixes, refactors, features, build changes, and documentation changes when each can stand alone.
- Put prerequisite commits before consumers.
- Isolate mechanical formatting or generated-file churn when it would obscure behavioral changes.
- Avoid commits that knowingly leave the repository uncompilable unless an unavoidable migration requires an explicitly documented sequence.

Present the plan before execution when grouping is ambiguous or changes span several independent concerns. For each proposed commit, list its intent, included paths or hunks, validation, and proposed message.

## Stage precisely

1. Prefer explicit paths:

   ```bash
   git add -- path/to/file another/file
   ```

2. Never use `git add .`, `git add -A`, or `git commit -a` unless every worktree change has been reviewed and belongs to the same authorized commit set.
3. When one file contains multiple intents, stage selected hunks with `git add -p` in an interactive environment. In a non-interactive environment, construct and apply an index-only patch, then verify it. Never edit or discard the unstaged working-tree portion to manufacture a split.
4. Include untracked files only after reading or identifying their purpose. Exclude secrets, credentials, local configuration, caches, logs, build outputs, downloaded models, and unexpectedly large binaries unless explicitly intended.
5. After staging, inspect the exact snapshot:

   ```bash
   git diff --cached --stat
   git diff --cached
   git diff --cached --check
   ```

If the staged diff contains unrelated material, unstage only the mistaken paths or hunks without altering their working-tree content.

## Validate and commit

1. Run the smallest reliable validation that covers the staged change. Prefer repository-provided commands. Expand to broader tests for shared infrastructure, security-sensitive code, dependencies, or release configuration.
2. Do not hide test failures. Fix them only when the fix belongs to the authorized task; otherwise report the blocker.
3. Write a concise imperative subject, normally no more than 72 characters. Explain motivation and material tradeoffs in the body when the subject is insufficient.
4. Commit without bypassing hooks:

   ```bash
   git commit -m "<subject>"
   ```

5. Verify the result and reassess the remaining tree:

   ```bash
   git show --stat --oneline --decorate HEAD
   git status --short
   ```

Repeat staging, validation, and committing for each planned group. Recalculate later groups if an earlier commit changes their dependencies.

## Report completion

Report:

- Each new commit hash and subject.
- Validation run for each commit and whether it passed.
- Any changes intentionally left staged, unstaged, or untracked.
- Any follow-up work or unresolved failure.

Do not claim the working tree is clean without checking it. Do not push unless explicitly requested.
