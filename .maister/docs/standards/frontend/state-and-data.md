# Frontend: State & Data Fetching

Server state goes through TanStack React Query; client-global state lives in React contexts.

### TanStack React Query for Server State
All REST fetches use `useQuery({ queryKey, queryFn })` from `@tanstack/react-query`. The catalog list uses `useCatalogQuery` with URL-synced filter state. Example: `useQuery({ queryKey: ['catalog', filter], queryFn: () => listCatalog(filter) })`. Sources: docs/architecture.md, ADR-007, code-patterns.

### AuthContext and ThemeContext for Client State
Session and theme state live in React context providers (`AuthContext`, `ThemeContext`) wrapping `<App>`. `AuthContext` exposes `user`, `isAuthenticated`, `permissions`, `hasPermission`. Sources: code-patterns (web/src/App.tsx, web/src/contexts/).

### Flat API Response Types in `web/src/types.ts`
All API types are defined in `web/src/types.ts` mirroring the flat JSON produced by `CatalogEntry.MarshalJSON()`. The `capabilities` field replaces the old `skills` field — each capability has a `kind` discriminator and a `properties` object. Source: docs/developer-guide.md.
