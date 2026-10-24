# Experimental file verification hooks

Install only after reviewing this executable bundle, then explicitly enable it:
`hermes plugins enable offgrid-verifier`.

The matching OffGrid CLI activates hooks only for a foreground `--verify-contract`
invocation. Ordinary Hermes runs have no verifier hooks. This bundle does not
grant tool permissions, replace tool results, or verify arbitrary tasks. The CLI
rejects unsupported runtime revisions and independently checks final files.
