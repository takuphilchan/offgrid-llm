# OffGrid LLM Desktop App

Native desktop application for OffGrid LLM using Tauri.

## Features
- Native system tray integration
- Auto-start with system (optional)
- Native notifications
- Lightweight (Rust backend + webview)
- Cross-platform (Linux, macOS, Windows)

## Building

### Prerequisites
```bash
# Install Rust
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh

# Install Tauri CLI
cargo install tauri-cli

# Install system dependencies (Linux)
sudo apt install libwebkit2gtk-4.0-dev build-essential curl wget file libssl-dev libgtk-3-dev libayatana-appindicator3-dev librsvg2-dev
```

### Build Desktop App
```bash
cd desktop
npm install  # or pnpm install
npm run tauri build
```

## Development
```bash
npm run tauri dev
```

The app will connect to the OffGrid server running on localhost:11611.
