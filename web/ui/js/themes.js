// themes.js - Theme support for OffGrid LLM UI
// Works with existing CSS variable system in styles.css

// Simple theme toggle - respects the existing CSS theme definitions
let currentTheme = 'dark';

// Initialize theme system
function initThemes() {
    const savedTheme = localStorage.getItem('offgrid-theme') || 'dark';
    applyTheme(savedTheme);
}

// Apply theme using data-theme attribute (matches styles.css)
function applyTheme(themeName) {
    if (themeName !== 'light' && themeName !== 'dark') {
        themeName = 'dark';
    }
    
    document.documentElement.setAttribute('data-theme', themeName);
    currentTheme = themeName;
    localStorage.setItem('offgrid-theme', themeName);
    
    // Dispatch event for components that need to know about theme changes
    window.dispatchEvent(new CustomEvent('themeChange', { detail: { theme: themeName } }));
}

// Get current theme
function getCurrentTheme() {
    return currentTheme;
}

// Toggle between light and dark
function toggleTheme() {
    const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
    applyTheme(newTheme);
    return newTheme;
}

// Show theme picker (simple toggle for now)
function showThemePicker() {
    toggleTheme();
}

// Initialize on load
document.addEventListener('DOMContentLoaded', () => {
    initThemes();
});

// Export for module use
if (typeof module !== 'undefined' && module.exports) {
    module.exports = {
        applyTheme,
        getCurrentTheme,
        toggleTheme,
        showThemePicker
    };
}
