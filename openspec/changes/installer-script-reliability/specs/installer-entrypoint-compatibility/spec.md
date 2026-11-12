## ADDED Requirements

### Requirement: Alternative desktop downloads are verified
Both desktop installer scripts SHALL verify each payload against exactly one matching SHA-256 entry from the same release before extraction or execution and fail on unavailable, missing, duplicate, or mismatched checksums.

#### Scenario: Invalid download
- **WHEN** a checksum is missing, ambiguous, or does not match the downloaded payload
- **THEN** that payload is neither extracted nor executed and installation reports failure

#### Scenario: Verified download
- **WHEN** the checksum matches
- **THEN** installation may proceed for that payload

### Requirement: Windows CLI installation preserves the complete bundle
The Windows script SHALL copy the complete extracted CLI payload and use user-local installation and user PATH for an unelevated caller.

#### Scenario: User installation
- **WHEN** a non-administrator installs the CLI
- **THEN** runtime executables, DLLs, UI and accompanying files are retained under LocalAppData and machine PATH is unchanged

#### Scenario: Administrator installation
- **WHEN** an administrator installs the CLI
- **THEN** the full payload is installed in Program Files and machine PATH is updated without duplicate entries

### Requirement: Alternative installers preserve running work
The Unix script SHALL refuse CLI replacement while OffGrid is running without stopping processes. Both scripts SHALL report installation errors without a false success message.

#### Scenario: Active service
- **WHEN** the Unix CLI installation detects a running OffGrid process
- **THEN** it stops with instructions to finish work and close the service normally

#### Scenario: Desktop installer failure
- **WHEN** the Windows setup process exits unsuccessfully
- **THEN** the script reports failure
