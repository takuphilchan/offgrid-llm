# Maintainability and generation

OffGrid uses generation at boundaries where it removes duplication. It avoids
generating the internal domain model or coupling the local runtime to a
microservice framework.

## Generated artifacts

The stable HTTP contract is `pkg/api/openapi.yaml`. Generate and verify the
React types with:

```bash
cd web/app
npm run api:generate
npm run api:check
```

`schema.generated.ts` is committed so releases do not depend on code-generation
network access. Application code should import schema types from that file and
keep behavior in focused feature modules.

## Adding an API operation

1. Define the operation and schemas in `pkg/api/openapi.yaml`.
2. Implement transport-neutral behavior in the relevant Go package or service.
3. Keep the server handler responsible for decoding, authorization, and response
   mapping only.
4. Regenerate the TypeScript schema.
5. Add Go handler tests and, for a visible workflow, a Playwright integration
   test.
6. Update capability documentation and state whether it is core, optional, or
   experimental.

## Infrastructure scripts

Build and deployment scripts must be deterministic, non-interactive in CI, and
safe to run repeatedly. Prefer a small script that invokes the real tool over a
second configuration language that duplicates package metadata.

- `go.mod` pins the Go language and toolchain versions.
- npm lockfiles pin UI and Electron dependency graphs.
- Docker builds use a multi-stage image and persistent volumes for mutable data.
- GitHub Actions checks Go tests, generated API drift, UI compilation, live
  navigation/state tests, and Electron packaging.
- Release jobs build platform-specific artifacts on the matching host.

Never generate files into a user data directory, and never make application
startup depend on a generator.

## Why the runtime does not use Go-Zero

Go-Zero is useful for cloud web/RPC microservices and provides its own API DSL
and `goctl` generation workflow. OffGrid's core is a single local process with a
long-lived inference child process, direct local storage, and desktop lifecycle
ownership. Replacing its runtime with Go-Zero would not solve the current
maintainability risks and would introduce concepts that most deployments do not
need.

The relevant ideas are still adopted: contract-first APIs, generated client
types, bounded packages, consistent middleware, and automated validation. A
future independently deployed fleet control plane can choose Go-Zero without
forcing the device runtime to use it.

## Review checklist

- Does the feature have one owner for mutable state?
- Can cancellation and shutdown reach every background operation it starts?
- Are unavailable optional dependencies represented as a capability state?
- Is an external error stable and free of internal implementation details?
- Do browser, Electron, CLI, and external API behavior agree?
- Is there a test at the lowest useful layer and one end-to-end test for a
  critical user workflow?
- Can the change be upgraded without deleting existing models or user data?
