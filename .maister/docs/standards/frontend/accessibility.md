## Accessibility

### Semantic HTML
Use appropriate elements (nav, main, button) that convey meaning to assistive technologies.

### Keyboard Navigation
Make all interactive elements accessible via keyboard with visible focus indicators.

### Color Contrast
Maintain 4.5:1 contrast for normal text; don't rely solely on color to convey information.

### Alt Text and Labels
Provide descriptive alt text for images and labels for form inputs.

### Screen Reader Testing
Verify all views work with screen readers.

### ARIA When Needed
Use ARIA attributes to enhance complex components when semantic HTML isn't enough.

### Heading Structure
Use heading levels (h1-h6) in proper order for clear document outline.

### Focus Management
Manage focus appropriately in dynamic content, modals, and SPAs.

## AgentLens-specific

### Icon-Only Buttons Need an Accessible Name
Icon-only controls (copy buttons, hamburger menu, accordion toggles) must expose an accessible name via `aria-label` or visually-hidden text. Accordion toggles need `aria-expanded` and `aria-controls`. Source: PR reviews (#18, #17, #9).

### Radix `TooltipTrigger` Uses `asChild` Over Custom Elements
`TooltipTrigger` wrapping custom elements like `Badge` must use `asChild` to avoid nested interactive elements and extra `<button>` wrappers. See `StatusBadge` as the canonical reference. Source: PR reviews (#18).
