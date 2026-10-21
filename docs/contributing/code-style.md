# Code Style Guide

This document outlines the coding standards and conventions for OffGrid LLM.
Following these guidelines ensures consistency across the codebase.

## General Principles

1. **Clarity over cleverness** - Write code that's easy to understand
2. **Consistency** - Follow existing patterns in the codebase
3. **Simplicity** - Prefer simple solutions over complex ones
4. **Documentation** - Code should be self-documenting with clear names

## Go Code Style

### Formatting

- Use `gofmt` for all Go code (enforced by CI)
- Use `goimports` to manage imports
- Maximum line length: 100 characters (soft limit)

### Naming Conventions

```go
// Package names: lowercase, single word
package config

// Exported functions: PascalCase, verb-first
func LoadConfig() {}
func ParseRequest() {}

// Unexported functions: camelCase
func validateInput() {}

// Constants: PascalCase or SCREAMING_SNAKE_CASE for special values
const MaxRetries = 3
const DEFAULT_TIMEOUT = 30

// Interfaces: Usually end with -er
type Reader interface {}
type Configurer interface {}

// Acronyms: Consistent case
func GetHTTPClient() {}  // Not GetHttpClient
type JSONParser struct {} // Not JsonParser
```

### Error Handling

```go
// Always handle errors explicitly
result, err := doSomething()
if err != nil {
    return fmt.Errorf("doing something: %w", err)
}

// Use error wrapping for context
if err != nil {
    return fmt.Errorf("loading config from %s: %w", path, err)
}

// Custom errors for specific cases
var ErrNotFound = errors.New("resource not found")
var ErrInvalidInput = errors.New("invalid input")
```

### Struct Organization

```go
type Config struct {
    // Group fields logically
    
    // Required fields first
    Name    string `json:"name"`
    Version string `json:"version"`
    
    // Optional fields
    Debug   bool   `json:"debug,omitempty"`
    Timeout int    `json:"timeout,omitempty"`
    
    // Internal/unexported fields last
    mu      sync.Mutex
    cache   map[string]interface{}
}
```

### Comments

```go
// Package config provides configuration loading and management.
//
// It supports multiple configuration sources including files,
// environment variables, and command-line flags.
package config

// LoadConfig reads configuration from the specified path.
// It returns an error if the file doesn't exist or is malformed.
func LoadConfig(path string) (*Config, error) {
    // Implementation note: we use viper for flexibility
    // but wrap it to hide the dependency
}

// TODO(username): Implement caching - issue #123
// FIXME: This breaks on Windows paths
// NOTE: This is intentionally slow for rate limiting
```

### Testing

```go
// Test file naming: *_test.go
// Test function naming: TestFunctionName_Scenario

func TestLoadConfig_ValidFile(t *testing.T) {
    // Arrange
    path := "testdata/valid.yaml"
    
    // Act
    cfg, err := LoadConfig(path)
    
    // Assert
    require.NoError(t, err)
    assert.Equal(t, "expected", cfg.Name)
}

func TestLoadConfig_MissingFile(t *testing.T) {
    _, err := LoadConfig("nonexistent.yaml")
    assert.ErrorIs(t, err, os.ErrNotExist)
}

// Table-driven tests for multiple cases
func TestValidate(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"valid", "test", false},
        {"empty", "", true},
        {"too long", strings.Repeat("a", 1000), true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := Validate(tt.input)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

## JavaScript Code Style

### General

- Use ES6+ features (modules, arrow functions, destructuring)
- Prefer `const` over `let`, avoid `var`
- Use template literals for string interpolation
- Use async/await over raw promises

### Naming

```javascript
// Variables and functions: camelCase
const userName = 'value';
function getUserName() {}

// Classes and components: PascalCase
class ChatComponent {}

// Constants: SCREAMING_SNAKE_CASE
const MAX_RETRIES = 3;
const API_ENDPOINT = '/v1';

// Private methods/properties: underscore prefix
class Example {
    _privateMethod() {}
}
```

### Functions

```javascript
// Arrow functions for callbacks
array.map(item => item.value);

// Regular functions for methods that use 'this'
const obj = {
    value: 42,
    getValue() {
        return this.value;
    }
};

// Destructuring in parameters
function createUser({ name, email, role = 'user' }) {}

// Default parameters
function fetch(url, options = {}) {}
```

### Documentation (JSDoc)

```javascript
/**
 * Fetches data from the API
 * @param {string} endpoint - API endpoint path
 * @param {Object} options - Request options
 * @param {string} [options.method='GET'] - HTTP method
 * @returns {Promise<Object>} Response data
 * @throws {Error} If the request fails
 */
async function fetchAPI(endpoint, options = {}) {}
```

### Error Handling

```javascript
// Use try-catch for async operations
async function loadData() {
    try {
        const response = await fetch('/api/data');
        return await response.json();
    } catch (error) {
        console.error('Failed to load data:', error);
        throw new Error(`Data loading failed: ${error.message}`);
    }
}

// Check response status
if (!response.ok) {
    throw new Error(`HTTP ${response.status}: ${response.statusText}`);
}
```

## CSS Code Style

### Organization

```css
/* 1. CSS Variables (at root) */
:root {
    --color-primary: #3b82f6;
    --spacing-md: 1rem;
}

/* 2. Base/Reset styles */
*, *::before, *::after {
    box-sizing: border-box;
}

/* 3. Layout */
.container {}
.grid {}

/* 4. Components */
.button {}
.card {}

/* 5. Utilities */
.hidden {}
.text-center {}
```

### Property Order

```css
.element {
    /* Positioning */
    position: relative;
    top: 0;
    z-index: 1;
    
    /* Display & Box Model */
    display: flex;
    width: 100%;
    padding: 1rem;
    margin: 0;
    
    /* Typography */
    font-size: 1rem;
    color: #333;
    
    /* Visual */
    background: white;
    border: 1px solid #ddd;
    border-radius: 4px;
    
    /* Animation */
    transition: all 0.2s ease;
}
```

### Naming (BEM-inspired)

```css
/* Block */
.card {}

/* Element */
.card__header {}
.card__body {}
.card__footer {}

/* Modifier */
.card--featured {}
.card--compact {}

/* State */
.card.is-active {}
.card.is-loading {}
```

## Git Commit Messages

### Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `style`: Code style (formatting, semicolons, etc)
- `refactor`: Code change that neither fixes nor adds
- `perf`: Performance improvement
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

### Examples

```
feat(chat): add streaming response support

Implement Server-Sent Events for real-time token streaming.
This improves perceived latency for long responses.

Closes #123

---

fix(models): handle missing model gracefully

Return helpful error message instead of crashing when
the requested model file doesn't exist.

---

docs(api): update authentication examples

Add Python and JavaScript examples for API authentication.
```

## File Organization

```
internal/
├── feature/
│   ├── feature.go       # Main implementation
│   ├── feature_test.go  # Tests
│   ├── options.go       # Configuration options
│   ├── errors.go        # Custom errors
│   └── doc.go           # Package documentation
```

## Code Review Checklist

Before submitting a PR, verify:

- [ ] Code follows style guidelines
- [ ] All tests pass
- [ ] New code has tests
- [ ] Documentation is updated
- [ ] No commented-out code
- [ ] No debug statements (fmt.Println, console.log)
- [ ] Error messages are helpful
- [ ] No hardcoded secrets or paths

## Tools

| Tool | Purpose | Command |
|------|---------|---------|
| `gofmt` | Format Go code | `gofmt -w .` |
| `goimports` | Manage imports | `goimports -w .` |
| `golangci-lint` | Lint Go code | `golangci-lint run` |
| `go vet` | Static analysis | `go vet ./...` |
| `prettier` | Format JS/CSS | `prettier --write .` |

## Editor Configuration

### VS Code Settings

```json
{
    "editor.formatOnSave": true,
    "editor.tabSize": 4,
    "go.formatTool": "goimports",
    "go.lintTool": "golangci-lint",
    "[javascript]": {
        "editor.tabSize": 2
    }
}
```

### EditorConfig

```ini
# .editorconfig
root = true

[*]
indent_style = space
indent_size = 4
end_of_line = lf
charset = utf-8
trim_trailing_whitespace = true
insert_final_newline = true

[*.{js,json,yaml,yml}]
indent_size = 2

[Makefile]
indent_style = tab
```
