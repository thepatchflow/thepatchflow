# Changelog

All notable changes to this project will be documented in this file.

## [Phase 6: High-Performance & Unified CLI] - 2026-07-29

### Added
- Complete rewrite of the proxy router to use `github.com/gofiber/fiber/v2` for high throughput.
- Implemented `github.com/valyala/fasthttp` for optimized upstream API requests.
- Added `go-redis` distributed caching layer for AI translations with an in-memory fallback.
- Added a Vercel-style interactive CLI (`promptui`) with an elegant `▲ Patchflow` selector menu.

### Changed
- Refactored `proxy`, `agent`, and `mock_api` out of independent scripts into modular packages.
- Unified the startup process into a single `main.go` orchestrator (`./patchflow serve`).
- Removed `node_modules` and JS tooling from the core repository.

## [Phase 5: Security / Dependabot Integration] - 2026-07-29

### Added
- Added `POST /webhook/dependabot` endpoint to `agent/main.go`.
- Implemented new AI prompt for structural code refactoring on dependency upgrades.
- Added `FixDependabotAlert` to `agent/github.go` to simulate repo cloning, AI code refactoring, and opening security PRs.

## [Phase 4: Dashboard] - 2026-07-29

### Added
- Created the initial version of the Patchflow Dashboard UI.
- Integrated the dashboard's production build (`dist/`) into the core repository.
- Added Vercel-inspired UI styling and responsive navigation.
- Stubbed out Services, Activity Logs, and Settings tabs for future dashboard features.

### Changed
- Refactored `patchflow-dashboard` to compile directly into the `patchflow/dist` directory.
- Updated `.gitignore` to track the `dist/` folder, allowing the Go proxy to serve the frontend directly.

## [Phase 3: Tkngate]

### Added
- Tkngate implementation.

## [Phase 2: PR Engine]

### Added
- PR Engine integration.

## [Phase 1: AI Brain]

### Added
- AI Brain infrastructure.
