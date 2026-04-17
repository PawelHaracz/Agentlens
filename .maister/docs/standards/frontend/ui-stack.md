# Frontend: UI Stack

shadcn/ui on top of Radix primitives with Tailwind CSS tokens.

### shadcn/ui Primitives via `@/components/ui/*`
Design-system primitives are imported from `@/components/ui/<name>` (alias to `web/src/components/ui/`). Higher-level components compose these rather than restyling raw HTML. Example: `import { Badge } from '@/components/ui/badge'`. Sources: code-patterns (23+ files), docs/developer-guide.md, ADR-007.

### PascalCase `.tsx` Filenames for Components
All React components in `src/components/` and `src/routes/**/components/` are PascalCase single-file modules. shadcn primitives under `components/ui/` are the lone kebab-case exception (they ship that way from the shadcn generator). Examples: `AuthBadge.tsx`, `CardPreview.tsx`, `RegisterAgentDialog.tsx`. Source: code-patterns (18 of 18 files).

### Folder Layout
`web/src/hooks/` for custom hooks (useCatalogQuery, useCapabilitiesQuery). `web/src/contexts/` for React contexts (AuthContext, ThemeContext). `web/src/pages/` for top-level non-feature pages (LoginPage, SettingsPage). `web/src/routes/<feature>/` for feature-scoped routes with co-located `components/` subfolders. Source: code-patterns.

### Tailwind + shadcn/ui HSL Tokens + Dark Mode Class
`web/tailwind.config.js` enables class-based dark mode (`darkMode: ['class']`) and extends the theme with shadcn/ui color tokens (`hsl(var(--x))`) for border/input/ring/background/foreground/primary/secondary/destructive/muted/accent/popover/card. Container centered with max-width 1400px at `2xl`. Plugin: `tailwindcss-animate`. Source: web/tailwind.config.js.
