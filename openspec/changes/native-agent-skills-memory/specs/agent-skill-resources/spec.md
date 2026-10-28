# Spec Delta

## Purpose

Make reusable, user-reviewed agent instructions available as owned, immutable
context resources without treating imported content as execution authority.

## ADDED Requirements

### Requirement: Owned immutable skill revisions
The service SHALL import bounded user-supplied skill files into actor/workspace-owned immutable revisions with content digests. Profile attachments MUST pin a revision and digest. Invalid paths, duplicate entries, invalid metadata and oversized imports MUST be rejected before persistence.

#### Scenario: Valid offline import
- **WHEN** a user imports a skill with valid SKILL.md metadata and relative supporting files
- **THEN** the service returns an owned immutable revision and digest without downloading or executing anything
- **AND** later imports cannot change that revision

#### Scenario: Unsafe import
- **WHEN** an import contains traversal, an absolute path, duplicate file names, invalid UTF-8 instructions or excessive content
- **THEN** the complete import is rejected without partially publishing a resource

### Requirement: Progressive authorized loading
Tasks SHALL receive bounded metadata for explicitly attached skill revisions and load their instructions/resources only through the canonical runner. Each load MUST recheck ownership, attachment and content integrity. Skill content MUST be labelled untrusted context and cannot expand tools, folders, grants or budgets.

#### Scenario: Selected revision
- **WHEN** an owned profile pins an earlier revision while a newer revision exists
- **THEN** its task loads the pinned content rather than silently replacing it

#### Scenario: Forged reference
- **WHEN** a task requests another actor's skill or an unattached revision
- **THEN** the resource is not disclosed or executed

### Requirement: Non-executable skill drafts
Agents SHALL create bounded inspectable skill drafts with task provenance. Drafting MUST NOT activate, attach or execute a skill. Import and review controls SHALL be available through authenticated service, CLI and shared application interfaces.

#### Scenario: Script inside a skill
- **WHEN** an imported resource or agent draft contains a script or allowed-tools hint
- **THEN** it remains data and grants no execution permission
- **AND** activation requires an explicit user review action

### Requirement: Compatible resource failure
Existing profiles and tasks SHALL gain no attachments automatically. Missing or corrupt skill resources MUST produce an actionable error without granting replacement authority or erasing saved work.

#### Scenario: Resource unavailable
- **WHEN** a pinned skill cannot be read or fails its digest check
- **THEN** the task cannot silently substitute another revision and its saved state remains inspectable
