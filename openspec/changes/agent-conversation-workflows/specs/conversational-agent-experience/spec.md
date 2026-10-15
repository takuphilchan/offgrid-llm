# Spec Delta

## Purpose

Provide one clear conversation surface for agent work and a reusable configuration home, consistently across desktop, browser and terminal clients.

## ADDED Requirements

### Requirement: Profiles in Agents and work in Chat
Agents SHALL provide profile management and Chat SHALL provide profile selection, conversation and work. Job details SHALL remain a secondary inspector for the same canonical work. Existing job links MUST remain inspectable without restoring the retired agent page or resubmitting their prompts.

#### Scenario: Configure and use an agent
- **WHEN** a user creates a profile, attaches an authorized connection and selects it in Chat
- **THEN** subsequent turns show the selected agent and use its pinned configuration with activity displayed inline

### Requirement: Clear conversation hierarchy
User messages, assistant output and concise work activity SHALL form the primary reading flow. Tool payloads, context statistics and detailed verification SHALL be expandable. Manual outcome input MUST appear only for unresolved recovery, with the affected action explained, and MUST NOT be a normal task-completion requirement.

#### Scenario: Completed task
- **WHEN** requested work is verified
- **THEN** Chat presents its result and useful attachments with collapsed technical activity and no required outcome textarea

### Requirement: Accessible control surfaces
Controls SHALL follow existing monochrome design tokens and button conventions. Popovers MUST dismiss on outside click and Escape, restore focus, support keyboard and assistive technology, and fit short or narrow screens. Conversation and configuration SHALL support both themes, all nine locales, RTL and IME input.

#### Scenario: Short screen with keyboard input
- **WHEN** a keyboard user opens Tools on a short screen and presses Escape
- **THEN** all controls were reachable by scrolling, the popover closes and focus returns to its trigger without submitting text

### Requirement: Persistent task controls
Active computer work SHALL retain visible Pause, Take over and Stop controls across navigation. UI acknowledgment MUST distinguish a requested stop from confirmed stopped input. Voice input SHALL submit and steer the same work; speech MUST NOT grant additional access and narration SHALL remain parent-owned.

#### Scenario: Stop from another page
- **WHEN** a user navigates away from an active conversation and requests Stop
- **THEN** the control remains available and the UI shows stopping until the execution authority confirms stopping

### Requirement: Shared client contracts
CLI and Python clients SHALL manage profiles and submit or inspect the same conversations and canonical jobs through authenticated service APIs. Existing commands and endpoints MUST remain compatible adapters. Non-interactive clients SHALL expose structured pending-access, uncertainty and failure states without silently granting access.

#### Scenario: Cross-client inspection
- **WHEN** a CLI user inspects a job initiated in Chat
- **THEN** its identity, status, results and available recovery actions match Chat and inspection executes no work
