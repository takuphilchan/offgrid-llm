# OffGrid LLM v0.2.5 Release Notes

**Release Date:** December 10, 2025

## UI Polish & Streaming Stability

This release focuses on refining the user interface and improving the stability and smoothness of the chat experience. We've synchronized the design between the Web and Desktop versions and eliminated visual jitter during text generation.

---

## Highlights

- **Unified UI Design**: Consistent "fluffy" aesthetic across Web and Desktop.
-  **Smoother Streaming**: New rendering engine eliminates jitter during generation.
- **Bug Fixes**: Resolved markdown parsing errors and theme inconsistencies.

---

### Improvements

#### User Interface
- **Consistent Styling**: The Web UI and Desktop app now share the exact same "fluffy" design language.
- **Theme Fixes**: Fixed the terminal header color to correctly match the cyan accent theme (`var(--accent-primary)`).
- **Visual Polish**: Improved spacing and color consistency in the chat interface.

#### Performance & Stability
- **Jitter-Free Streaming**: Implemented an 80ms throttle (12fps) for chat updates. This ensures smooth text rendering without the "jumping" effect seen in previous versions, especially when generating bold text or code blocks.
- **Robust Markdown Parsing**: Fixed a `TypeError` in the markdown parser that could occur with empty or malformed inputs.
- **Trailing Space Handling**: Improved how the streaming engine handles trailing spaces, preventing layout shifts during generation.

### Technical Details

- **Render Throttling**: The chat window now updates at a maximum of 12 frames per second during streaming, significantly reducing CPU usage and visual noise.
- **Type Safety**: Added stricter type checks for the `marked.js` integration to prevent runtime errors.
- **Version Synchronization**: Ensured version strings are consistent across all application components (CLI, Server, Desktop, Web).
