# Changelog

All notable changes to this project will be documented in this file.

## [Phase 8: Replay-Verification & Ground-Truth Index] - 2026-09-06

### Added
- **Replay-verified heals:** the proxy now replays the *exact failing request* with the AI translation applied against the upstream API and only caches rules that return < HTTP 400.
- **Ground-truth breaking-change index:** every heal event is persisted to `data/heals.jsonl` as `{vendor, endpoint, old_schema, new_schema, patch, verified, replay_status, pr_url, timestamp}` — the cross-customer dataset that compounds.
- New `healstore` package: JSONL persistence, vendor inference from endpoints, PR-URL backfill, and vendor leaders stats.
- Agent endpoints: `POST /heal/record` (ground-truth ingestion + async PR trigger) and `GET /api/heals` (breaking-change index).
- Proxy endpoint: `GET /api/heals` proxies the index into the dashboard.
- **PR verification comments:** `CommentVerification` stamps `Replay Verification Passed/Failed (HTTP n)` on the opened PR — proof Dependabot/Renovate-class tools can't produce, since they never saw the failing traffic.
- Dashboard: new **Ground Truth** tab rendering the breaking-change index, verified/unverified badges, old→new schema migrations, and PR links.

### Changed
- PR opening moved off the proxy hot path into the agent's record handler.
- Agent calls in the proxy now use a `20s` timeout instead of blocking forever.
- Telemetry events carry a `verified` flag.

## [Phase 7: Live Telemetry & API Integration] - 2026-07-29

### Added
- Integrated live telemetry feed from the Go Proxy to the React Dashboard.
- Added `GET /api/telemetry` endpoint to the Fiber proxy to serve real-time metrics.
- Built a React `useEffect` polling engine in the dashboard to render healing events live.
- Updated the CLI to automatically boot the Mock API alongside the Proxy and Agent for streamlined local testing.

## [Phase 6: High-Performance & Unified CLI] - 2026-07-29

### Added
- Complete rewrite of the proxy router to use `github.com/gofiber/fiber/v2` for high throughput.
- Implemented `github.com/valyala/fasthttp` for optimized upstream API requests.
- Added `go-redis` distributed caching layer for AI translations with an in-memory fallback.
- Added a interactive CLI (`promptui`) with an elegant `Patchflow` selector menu.

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
