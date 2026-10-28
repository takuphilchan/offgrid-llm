## MODIFIED Requirements

### Requirement: Authority and execution location remain separate

Computer access MUST NOT grant service-file or arbitrary local execution authority. Task-first discovery and dispatch SHALL exclude unreviewed local execute tools. Reviewed execution-provider tools SHALL require their own explicit actor/job-bound execution grant. Host paths MUST NOT be silently translated into service paths. Legacy computer-only tasks SHALL retain their narrower tool surface.

#### Scenario: Script hidden under another name
- **WHEN** a task-first model requests an unreviewed local execute tool not named shell
- **THEN** the tool is absent from discovery and denied before dispatch

#### Scenario: Host path after computer access
- **WHEN** a service-file tool receives a recognizable foreign host path while a computer session exists
- **THEN** the service file operation does not run and the model receives guidance to use an independently authorized provider without translating the path

#### Scenario: Legacy session
- **WHEN** an existing computer-only run without task-first semantics resumes
- **THEN** registry tools are not added to its original scope

#### Scenario: Approved workspace execution
- **WHEN** a job has a current explicit execution grant in addition to computer access
- **THEN** reviewed file/process tools are discoverable and dispatch rechecks the execution grant independently of computer consent
