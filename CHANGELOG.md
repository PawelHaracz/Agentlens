# Changelog

All notable changes to AgentLens are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

### Added
- Capability-based discovery: new Capabilities tab in main navigation
- GET /api/v1/capabilities — list capability instances with agent metadata
- GET /api/v1/capabilities/{key} — get all agents offering a specific capability
- Capability registry extended with discoverability metadata (`DiscoverableKinds()` helper)
- Cross-linking: capability names on agent detail page now link to capability detail view
- URL state: capability list view supports shareable URLs with search and filter params

### Changed
- Capability registry: `RegisterCapability` signature now includes `discoverable bool` parameter

### Removed
- **BREAKING:** GET /api/v1/skills endpoint removed (replaced by /api/v1/capabilities)
- **BREAKING:** `SearchCapabilities(query string)` method removed from Store interface (replaced by `ListCapabilities(filter CapabilityFilter)`)

### Migration Notes
- If your code calls `GET /api/v1/skills`, replace with `GET /api/v1/capabilities`. The response shape is different — see docs/api.md.
- If you have custom store implementations, remove `SearchCapabilities` method and implement `ListCapabilities` and `ListAgentsByCapability`.
