# Spec Delta

## Purpose

Make operation phases, control labels, processing context, and recovery actions
understandable and truthful in OffGrid's shared browser and desktop workspace.

## ADDED Requirements

### Requirement: Access actions describe their next effect
The workspace SHALL distinguish discovering applications, opening an installed
application, and consenting to access. Discovery MUST NOT imply that access has
already been granted or the saved task has resumed.

#### Scenario: Initial application access
- **WHEN** a saved task requests application access and no window list is loaded
- **THEN** the primary action says Choose an application and only discovers targets
- **AND** Allow access and continue appears only after a target can be selected

#### Scenario: Empty installed application list
- **WHEN** the installed application picker contains no targets
- **THEN** the workspace explains the empty state and does not dispatch a launch
  or grant access to a different target

### Requirement: Read recovery targets the failed operation
The workspace SHALL associate a recoverable read failure with the exact read and
its inputs. Polling and unrelated successful reads MUST NOT erase that failure.
Recovery MUST NOT replay installations or other mutations.

#### Scenario: Search fails
- **WHEN** a speech model search for a partial name fails
- **THEN** Retry search repeats the same search and displays its results
- **AND** refreshing installed models alone does not claim that search recovered

#### Scenario: Repository inspection fails
- **WHEN** inspecting or resolving a selected repository fails
- **THEN** retry repeats that inspection with its original inputs without starting
  an installation

#### Scenario: Read is superseded
- **WHEN** a newer search replaces a pending read or the user leaves the category
- **THEN** late responses cannot replace the new state and cancellation is not
  presented as a network failure

### Requirement: Microphone phases match actual work
The workspace SHALL distinguish readiness checking, permission waiting, recording,
and transcription. Cancellation MUST remain available during asynchronous phases
and MUST release late-arriving media without inserting stale transcripts.

#### Scenario: Permission is pending
- **WHEN** recognition is available and microphone consent has not resolved
- **THEN** the interface requests permission rather than claiming to transcribe
- **AND** cancelling prevents late permission results from starting recording

#### Scenario: Recording and transcription
- **WHEN** capture is active and then stopped
- **THEN** Stop recording ends capture and the label changes to Transcribing
- **AND** cancellation during transcription aborts the request without changing
  a newer draft or another composer

### Requirement: Knowledge readiness is not conflated with failure
The workspace SHALL distinguish checking, ready, unavailable, and failed Knowledge
checks. Ordinary chat MUST NOT display a permanent unavailable warning for unused
Knowledge. Requested Knowledge MUST NOT silently become ordinary inference.

#### Scenario: Initial check
- **WHEN** Knowledge readiness has not resolved
- **THEN** the control reports checking, remains unavailable for selection, and
  does not display a failure message

#### Scenario: Check fails or Knowledge is disabled
- **WHEN** a readiness read fails or returns disabled
- **THEN** the UI distinguishes retrying the check from configuring Knowledge
- **AND** typed chat remains usable when Knowledge was not requested

### Requirement: Processing claims remain evidence based
The shared workspace SHALL identify the connected service without claiming that
all processing or storage occurs on the client computer. Loopback addresses alone
MUST NOT establish physical device locality. New feedback copy MUST exist in all
nine interface locales.

#### Scenario: Service location is not established
- **WHEN** the renderer has no verified physical-device locality information
- **THEN** shell, empty-chat, authentication, and speech-preparation text refer to
  the connected OffGrid service/workspace without unconditional on-device claims
- **AND** service details remain discoverable
