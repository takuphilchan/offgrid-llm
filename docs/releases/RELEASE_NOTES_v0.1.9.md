# Release Notes - v0.1.9

**Release Date:** November 22, 2025

## UI Polish & "True Neutral" Theme

This release focuses on refining the user interface with a professional "True Neutral" dark theme and improved interaction patterns.

### Key Features

#### 🎨 True Neutral Dark Theme
- **No Blue Tint**: The dark mode has been updated to use pure grays and blacks, removing the previous navy blue undertone for a cleaner, more professional look.
- **Consistent Styling**: Sidebar, headers, and card backgrounds now share a unified neutral palette.
- **Improved Contrast**: Text and accent colors have been adjusted for better readability against the new neutral background.

#### 🪟 Modern Modal System
- **Custom Dialogs**: Replaced all native browser `alert()` and `prompt()` dialogs with custom, theme-aware modals.
- **Unified Experience**: Error messages, confirmations, and input prompts now match the application's design language.
- **Enhanced Inputs**: New input dialogs for session renaming, custom system prompts, and export options.

#### 🖥️ Desktop & Web Parity
- **Synchronized UI**: Both the Web UI and Desktop application now share the exact same visual improvements and modal behaviors.
- **Export Options**: Enhanced chat export dialog with clear descriptions for Markdown, Plain Text, and JSON formats.

### Bug Fixes

- **Terminal Execution**: Fixed an issue where terminal commands would fail with HTTP 500 errors.
- **JavaScript Errors**: Resolved syntax errors that were causing `switchTab` and other functions to fail.
- **Windows Compatibility**: Minor updates to USB utility functions for Windows builds.

### Technical Details

- **Version Bump**: Updated core version to v0.1.9 across all components.
- **Build System**: Updated Makefile default version.
