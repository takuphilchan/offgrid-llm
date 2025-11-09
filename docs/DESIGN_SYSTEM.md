# OffGrid LLM Design System

**A unique, technical aesthetic for edge inference orchestration**

## Philosophy

OffGrid LLM embraces a **technical-first design** that respects the intelligence of its users while providing a refined, modern experience. The design language balances:

- **Technical clarity** over decoration
- **Information density** over whitespace  
- **Consistent patterns** over novelty
- **Functional beauty** over pure aesthetics

## Visual Identity

### Color Palette

#### Brand Colors (CLI & Web)
```
Cyan Primary    #00d4ff  (--brand-primary)    Main accent, highlights, interactive elements
Purple Secondary #af87ff  (--brand-secondary)  Sections, groupings, secondary actions  
Yellow Accent    #ffff00  (--brand-accent)     Warnings, code, attention grabbers
Green Success    #5fd787  (--brand-success)    Confirmations, healthy states
Red Error        #ff005f  (--brand-error)      Errors, critical warnings
Orange Warning   #ffaf00  (--brand-warning)    Cautions, processing states
```

#### Neutral Grays
```
gray-950  #0a0a0a  Primary background
gray-900  #171717  Elevated surfaces
gray-800  #262626  Hover states
gray-700  #404040  Borders (default)
gray-600  #525252  Borders (strong)
gray-500  #737373  Text (muted)
gray-400  #a3a3a3  Text (tertiary)
gray-300  #d4d4d4  Text (secondary)
gray-50   #fafafa  Text (primary)
```

### Typography

#### CLI
- **Headings**: Bold, UPPERCASE sparingly, letter-spacing for impact
- **Body**: System default, high contrast on dark backgrounds
- **Code**: Monospace (JetBrains Mono, Fira Code, Monaco)

#### Web
```css
font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif;
mono-family: 'JetBrains Mono', 'Fira Code', 'Menlo', 'Monaco', monospace;
```

**Scale**:
- Headings: 1.25rem → 2rem (800-900 weight)
- Body: 0.938rem → 1rem (400-600 weight)  
- Small: 0.75rem → 0.875rem (600-700 weight)

### Icons & Symbols

#### Custom Icon Set
```
⚡ iconBolt       Energy, performance, speed
✓  iconCheck      Success, completion, verification
✗  iconCross      Error, failure, cancellation
→  iconArrow      Direction, progression, next step
•  iconDot        List items, separation
★  iconStar       Recommendations, favorites
▪  iconBox        Solid list markers
◉  iconCircle     Status indicators
◆  iconDiamond    Section headers, attention
›  iconChevron    Navigation, expansion
⇣  iconDownload   Downloads, receiving
⇡  iconUpload     Uploads, sending
⌕  iconSearch     Search, discovery
◭  iconModel      ML models, files
⟨⟩ iconCpu        CPU processing
⟪⟫ iconGpu        GPU processing
```

### Box Drawing

#### Characters
```
╭ ╮ ╯ ╰  Rounded corners (modern, friendly)
─ │      Straight lines (structure)
├ ┤ ┬ ┴  Connectors (relationships)
┼        Intersections (grids)
━        Bold separator (emphasis)
```

#### Usage Patterns
```
╔═══════════════╗
║   Banner      ║  Heavy boxes for major sections
╚═══════════════╝

╭─────────────╮
│  Card       │     Rounded for content blocks
╰─────────────╯

━━━━━━━━━━━━━━━━   Heavy line for dividers

──────────────────   Light line for subsections
```

## Component Patterns

### CLI Components

#### Banner
```
    ╔═══════════════════════════════════╗
    ║                                   ║
    ║      OFFGRID LLM  v0.1.0α        ║
    ║                                   ║
    ║   Edge Inference Orchestrator    ║
    ║                                   ║
    ╚═══════════════════════════════════╝
```

#### Section Header
```
◆ Section Title
──────────────────────────────────────────────────
```

#### Success Message
```
✓ Operation completed successfully
```

#### Error Message
```
✗ Operation failed: reason

Common causes:
  • First possible cause
  • Second possible cause
  
Next steps:
  → Try this command first
  → Or consult documentation
```

#### Info/Tip
```
→ Helpful information or next step
```

#### Warning
```
⚡ Important warning or processing state
```

#### Item with Details
```
Key:             Value
Another Key:     Another Value
```

#### List with Icon
```
  • First item
  • Second item
  • Third item
```

#### Progress/Status
```
⏬ Downloading model.gguf: 45.3% · 2.1 GB · 12.5 MB/s
```

#### Divider
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

### Web Components

#### Card
```css
background: var(--bg-surface);
border: 1px solid var(--border-default);
border-radius: 8px;
padding: 2rem;
box-shadow: var(--shadow-md);
```

#### Button Primary
```css
background: var(--brand-primary);
color: var(--gray-950);
font-weight: 600;
padding: 0.75rem 1.5rem;
border-radius: 6px;
transition: all 0.2s ease;
```

#### Input Field
```css
background: var(--bg-input);
border: 1px solid var(--border-default);
color: var(--text-primary);
padding: 0.75rem 1rem;
border-radius: 6px;
font-family: 'MonoLisa', monospace;
```

#### Status Badge
```css
display: inline-flex;
align-items: center;
gap: 0.5rem;
padding: 0.5rem 1rem;
background: var(--bg-elevated);
border: 1px solid var(--border-default);
border-radius: 6px;
font-size: 0.813rem;
font-weight: 600;
```

## Spacing System

### Scale
```
xs:  0.25rem  (4px)
sm:  0.5rem   (8px)
md:  1rem     (16px)
lg:  1.5rem   (24px)
xl:  2rem     (32px)
2xl: 3rem     (48px)
```

### Application
- **Tight grouping**: xs-sm (related elements)
- **Component padding**: md-lg (internal spacing)
- **Section separation**: lg-xl (distinct areas)
- **Major divisions**: xl-2xl (page-level structure)

## Animation & Motion

### Principles
- **Subtle**: Animations support, don't distract
- **Fast**: 150-250ms for micro-interactions
- **Purpose-driven**: Motion communicates state changes

### Patterns
```css
/* Pulse for status indicators */
@keyframes pulse {
    0%, 100% { opacity: 1; transform: scale(1); }
    50% { opacity: 0.6; transform: scale(1.2); }
}

/* Fade in for content */
@keyframes fadeIn {
    from { opacity: 0; transform: translateY(-4px); }
    to { opacity: 1; transform: translateY(0); }
}

/* Hover states */
transition: all 0.2s ease;
```

## Accessibility

### Contrast Ratios
- **Normal text**: 7:1 minimum (AAA standard)
- **Large text**: 4.5:1 minimum (AA standard)
- **Interactive elements**: Clear hover/focus states

### Focus Indicators
```css
:focus-visible {
    outline: 2px solid var(--brand-primary);
    outline-offset: 2px;
}
```

### Screen Readers
- Semantic HTML elements
- ARIA labels where needed
- Descriptive error messages

## CLI Output Guidelines

### Information Hierarchy
1. **Banner/Title**: Bold, clear, one per command
2. **Section headers**: Marked with icons (◆ ›)
3. **Body content**: Organized in logical groups
4. **Actions/Tips**: Highlighted with arrows (→)

### Message Types

#### Success Path
```
✓ Action completed
  Details about what happened
  
Next steps:
  → What you can do now
```

#### Error Path
```
✗ Action failed: specific reason

Why this happened:
  • Possible cause one
  • Possible cause two
  
Fix it:
  → Try this command
  → Or check this setting
```

#### Information
```
→ Helpful tip or guidance
⚡ Important notice
```

### Consistency Rules

1. **Always show context**: Where am I? What happened?
2. **Provide next steps**: What can I do now?
3. **Use consistent spacing**: Blank lines separate concepts
4. **Align data**: Tables and key-value pairs align nicely
5. **Color sparingly**: Only for status and emphasis

## Web UI Guidelines

### Layout Principles

1. **Grid-based**: Use CSS Grid for major layout
2. **Responsive**: Mobile-first, progressive enhancement
3. **Consistent spacing**: Use spacing scale throughout
4. **Visual hierarchy**: Size, weight, color communicate importance

### Interactive States

```css
/* Rest */
opacity: 1;
transform: scale(1);

/* Hover */
border-color: var(--brand-primary);
box-shadow: var(--shadow-glow);

/* Active */
transform: scale(0.98);

/* Focus */
outline: 2px solid var(--brand-primary);
outline-offset: 2px;

/* Disabled */
opacity: 0.5;
cursor: not-allowed;
```

## Implementation

### CLI Functions (Go)

```go
// Color constants
const (
    brandPrimary = "\033[38;5;45m"    // Cyan
    brandSuccess = "\033[38;5;78m"    // Green
    brandError = "\033[38;5;196m"     // Red
    colorReset = "\033[0m"
    colorBold = "\033[1m"
)

// Helper functions
func printSuccess(message string) {
    fmt.Printf("%s%s%s %s\n", brandSuccess, iconCheck, colorReset, message)
}

func printError(message string) {
    fmt.Printf("%s%s%s %s\n", brandError, iconCross, colorReset, message)
}

func printSection(title string) {
    fmt.Printf("%s%s%s %s%s\n", brandPrimary, iconDiamond, colorReset, colorBold, title)
    fmt.Printf("%s%s%s\n", brandMuted, strings.Repeat(boxH, 50), colorReset)
}

func printDivider() {
    fmt.Printf("%s%s%s\n", brandMuted, strings.Repeat(separator, 60), colorReset)
}
```

### Web CSS Variables

```css
:root {
    --brand-primary: #00d4ff;
    --brand-secondary: #af87ff;
    --brand-accent: #ffff00;
    --brand-success: #5fd787;
    --brand-error: #ff005f;
    
    --text-primary: #fafafa;
    --text-secondary: #d4d4d4;
    
    --bg-page: #000000;
    --bg-surface: #0a0a0a;
    --bg-elevated: #171717;
    
    --border-default: #404040;
}
```

## Examples

### Before (Plain)
```
Downloading model...
Progress: 45%
Done
```

### After (Branded)
```
⇣ Downloading model.gguf
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  Progress: 45.3% · 2.1 GB · 12.5 MB/s

✓ Download complete
```

### Before (Error)
```
Error: model not found
```

### After (Error)
```
✗ Model not found: llama-2-7b

Available models:
  • tinyllama-1.1b-chat.Q4_K_M
  • mistral-7b-instruct.Q5_K_M

→ Use 'offgrid list' to see all installed models
→ Use 'offgrid search llama' to find more models
```

## Maintenance

### Adding New Components

1. **Follow existing patterns**: Match style of similar components
2. **Use design tokens**: Always reference CSS variables/constants
3. **Test accessibility**: Check contrast, keyboard navigation
4. **Document usage**: Add to this guide with examples

### Updating Colors

1. **Update constants**: Modify in both CLI (Go) and Web (CSS)
2. **Test contrast**: Ensure WCAG AAA compliance
3. **Update documentation**: Reflect changes in this guide

## Resources

- **Box Drawing**: https://en.wikipedia.org/wiki/Box-drawing_character
- **ANSI Colors**: https://en.wikipedia.org/wiki/ANSI_escape_code
- **WCAG Guidelines**: https://www.w3.org/WAI/WCAG21/quickref/
- **Color Contrast**: https://contrast-ratio.com/

---

**Design Version**: 1.0  
**Last Updated**: November 2025  
**Maintained By**: OffGrid LLM Team
