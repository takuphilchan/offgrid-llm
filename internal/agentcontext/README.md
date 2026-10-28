# Agent context resources

These validators handle data, not tools or execution grants. Imports contain an
explicit list of portable relative files: at most 64 files, 64 KiB per file,
512 KiB total, and 32 KiB for UTF-8 `SKILL.md`. Required Agent Skills name and
description frontmatter is validated. Aliases, duplicate YAML keys, path
traversal and case-folded file collisions are rejected. Imported scripts and
`allowed-tools` are inert content. Nothing is downloaded or executed.

The canonical digest covers sorted paths and exact bytes, not filesystem paths
or caller-controlled permission metadata. The agent manager owns immutable
revisions, current ownership checks and authenticated access. Loading a skill
does not install software or widen an existing job's permissions.

Automatic profile memory is separate from transcript compression. It defaults
off; each newly admitted root job pins a live consent revision and every access
checks it again. Disable/re-enable never reauthorizes an old task. Entries are
bounded to 8 KiB each, 128 entries and 512 KiB per actor/workspace/profile; model
retrieval is bounded to 16 KiB of labels/content. Writes use stable operation
identities and expected revisions, with consent and mutation in one SQLite
transaction. A conflict never overwrites newer content. Deletion clears live
text and cached write response bodies and retains tombstone identity/digests;
it is not a promise of forensic erasure from SQLite pages/backups or historical
task evidence. User inspection/deletion is separate from model authority.
