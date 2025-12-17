# OffGrid LLM Web UI

> A modular, contributor-friendly web interface for OffGrid LLM.

## Overview

The UI provides a complete interface for interacting with OffGrid LLM:

| Feature | Description |
|---------|-------------|
| 💬 **Chat** | Conversational AI with streaming responses |
| 🔧 **Models** | Download, load, and configure models |
| 🎤 **Voice** | Speech-to-text and text-to-speech |
| 🤖 **Agent** | Autonomous task execution with tools |
| 📚 **Knowledge** | RAG with document ingestion |
| 📊 **Benchmarks** | Performance testing and comparison |
| 💻 **Terminal** | Command-line interface in browser |
| 👥 **Users** | User management and authentication |

---

## Quick Start

```bash
# Start from project root
cd web/ui

# Option 1: Python server
python3 -m http.server 8081

# Option 2: Node server  
npx serve -p 8081

# Open http://localhost:8081
```

---

## File Structure

```
web/ui/
├── index.html              # Main entry point (loads external files)
├── README.md               # This file
├── CONTRIBUTING.md         # UI contribution guide
│
├── css/
│   └── styles.css          # All CSS (~1,600 lines)
│       ├── CSS Variables   # Theme colors, fonts
│       ├── Base Styles     # Typography, layout
│       ├── Components      # Buttons, cards, modals
│       └── Utilities       # Helpers, animations
│
└── js/                     # JavaScript modules (~6,800 lines total)
    │
    │── [Core]
    ├── utils.js            # State variables, helpers, init
    ├── modals.js           # Alert, confirm, prompt dialogs
    ├── auth.js             # Login, logout, session check
    │
    │── [Chat & Models]
    ├── chat.js             # Chat logic, sessions, history
    ├── chat-ui.js          # Message rendering, streaming
    ├── models.js           # Model loading, configuration
    ├── models-ui.js        # Model browser, downloads
    │
    │── [Features]
    ├── terminal.js         # Terminal emulator
    ├── rag.js              # Knowledge base, documents
    ├── benchmark.js        # Performance benchmarks
    ├── agent.js            # AI Agent with MCP
    ├── audio.js            # Voice input/output
    │
    │── [Management]
    ├── users.js            # User management
    ├── metrics.js          # System monitoring
    ├── lora.js             # LoRA adapter management
    └── file-browser.js     # File selection modal
```

---

## Contributing

See **[CONTRIBUTING.md](CONTRIBUTING.md)** for detailed contribution guidelines.

### Quick Reference: Which File to Edit

| I want to change... | Edit this file |
|---------------------|----------------|
| Colors, themes, dark mode | `css/styles.css` (`:root` variables) |
| Chat messages, streaming | `js/chat-ui.js` |
| Session management | `js/chat.js` |
| Model loading | `js/models.js` |
| Model browser UI | `js/models-ui.js` |
| Voice/audio features | `js/audio.js` |
| Terminal commands | `js/terminal.js` |
| AI Agent behavior | `js/agent.js` |
| Knowledge base/RAG | `js/rag.js` |
| Benchmarks | `js/benchmark.js` |
| User management | `js/users.js` |
| Popup dialogs | `js/modals.js` |
| Utility functions | `js/utils.js` |

---

## Architecture

### Technology Stack

| Technology | Purpose |
|------------|---------|
| Tailwind CSS | Utility classes (CDN) |
| Vanilla JS | No framework, no build step |
| marked.js | Markdown rendering |
| highlight.js | Code syntax highlighting |
| JetBrains Mono | Terminal font |

### Design Principles

1. **No Build Required** - Works directly in browser
2. **Global Scope** - Functions accessible everywhere (no ES6 modules)
3. **Progressive Enhancement** - Works without JS for basic viewing
4. **Offline-First** - Minimal external dependencies
5. **Theme Support** - CSS variables for easy theming

### State Management

Global state in `js/utils.js`:

```javascript
// Current state
let currentModel = '';
let sessions = {};
let currentSessionId = null;
let isStreaming = false;

// Configuration
let config = {
    temperature: 0.7,
    maxTokens: 2048
};
```

---

## Theming

Colors are defined via CSS variables in `css/styles.css`:

```css
:root {
    --bg-primary: #ffffff;
    --bg-secondary: #f9fafb;
    --text-primary: #111827;
    --accent: #3b82f6;
}

.dark {
    --bg-primary: #111827;
    --bg-secondary: #1f2937;
    --text-primary: #f9fafb;
}
```

Toggle dark mode by clicking the theme toggle in the sidebar.

---

## API Integration

The UI communicates with the OffGrid LLM server:

| Endpoint | Purpose |
|----------|---------|
| `GET /api/v1/models` | List available models |
| `POST /api/v1/chat/completions` | Send chat messages |
| `POST /api/v1/embeddings` | Generate embeddings |
| `GET /api/v1/health` | Server health check |
| `WS /api/v1/ws` | WebSocket for streaming |

See **[API Documentation](../../docs/API.md)** for full reference.

---

## Testing

### Manual Testing

1. Start local server: `python3 -m http.server 8081`
2. Open browser DevTools (F12)
3. Check Console for errors
4. Test each feature tab

### Checklist

- [ ] Chat sends and receives messages
- [ ] Models load and switch correctly
- [ ] Dark/light theme toggle works
- [ ] Voice recording (if microphone available)
- [ ] Session save/load/export
- [ ] Mobile responsive layout

---

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Styles not loading | Check for 404 errors, verify `css/styles.css` exists |
| JavaScript errors | Scripts must load in order (`utils.js` first) |
| API calls failing | Ensure OffGrid server is running, check CORS |

---

## Related Documentation

- [Main Documentation](../../docs/README.md)
- [API Reference](../../docs/API.md)
- [Contributing Guide](../../dev/CONTRIBUTING.md)
- [Features Guide](../../docs/guides/FEATURES_GUIDE.md)
