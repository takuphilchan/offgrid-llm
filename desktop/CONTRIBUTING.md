# Contributing to OffGrid LLM Web UI

Thank you for your interest in improving the OffGrid LLM web interface!

---

## Table of Contents

- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Code Organization](#code-organization)
- [Making Changes](#making-changes)
- [Code Style Guide](#code-style-guide)
- [Testing Your Changes](#testing-your-changes)
- [Submitting Changes](#submitting-changes)

---

## Getting Started

### Prerequisites

- Modern web browser (Chrome, Firefox, Edge, Safari)
- Text editor or IDE (VS Code recommended)
- Basic knowledge of HTML, CSS, JavaScript
- Local web server (Python, Node, or any static server)

### Quick Setup

```bash
# Clone the repository
git clone https://github.com/YOUR_USERNAME/offgrid-llm.git
cd offgrid-llm/web/ui

# Start a local server
python3 -m http.server 8081

# Open in browser
open http://localhost:8081
```

---

## Development Setup

### Recommended VS Code Extensions

```json
{
  "recommendations": [
    "bradlc.vscode-tailwindcss",
    "esbenp.prettier-vscode",
    "dbaeumer.vscode-eslint"
  ]
}
```

### Browser DevTools

- **Console**: Check for JavaScript errors
- **Network**: Monitor API calls
- **Elements**: Inspect and debug CSS
- **Application**: View localStorage data

---

## Code Organization

### File Responsibilities

```
js/
├── utils.js        ← START HERE: Global state and helpers
├── modals.js       ← Popup dialogs (showAlert, showConfirm)
├── auth.js         ← Authentication state
│
├── chat.js         ← Session management, history
├── chat-ui.js      ← Message rendering, streaming display
│
├── models.js       ← Model loading, configuration
├── models-ui.js    ← Model browser, HuggingFace integration
│
├── audio.js        ← Voice recording, STT, TTS
├── terminal.js     ← Terminal emulator
├── rag.js          ← Knowledge base operations
├── benchmark.js    ← Performance testing
├── agent.js        ← AI Agent functionality
│
├── users.js        ← User management (multi-user mode)
├── metrics.js      ← System metrics display
├── lora.js         ← LoRA adapter management
└── file-browser.js ← File selection modal
```

### Load Order (Important!)

Scripts are loaded in order in `index.html`. Dependencies must be loaded first:

```html
<!-- Core (must be first) -->
<script src="js/utils.js"></script>
<script src="js/modals.js"></script>
<script src="js/auth.js"></script>

<!-- Features (depend on core) -->
<script src="js/models.js"></script>
<script src="js/chat.js"></script>
<!-- ... etc ... -->
```

---

## Making Changes

### Adding a New Feature

1. **Determine the right file** or create a new one in `js/`
2. **Add functions** in global scope
3. **Update `index.html`** if adding new script
4. **Add CSS** to `css/styles.css` if needed
5. **Test thoroughly** in multiple browsers

### Example: Adding a New Button

**1. Add HTML in `index.html`:**
```html
<button onclick="myNewFeature()" class="btn btn-primary">
    My Feature
</button>
```

**2. Add JavaScript in appropriate file:**
```javascript
// js/my-feature.js or add to existing file
function myNewFeature() {
    // Implementation
    console.log('Feature activated');
    showToast('Feature activated!', 'success');
}
```

**3. Add CSS if needed:**
```css
/* css/styles.css */
.my-feature-class {
    /* styles */
}
```

### Modifying Existing Features

1. **Find the right file** using the table in README.md
2. **Understand the function** before modifying
3. **Keep backward compatibility** if possible
4. **Test related features** to avoid breaking changes

---

## Code Style Guide

### JavaScript

```javascript
// ✅ Good: Descriptive function names
function loadChatModels() { }
function handleModelChange(modelId) { }

// ❌ Bad: Unclear names
function load() { }
function handle(x) { }

// ✅ Good: Use const/let, not var
const API_BASE = '/api/v1';
let currentModel = '';

// ❌ Bad: Using var
var currentModel = '';

// ✅ Good: Early returns for readability
function processMessage(msg) {
    if (!msg) return null;
    if (msg.type === 'error') return handleError(msg);
    return formatMessage(msg);
}

// ✅ Good: Async/await for promises
async function fetchModels() {
    try {
        const response = await fetch('/api/v1/models');
        const data = await response.json();
        return data.models;
    } catch (error) {
        showToast('Failed to load models', 'error');
        return [];
    }
}
```

### CSS

```css
/* ✅ Good: Use CSS variables for colors */
.my-component {
    background: var(--bg-secondary);
    color: var(--text-primary);
    border-color: var(--border);
}

/* ❌ Bad: Hardcoded colors (breaks themes) */
.my-component {
    background: #f9fafb;
    color: #111827;
}

/* ✅ Good: Use Tailwind utilities when possible */
/* In HTML: class="flex items-center gap-2 p-4" */

/* ✅ Good: Custom CSS only when needed */
.terminal-output {
    font-family: 'JetBrains Mono', monospace;
    white-space: pre-wrap;
}
```

### HTML

```html
<!-- ✅ Good: Semantic elements -->
<nav class="sidebar">
    <button onclick="switchTab('chat')" aria-label="Chat">
        <span>Chat</span>
    </button>
</nav>

<!-- ✅ Good: Accessibility attributes -->
<button 
    onclick="sendMessage()" 
    aria-label="Send message"
    title="Send message (Enter)">
    Send
</button>
```

---

## Testing Your Changes

### Manual Testing Checklist

Before submitting, verify:

- [ ] **No console errors** in DevTools
- [ ] **Feature works** as expected
- [ ] **Dark mode** displays correctly
- [ ] **Mobile layout** is responsive
- [ ] **Related features** still work
- [ ] **Page loads** without issues

### Browser Testing

Test in multiple browsers:
- Chrome/Edge (Chromium)
- Firefox
- Safari (if available)

### API Testing

If your changes involve API calls:

```javascript
// Test with server running
./offgrid serve --verbose

// Check Network tab for:
// - Correct endpoints
// - Proper request/response
// - Error handling
```

---

## Submitting Changes

### Commit Messages

```bash
# Format: type(scope): description

# Examples:
git commit -m "feat(chat): add message copy button"
git commit -m "fix(audio): resolve microphone permission issue"
git commit -m "style(theme): improve dark mode contrast"
git commit -m "docs(ui): update README with new structure"
```

### Pull Request Template

```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] UI improvement
- [ ] Documentation

## Testing Done
- [ ] Tested in Chrome
- [ ] Tested in Firefox
- [ ] Tested mobile layout
- [ ] No console errors

## Screenshots
(if applicable)
```

---

## Common Patterns

### Showing Notifications

```javascript
// Toast notifications
showToast('Message sent!', 'success');
showToast('Error occurred', 'error');
showToast('Processing...', 'info');

// Modal dialogs
showAlert('Important message');
const confirmed = await showConfirm('Are you sure?');
const value = await showPrompt('Enter name:', 'Default');
```

### API Calls

```javascript
// Standard pattern
async function apiCall(endpoint, method = 'GET', body = null) {
    const options = {
        method,
        headers: { 'Content-Type': 'application/json' }
    };
    if (body) options.body = JSON.stringify(body);
    
    const response = await fetch(`/api/v1${endpoint}`, options);
    if (!response.ok) throw new Error(`API error: ${response.status}`);
    return response.json();
}
```

### State Updates

```javascript
// Update state and UI together
function updateCurrentModel(modelId) {
    currentModel = modelId;
    document.getElementById('current-model').textContent = modelId;
    localStorage.setItem('currentModel', modelId);
}
```

---

## Need Help?

- Check existing code for patterns
- Read the [README.md](README.md) for architecture overview
- Look at [API docs](../../docs/API.md) for endpoints
- Open an issue for questions

Thank you for contributing! 🎉
