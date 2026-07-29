# Changelog

All notable changes to this project will be documented in this file.

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
