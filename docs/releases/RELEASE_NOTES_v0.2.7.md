# Release Notes v0.2.7

**Release Date:** December 17, 2025

## Overview

Version 0.2.7 focuses on **documentation improvements**, **codebase cleanup**, and **modular UI architecture**. This release makes the project more accessible to contributors and improves maintainability.

---

## 🆕 New Features

### Modular Web UI Architecture
- **Refactored monolithic UI** - Split 10,650-line `index.html` into 16 JavaScript modules
- **Organized CSS** - Extracted all styles into `web/ui/css/styles.css` (1,607 lines)
- **Better separation of concerns** - Each feature (chat, models, agents, etc.) has its own module
- **Desktop app sync** - UI now syncs to desktop via `scripts/sync-ui.sh`

### Improved Documentation Structure
- **Reorganized docs/** - New folder structure: `setup/`, `guides/`, `reference/`, `advanced/`
- **Cleaner naming** - Lowercase with hyphens (e.g., `getting-started.md` instead of `GETTING_STARTED.md`)
- **New architecture docs** - Comprehensive system design documentation with diagrams
- **Better contributor guide** - ASCII architecture diagrams, code style guide, testing patterns

---

## 📚 Documentation Changes

### New Documentation Structure
```
docs/
├── setup/           # Installation & configuration
│   ├── quickstart.md
│   ├── installation.md
│   ├── docker.md
│   └── autostart.md
├── guides/          # Feature tutorials
│   ├── getting-started.md
│   ├── models.md
│   ├── agents.md
│   ├── embeddings.md
│   └── ...
├── reference/       # API & CLI specs
│   ├── api.md
│   ├── cli.md
│   └── ...
└── advanced/        # Architecture & optimization
    ├── architecture.md
    ├── performance.md
    └── ...
```

### New/Updated Documents
- `docs/advanced/architecture.md` - System design with component diagrams
- `docs/guides/getting-started.md` - Complete beginner's guide
- `dev/CONTRIBUTING.md` - Enhanced with architecture diagrams
- `README.md` - Improved with visual feature grid

---

## 🧹 Codebase Cleanup

### Removed Files
- Backup files (`.bak`, `.orig`)
- Build artifacts (`bin/`, `dist/`, `*.egg-info`)
- Embedded binaries from desktop app

### Fixed Issues
- Placeholder usernames in documentation
- Broken internal doc links after reorganization
- Python library version mismatch (`0.1.5` → `0.1.6`)

---

## 🔧 Technical Changes

### Web UI Modules
| Module | Purpose |
|--------|---------|
| `app.js` | Main application initialization |
| `chat.js` | Chat session management |
| `chat-ui.js` | Chat UI components |
| `models.js` | Model management logic |
| `models-ui.js` | Models UI components |
| `agent.js` | AI agent functionality |
| `audio.js` | Voice input/output |
| `terminal.js` | Terminal emulator |
| `settings.js` | User settings |
| `utils.js` | Shared utilities |

### Python Library
- Version synced to `0.1.6`
- All server endpoints covered
- Documentation links fixed

---

## 📦 Installation

### One-Line Install
```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh | bash
```

### Docker
```bash
docker pull ghcr.io/takuphilchan/offgrid-llm:v0.2.7
docker run -p 11611:11611 ghcr.io/takuphilchan/offgrid-llm:v0.2.7
```

### From Source
```bash
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm
go build -o offgrid ./cmd/offgrid
./offgrid serve
```

---

## 🔄 Upgrade Notes

- No breaking API changes
- Documentation paths have changed - update any bookmarks
- Web UI structure changed but functionality remains the same

---

## 📊 Stats

- **Files changed:** 50+
- **Documentation:** 57 markdown files
- **Web UI:** 16 JS modules, 2 CSS files
- **Total codebase:** ~3.8MB (excluding binaries)

---

## Contributors

Thanks to all contributors who helped with this release!

---

**Full Changelog:** [v0.2.6...v0.2.7](https://github.com/takuphilchan/offgrid-llm/compare/v0.2.6...v0.2.7)
