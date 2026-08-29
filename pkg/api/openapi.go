package api

import _ "embed"

// OpenAPISpec is the versioned API contract served by the runtime and used to
// generate client types. Keeping it in this package makes the contract part of
// every CLI, desktop, and container build.
//
//go:embed openapi.yaml
var OpenAPISpec []byte
