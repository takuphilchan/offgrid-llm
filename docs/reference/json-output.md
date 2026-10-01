# JSON output and scripting

Use `--json` only with commands whose output contract you have checked.
The global flag does not make every legacy command machine-readable.
Service-aware commands share error handling; see [client contracts](../advanced/client-contracts.md).

## Useful commands

| Command | Result to inspect |
| --- | --- |
| `offgrid list --json` | `server`, `models` array, `count` |
| `offgrid list --catalog --json` | `models` catalog array |
| `offgrid search qwen --limit 5 --json` | `results` array and `count`; requires internet from the CLI |
| `offgrid download MODEL_ID --json` | Final `success`, `status`, `file_name`, `server` |
| `offgrid session list --json` | `sessions` array |
| `offgrid session show NAME --json` | Saved session, including its message array |
| `offgrid agent run TASK --model MODEL_ID --json` | Persisted task snapshot with `run_id` and `status` |
| `offgrid agent status RUN_ID --json` | Current snapshot; do not infer completion from submission |
| `offgrid workspace verify BACKUP.zip --json` | Offline verification result, not restore or schema upgrade |

A detached download's `status: accepted` means queued/accepted, not installed.
Without `--detach`, the command waits for the recorded final download result.
Progress belongs on stderr; it is not a stream of JSON progress objects on stdout.

Illustrative model-list shape, with deliberately empty data rather than invented
model sizes or filesystem paths:

```json
{"server":"http://127.0.0.1:11611","models":[],"count":0}
```

Model `size` is display text, not a numeric byte counter. Do not compare it as a
number or assume every model contains `path` or `quantization` fields.
Session `messages` is an array, not a precomputed count.

## Read results safely

PowerShell, against the configured service:

```powershell
$inventoryText = offgrid list --json
if ($LASTEXITCODE -ne 0) { throw ($inventoryText -join [Environment]::NewLine) }
$inventory = ($inventoryText -join [Environment]::NewLine) | ConvertFrom-Json
$inventory.models | Select-Object -ExpandProperty name
```

Bash, with `jq` installed separately:

```bash
if inventory=$(offgrid list --json); then
  printf '%s\n' "$inventory" | jq -r '.models[].name'
else
  printf '%s\n' "$inventory" >&2
  exit 1
fi
```

These examples only read model inventory. Do not automatically download, remove
models, or recreate a workspace because a request failed or returned an empty
list. First verify the service address, authentication, and ownership.

## Task submissions are not completion

`agent run --json` returns the accepted snapshot even when `--wait` is supplied.
Save `run_id` and inspect `agent status RUN_ID --json` or use the
[API event stream](api.md#activity-replay). Handle approval, access-needed,
interrupted, failed, cancelled, and uncertain states explicitly.

Generate and retain a request ID before submission if your script needs safe
transport retries. Use the same `--request-id` only for identical work. A new
request ID can create another task and repeat effects.

## Errors and exit status

The shared command error path emits JSON on stdout and a nonzero exit status:

```json
{"error":{"code":"invalid_usage","message":"A required argument is missing."}}
```

This is an illustrative error shape, not exact output for every command.
Service errors can also include retryability, request identity, and partial
operation details. Parse the stable code/status; do not match prose.

| Exit status | Shared command contract |
| --- | --- |
| `0` | Command succeeded; a submitted job may still be unfinished |
| `1` | Operational failure |
| `2` | Invalid usage |
| `130` | Cancellation |

Keep stderr separate from stdout. Check exit status **before** treating a parsed
object as success. Legacy optional commands are not all migrated to this contract.
A retryable error does not authorize repeating an uncertain external operation.

## Export and privacy

Session and task exports may include private prompts, results, and source/tool
details. Keep them out of public CI logs and support attachments unless redacted.
CLI export refuses an existing output file instead of silently overwriting it.
Backups contain credentials and require stronger handling; see
[workspace recovery](../advanced/workspace-recovery.md).

See [CLI reference](cli.md), [API reference](api.md), and
[history management](../guides/history-management.md).
