# JSON Output Mode

**Feature**: Machine-readable JSON output for automation and integration

## Overview

All major `offgrid` commands support the `--json` flag, which outputs structured JSON data instead of human-readable text. This is perfect for:

- **CI/CD Pipelines**: Integrate model management into build systems
- **Monitoring Dashboards**: Extract metrics for visualization
- **Automation Scripts**: Parse output programmatically  
- **Integration with Other Tools**: Pipe data to jq, Python, etc.

## Supported Commands

### 1. List Models

```bash
offgrid list --json
```

**Output:**
```json
{
  "count": 1,
  "models": [
    {
      "name": "Llama-3.2-3B-Instruct-Q4_K_M",
      "size": "1.9 GB",
      "quantization": "Q4_K_M",
      "format": "gguf",
      "path": "/var/lib/offgrid/models/Llama-3.2-3B-Instruct-Q4_K_M.gguf"
    }
  ]
}
```

### 2. Search Models

```bash
offgrid search llama --limit 2 --json
```

**Output:**
```json
{
  "count": 2,
  "results": [
    {
      "name": "bartowski/Llama-3.2-3B-Instruct-GGUF",
      "downloads": 224024,
      "likes": 172,
      "tags": ["gguf", "llama", "text-generation"],
      "model_id": "bartowski/Llama-3.2-3B-Instruct-GGUF"
    }
  ]
}
```

### 3. List Sessions

```bash
offgrid session list --json
```

**Output:**
```json
{
  "count": 2,
  "sessions": [
    {
      "name": "test-project",
      "model_id": "Llama-3.2-3B-Instruct-Q4_K_M",
      "messages": 4,
      "created_at": "2025-11-10T10:00:00Z",
      "updated_at": "2025-11-10T10:01:10Z"
    }
  ]
}
```

### 4. System Info

```bash
offgrid info --json
```

**Output:**
```json
{
  "version": "0.1.5-alpha",
  "config": {
    "port": 11611,
    "models_dir": "/var/lib/offgrid/models",
    "max_context": 4096,
    "threads": 4,
    "max_memory": 4096,
    "p2p_enabled": false
  },
  "models": {
    "count": 1,
    "installed": [...]
  },
  "system": {
    "cpu": "6 cores",
    "memory": "12249828 kB",
    "os": "linux",
    "architecture": "amd64"
  }
}
```

## Usage Examples

### 1. Count Installed Models

```bash
offgrid list --json | jq '.count'
# Output: 1
```

### 2. Get Model Names Only

```bash
offgrid list --json | jq -r '.models[].name'
# Output: Llama-3.2-3B-Instruct-Q4_K_M
```

### 3. Check if Specific Model Exists

```bash
offgrid list --json | jq '.models[] | select(.name == "Llama-3.2-3B-Instruct-Q4_K_M")'
```

### 4. Get Most Popular Model from Search

```bash
offgrid search llama --limit 5 --json | \
  jq '.results | sort_by(.downloads) | reverse | .[0] | {name, downloads, likes}'
```

### 5. Export to CSV

```bash
offgrid list --json | jq -r '.models[] | [.name, .size, .quantization] | @csv'
# Output: "Llama-3.2-3B-Instruct-Q4_K_M","1.9 GB","Q4_K_M"
```

### 6. Check System Resources

```bash
offgrid info --json | jq '{cpu: .system.cpu, memory: .system.memory, models: .models.count}'
```

### 7. Session Summary

```bash
offgrid session list --json | jq '.sessions[] | {name, model: .model_id, messages}'
```

## Automation Scripts

### Auto-Download if No Models Installed

```bash
#!/bin/bash
MODEL_COUNT=$(offgrid list --json | jq '.count')

if [ "$MODEL_COUNT" -eq 0 ]; then
    echo "No models installed. Downloading recommended model..."
    
    # Get most downloaded llama model
    BEST_MODEL=$(offgrid search llama --limit 1 --json | jq -r '.results[0].name')
    
    echo "Downloading: $BEST_MODEL"
    offgrid download-hf "$BEST_MODEL"
else
    echo "$MODEL_COUNT model(s) already installed"
fi
```

### Monitor Model Storage

```bash
#!/bin/bash
# Monitor total model storage usage

MODELS=$(offgrid list --json)
COUNT=$(echo "$MODELS" | jq '.count')

echo "Models installed: $COUNT"

# Extract sizes and calculate total (would need size parsing logic)
echo "$MODELS" | jq -r '.models[] | "\(.name): \(.size)"'
```

### CI/CD Integration

```bash
#!/bin/bash
# Pre-deployment check

# Verify models are installed
MODEL_COUNT=$(offgrid list --json | jq '.count')
if [ "$MODEL_COUNT" -eq 0 ]; then
    echo "ERROR: No models installed"
    exit 1
fi

# Check system resources
CPU_CORES=$(offgrid info --json | jq -r '.system.cpu' | grep -oP '\d+')
if [ "$CPU_CORES" -lt 4 ]; then
    echo "WARNING: Less than 4 CPU cores available"
fi

echo "Pre-flight checks passed"
```

## Integration with Other Tools

### Python Integration

```python
import subprocess
import json

def get_installed_models():
    result = subprocess.run(
        ['offgrid', 'list', '--json'],
        capture_output=True,
        text=True
    )
    return json.loads(result.stdout)

models = get_installed_models()
print(f"Found {models['count']} models:")
for model in models['models']:
    print(f"  - {model['name']} ({model['size']})")
```

### JavaScript/Node.js Integration

```javascript
const { execSync } = require('child_process');

function getInstalledModels() {
    const output = execSync('offgrid list --json', { encoding: 'utf-8' });
    return JSON.parse(output);
}

const models = getInstalledModels();
console.log(`Found ${models.count} models`);
models.models.forEach(model => {
    console.log(`  - ${model.name} (${model.size})`);
});
```

### Go Integration

```go
package main

import (
    "encoding/json"
    "os/exec"
)

type ModelList struct {
    Count  int     `json:"count"`
    Models []Model `json:"models"`
}

type Model struct {
    Name         string `json:"name"`
    Size         string `json:"size"`
    Quantization string `json:"quantization"`
}

func getInstalledModels() (*ModelList, error) {
    out, err := exec.Command("offgrid", "list", "--json").Output()
    if err != nil {
        return nil, err
    }
    
    var models ModelList
    if err := json.Unmarshal(out, &models); err != nil {
        return nil, err
    }
    
    return &models, nil
}
```

## Tab Completion

The `--json` flag is included in bash/zsh/fish completions:

```bash
offgrid list --<TAB>      # Shows: --json
offgrid search --<TAB>    # Shows: --json --author --limit
offgrid session --<TAB>   # Shows: --json list show export delete
offgrid info --<TAB>      # Shows: --json
```

## Best Practices

1. **Always use `jq` for parsing**: Don't try to parse JSON with grep/sed/awk
2. **Check exit codes**: JSON mode still exits with error codes on failure
3. **Handle errors**: JSON errors are output to stderr with proper structure
4. **Version stability**: JSON schema is stable across minor versions

## Error Handling

When errors occur in JSON mode, the output follows this structure:

```json
{
  "success": false,
  "message": "Error description",
  "error": "Detailed error message"
}
```

Exit code will be non-zero (typically 1) for errors.

## Future Enhancements

Planned additions:
- `--json` support for `download` and `download-hf` (progress updates as JSON events)
- `--json` support for `run` (streaming chat as JSON-LD)
- Prometheus metrics endpoint integration
- OpenTelemetry trace export

## See Also

- [API Documentation](API.md) - REST API endpoints
- [Shell Completions](CLI_REFERENCE.md#completions) - Tab completion setup
- [Session Management](CLI_REFERENCE.md#sessions) - Persistent conversations
