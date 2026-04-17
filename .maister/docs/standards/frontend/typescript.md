# Frontend: TypeScript Conventions

TypeScript strict mode with a dedicated path alias. Enforced by `tsc --noEmit` in pre-commit and CI.

### Strict TypeScript + Unused-Symbol Checks
Frontend TS compilation uses `strict: true`, `noUnusedLocals`, `noUnusedParameters`, `noFallthroughCasesInSwitch`, `isolatedModules`, and `noEmit` (Vite handles bundling). Target: ES2020; `moduleResolution: bundler`; JSX: `react-jsx`. Type-check via `make web-lint` (`bunx tsc --noEmit`). Source: web/tsconfig.json, CONTRIBUTING.md.

### `@/*` Path Alias for `src/`
Imports under `@/*` resolve to `./src/*`. The alias is declared in TypeScript (`web/tsconfig.json`), Vite (`web/vite.config.ts`), and Vitest (`web/vitest.config.ts`) so builds, type-checks, and tests stay consistent. Example: `import { cn } from '@/lib/utils'`. Source: web/tsconfig.json, web/vite.config.ts, web/vitest.config.ts.
