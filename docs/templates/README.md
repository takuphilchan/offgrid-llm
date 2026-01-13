# Documentation Templates

This directory contains templates for writing consistent documentation across the OffGrid LLM project.

## Available Templates

| Template | Purpose | When to Use |
|----------|---------|-------------|
| [feature-template.md](feature-template.md) | Document new features | Adding new capabilities |
| [api-template.md](api-template.md) | Document API endpoints | New REST endpoints |
| [guide-template.md](guide-template.md) | Write how-to guides | Tutorials and walkthroughs |

---

## Quick Start

1. Copy the appropriate template
2. Rename to your document name
3. Fill in the sections
4. Remove any unused sections

```bash
# Example: Create a new feature doc
cp templates/feature-template.md guides/my-feature.md
```

---

## Documentation Standards

### File Naming

| Location | Convention | Example |
|----------|------------|---------|
| All docs | lowercase-kebab | `installation.md`, `api.md` |
| docs/guides/ | SCREAMING_SNAKE_GUIDE | `AGENT_GUIDE.md` |
| docs/advanced/ | SCREAMING_SNAKE | `ARCHITECTURE.md` |
| docs/guides/ | lowercase-kebab | `agents.md`, `getting-started.md` |
| docs/advanced/ | lowercase-kebab | `architecture.md`, `performance.md` |

### Markdown Standards

```markdown
# Title (H1) - One per document

Brief introduction (1-2 sentences).

---

## Section (H2) - Major sections

### Subsection (H3) - Details within sections

#### Minor heading (H4) - Rarely needed
```

### Required Sections

Every document should have:

1. **Title** - Clear, descriptive H1
2. **Brief Description** - 1-2 sentence overview
3. **Table of Contents** - For docs > 100 lines
4. **Prerequisites** - What users need first
5. **Main Content** - Organized with H2/H3
6. **Examples** - Practical code examples
7. **Troubleshooting** - Common issues (if applicable)
8. **See Also** - Related documentation links

### Code Examples

Always specify the language:

````markdown
```bash
# Shell commands
offgrid serve
```

```go
// Go code with comments
func Example() {}
```

```python
# Python code
client = offgrid.Client()
```

```json
{
  "json": "with proper formatting"
}
```
````

### Tables

```markdown
| Column 1 | Column 2 | Column 3 |
|----------|----------|----------|
| Data | Data | Data |
```

### Callouts

```markdown
> 💡 **Tip:** Helpful suggestion

> ⚠️ **Warning:** Important caution

> ❌ **Danger:** Critical warning

> ℹ️ **Note:** Additional information
```

---

## Writing Tips

### Do

✅ Write for beginners - assume minimal context  
✅ Be concise - respect reader's time  
✅ Use examples - show, don't just tell  
✅ Stay current - update when code changes  
✅ Test instructions - verify steps work  
✅ Use relative links - `../API.md` not absolute URLs  

### Don't

❌ Assume prior knowledge  
❌ Use jargon without explanation  
❌ Leave outdated information  
❌ Skip error handling in examples  
❌ Use absolute file paths  

---

## Cross-References

Use relative paths for internal links:

```markdown
<!-- From docs/guides/FEATURE.md -->
[API Reference](../API.md)
[Installation](../INSTALLATION.md)
[Contributing](../../dev/CONTRIBUTING.md)
```

---

## Screenshots

- Save in `docs/images/`
- Use descriptive names: `feature-action-result.png`
- Include alt text for accessibility
- Keep file sizes reasonable (<500KB)
- Use dark mode for consistency

```markdown
![Feature screenshot](images/feature-action.png)
```

---

## Review Checklist

Before submitting documentation:

- [ ] Follows naming convention
- [ ] Has all required sections
- [ ] Code examples are complete and tested
- [ ] Links work correctly
- [ ] No spelling/grammar errors
- [ ] Renders correctly in preview
