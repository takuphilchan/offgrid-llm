# Exact JSON text

`Validate` runs before decoding tool arguments or private computer-control frames.
Go's standard JSON decoder replaces malformed Unicode with U+FFFD and accepts
repeated object keys, keeping the last value. Those conversions are inappropriate
for exact document text, payload identity, and approved actions.

This package rejects malformed UTF-8, unpaired escaped UTF-16 surrogates, and
duplicate keys (including equivalent escaped keys) at every nesting level. It
preserves valid text without normalization: literal U+FFFD, combining characters,
line endings, non-BMP characters, and literal backslash sequences remain valid.
Numbers are not converted to floating point.

This is a wire-integrity check, not authorization, a schema validator, or a claim
that native editing works. Callers must still enforce their existing frame limits,
closed argument shapes, permissions, target bindings, and dispatch journal.

Checks: `go test ./internal/jsonwire ./internal/agents ./internal/computer`.
The fuzz target exercises arbitrary input without starting an application or
performing native input.
