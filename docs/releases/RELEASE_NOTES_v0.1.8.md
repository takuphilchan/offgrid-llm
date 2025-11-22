# Release Notes - v0.1.9

**Release Date:** November 21, 2025

## Vision Language Model (VLM) Support

This release brings comprehensive vision model support with automatic projector management, making multimodal inference as simple as downloading a text model.

### Key Features

#### 🎯 Automatic Vision Projector Downloads
- **Smart Fallback Catalog**: When downloading VLMs, OffGrid now automatically fetches matching `.mmproj` adapter files from a curated catalog (powered by `koboldcpp/mmproj`)
- **Zero Manual Steps**: No more hunting for compatible projectors—everything downloads in one command
- **Broad Model Coverage**: Pre-wired support for:
  - Qwen2.5-VL (3B/7B) and Qwen2-VL (2B/7B)
  - LLaMA 3 Vision (8B) and LLaVA (7B/13B)
  - Gemma 3 Vision (4B/12B/27B)
  - Pixtral 12B, Mistral Small 24B Vision
  - Yi 34B, MiniCPM, Obsidian 3B

#### 🔧 Enhanced Download Engine
- **Tree API Fallback**: `offgrid download-hf` now uses Hugging Face's tree API when model cards don't list file siblings, fixing downloads for repositories like `mradermacher/*-GGUF`
- **Accurate File Sizes**: All GGUF listings now show real file sizes instead of estimates
- **Better Error Messages**: Vision-specific errors (`missing_mmproj`) now surface through the API with actionable guidance

#### 🖼️ UI/CLI Parity
- **Multi-Image Upload**: Desktop and web UIs support dragging/selecting multiple images per message
- **Capability Detection**: Automatic check for vision support—image controls only appear for compatible models
- **Structured Error Display**: When projectors are missing, the UI explains what's needed and suggests next steps

### Technical Improvements

#### Backend
- New `ProjectorSource` abstraction tracks whether adapters come from the model repo or fallback catalog
- `ResolveProjectorSource` helper unifies companion detection + fallback lookup
- `downloadVisionProjector` refactored to accept source metadata and display provenance
- Added `HuggingFaceClient.ListGGUFFiles` to wrap tree queries with proper parsing

#### Frontend
- `modelSupportsVision` checks capabilities before rendering image upload
- `handleImageUpload` manages multiple files, base64 encoding, and removal
- Error responses from `/v1/chat/completions` now display inline with retry hints

#### Testing
- Unit tests for `ResolveProjectorSource` guard both companion-first and fallback logic
- Fallback matcher heuristics normalize repo basenames and filename stems for robust matching

### Developer Experience
- **Extensible Catalog**: Add new VLM→projector mappings in `internal/models/projector_fallbacks.go` without recompiling docs
- **CLI Documentation**: Updated `docs/CLI_REFERENCE.md` with vision fallback coverage and extension instructions
- **Logging & Provenance**: Download output clearly labels when projectors come from fallback sources

### Bug Fixes
- Fixed missing GGUF enumeration for repos without `siblings` metadata
- Corrected image payload structure in chat completion requests (switched from object to array)
- Ensured projector files download to the same directory as their paired GGUF weights

### Breaking Changes
None—existing text-only workflows remain unchanged.

---

## Installation & Upgrade

```bash
# Download latest binary
curl -LO https://github.com/takuphilchan/offgrid-llm/releases/download/v0.1.9/offgrid-linux-amd64
chmod +x offgrid-linux-amd64
sudo mv offgrid-linux-amd64 /usr/local/bin/offgrid

# Or build from source
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm
git checkout v0.1.9
go build -o offgrid cmd/offgrid/main.go
```

## Quick Start with Vision Models

```bash
# Download a VLM—projector fetches automatically
offgrid download-hf mradermacher/VLM-R1-Qwen2.5VL-3B-OVD-0321-i1-GGUF \
  --file VLM-R1-Qwen2.5VL-3B-OVD-0321.i1-Q4_K_M.gguf

# Run vision inference
offgrid run VLM-R1-Qwen2.5VL-3B-OVD-0321.i1-Q4_K_M --image photo.jpg

# Or use the desktop/web UI for drag-and-drop image chat
```

## What's Next (v0.1.9 Roadmap)

- Multi-turn vision conversations with persistent image context
- Dynamic projector resolution (different adapters for different quant levels)
- Voice-to-text integration for Omni models
- Server-side image preprocessing (resize, normalize) to reduce client upload size

## Contributors

Thank you to everyone who tested, reported issues, and contributed to this release!

## Feedback & Support

- **Issues**: https://github.com/takuphilchan/offgrid-llm/issues
- **Discussions**: https://github.com/takuphilchan/offgrid-llm/discussions
- **Documentation**: https://github.com/takuphilchan/offgrid-llm/tree/main/docs
