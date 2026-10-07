# Find models on Hugging Face

OffGrid can discover public GGUF repositories beyond its curated catalog.
Use [Models → Find more models](model-discovery.md#language-and-embedding-models) for the
guided workflow. This page covers CLI selection and its limits, not publisher
endorsement or current model rankings.

## Search and choose a file

```sh
offgrid search qwen --limit 5
offgrid search qwen --quant Q4_K_M --files --limit 5
offgrid search llama --ram 16
offgrid search qwen --sort modified --limit 5 --json
```

CLI search currently connects to Hugging Face from the CLI process. Web search
and accepted downloads use the service environment. A Windows CLI succeeding
does not prove a container can reach the same upstream host.

Useful options:

| Option | Meaning |
| --- | --- |
| `--author NAME` | Filter publisher |
| `--quant TYPE` | Filter quantization |
| `--files` | Show individual GGUF files and download commands |
| `--ram GB` | Optional rough estimate filter, not a memory guarantee |
| `--limit N` | 1–50 results; default 20 |
| `--sort FIELD` | `downloads`, `likes`, `created`, `modified`, or `relevance` |
| `--all` | Include gated metadata; does not authenticate gated downloads |
| `--json` | Machine-readable results; progress stays on stderr |

Read the model card/license and choose a supported file. Check total memory and
runtime architecture, not only a repository's popularity. See
[choosing models](models.md).

## Download the exact selection

Start/connect to the intended OffGrid service. Replace these placeholders with
a repository and filename returned by discovery:

```sh
offgrid download OWNER/REPOSITORY --file MODEL.gguf
offgrid list
```

`download-hf` is a compatibility alias for the same download command.
`--quant` can narrow choices, but multiple matching files produce guidance to
select explicitly; do not expect an interactive file-selection menu.
`--detach` returns after acceptance, not installation.

Public discovery does not support private/gated downloads. Split GGUF weights
and projectors are identified separately; one file does not establish a complete
multimodal installation. Do not bypass these checks by changing extensions or
supplying authentication in a repository URL.

## Recover a transfer

Use the [download phases and recovery guide](model-discovery.md#language-and-embedding-download-recovery).
Cancellation preserves resumable partial bytes; reuse the same repository/file
identity. Disk errors, permissions, Windows file locks, upstream access, and
network failures require different repairs.

Keep required VPNs enabled. Correct routing/certificates rather than disabling
TLS verification. Never run an arbitrary script from a model repository to make
a download work. Installing model weights does not authorize executing remote code.

After installation, check actual inference and the intended workflow.
Benchmark numbers and memory requirements are meaningful only for recorded
model/runtime/hardware profiles; see [performance](../advanced/PERFORMANCE.md).
