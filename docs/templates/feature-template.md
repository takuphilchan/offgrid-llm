# Feature Name

> Brief one-line description of what this feature does.

## Overview

Provide a 2-3 paragraph overview of the feature:
- What problem does it solve?
- Who is it for?
- What are the key benefits?

## Prerequisites

List any requirements before using this feature:

- [ ] OffGrid LLM version X.Y.Z or higher
- [ ] Specific hardware requirements (if any)
- [ ] Configuration requirements
- [ ] Dependencies

## Quick Start

```bash
# Show the simplest way to use this feature
offgrid <command> --option value
```

## Configuration

### Basic Configuration

```yaml
# config.yaml
feature_name:
  enabled: true
  option1: value1
  option2: value2
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | `false` | Enable/disable feature |
| `option1` | string | `""` | Description of option1 |
| `option2` | int | `100` | Description of option2 |

## Usage

### Basic Usage

Explain the most common use case with examples:

```go
// Example code showing basic usage
import "github.com/yourorg/offgrid-llm/internal/feature"

func main() {
    f := feature.New(feature.Config{
        Option1: "value",
    })
    f.DoSomething()
}
```

### Advanced Usage

Show more complex scenarios:

```go
// Advanced example with all options
```

## API Reference

### Functions

#### `FunctionName(params) returns`

Description of what the function does.

**Parameters:**
- `param1` (type): Description
- `param2` (type): Description

**Returns:**
- `result` (type): Description

**Example:**
```go
result := FunctionName(param1, param2)
```

## Best Practices

1. **Tip 1** - Explanation of best practice
2. **Tip 2** - Explanation of best practice
3. **Tip 3** - Explanation of best practice

## Performance Considerations

Discuss any performance implications:
- Memory usage
- CPU impact
- Recommended settings for different scenarios

## Troubleshooting

### Common Issues

#### Issue: Description of problem

**Symptoms:**
- What the user sees

**Cause:**
- Why it happens

**Solution:**
```bash
# How to fix it
```

#### Issue: Another common problem

**Solution:** Brief explanation or link to detailed fix.

## Examples

### Example 1: Use Case Name

Full working example for a specific use case:

```go
// Complete, runnable example
package main

import (
    "github.com/yourorg/offgrid-llm/internal/feature"
)

func main() {
    // ... complete example
}
```

### Example 2: Another Use Case

Another practical example.

## FAQ

**Q: Common question?**

A: Clear, concise answer.

**Q: Another question?**

A: Answer with code example if helpful.

## See Also

- [Related Feature 1](./RELATED_FEATURE_1.md)
- [Related Feature 2](./RELATED_FEATURE_2.md)
- [API Reference](./API.md)

---

*Last updated: YYYY-MM-DD*
