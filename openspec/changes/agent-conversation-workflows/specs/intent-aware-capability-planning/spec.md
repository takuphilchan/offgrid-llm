# Spec Delta

## Purpose

Let agents converse, clarify and act according to the request while presenting relevant capabilities within model and authority limits.

## ADDED Requirements

### Requirement: Conversation does not require action
An agent SHALL be able to answer, clarify or propose a tool call in the same turn contract. Available tools and remembered desktop access MUST NOT require tool use, application discovery or outcome declarations. Completion checks SHALL apply to requested external outcomes and dispatched work rather than the mere availability of computer access.

#### Scenario: Greeting with Full access
- **WHEN** a user says hello with a profile, MCP connections and Full access enabled
- **THEN** the agent responds conversationally with zero tool execution, host action or required outcome declaration

#### Scenario: Mixed request
- **WHEN** a user asks a question and requests an application action
- **THEN** the response distinguishes its answer from the action and reports action completion only with matching evidence

### Requirement: Relevant authorized discovery
Discovery SHALL search all authorized capabilities and installed applications, including entries beyond initial result pages. Selection MUST use observed identities and preserve explicit user application constraints. Ambiguous matches SHALL request clarification; an unavailable requested application MUST NOT be silently replaced with another application.

#### Scenario: Settings beyond the first page
- **WHEN** Settings is outside the initial application catalog page
- **THEN** lookup can resolve its host-issued identity and does not substitute Access

#### Scenario: Composed tool workflow
- **WHEN** a task needs an MCP documentation tool and a native editor
- **THEN** both authorized capabilities remain discoverable throughout the task

### Requirement: Bounded model context
The system SHALL budget instructions, tool schemas, history, evidence and result projections against the selected model's actual context capacity. Large results SHALL retain retrievable provenance while exposing bounded relevant content. Compaction MUST retain current user constraints, outstanding actions and verification state.

#### Scenario: Large documentation result
- **WHEN** a documentation tool returns content exceeding the turn's remaining context budget
- **THEN** the model receives a bounded projection and an authorized retrieval reference without losing the original task or replaying actions

### Requirement: Outcome and execution location
Plans and verification SHALL distinguish host application work, service files and generated artifacts. Service artifact integrity MUST NOT satisfy a request to edit an application document. Unsupported native operations SHALL produce an actionable limitation before input rather than a substitute result.

#### Scenario: Unsaved multilingual document
- **WHEN** a user requests a new unsaved Notepad document containing exactly `OffGrid test — café 日本語 12345`
- **THEN** the task uses native document creation, insertion and read-back or explains unavailable capability, and neither a service file nor an artifact is reported as fulfillment

### Requirement: Relevant completion
Agents SHALL conclude a satisfied request using matching evidence without requiring unrelated tool calls. A tool's successful result alone MUST NOT prove an unrelated outcome. Plans and check declarations SHALL be generated from the user's request without a mandatory separate desired-outcome form.

#### Scenario: Application opening completed
- **WHEN** OBS was requested and its usable surface is verified
- **THEN** the agent reports the verified result without calculator or documentation calls added solely to produce its final answer
