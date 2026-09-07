# Patchflow: Project Instructions & Context

This file is automatically loaded by the AI whenever a new conversation starts in this workspace. It provides the full context of what Patchflow is, the history of what has been built, and the roadmap for what to build next.

## 🚀 The Mission (YC Fall 2026 Strategy)
Patchflow is building **Autonomic Infrastructure**. 
The core pitch: "When an API changes, our agent fixes every customer's code automatically." 
We are targeting the "Self-Maintaining APIs" YC Request for Startups. Patchflow is software that fixes itself when the world changes (API schemas break, SDKs update, or security vulnerabilities like Dependabot alerts appear).

## 🏗️ Architecture & History
The core MVP is complete. Patchflow is structured in two planes:
1. **The Proxy (Data Plane - `proxy/main.go`):** Fiber reverse proxy that intercepts traffic, catches HTTP 400s, applies cached translation rules, and **replays the exact failing request against the upstream to verify the fix** (only < 400 responses get cached).
2. **The AI Agent (Control Plane - `agent/main.go`):** Receives the failed payload + error, uses OpenAI (JSON-mode) to deduce a key-mapping translation patch, and records every heal to the ground-truth index.
3. **The PR Engine (`agent/github.go`):** Opens automated PRs via the GitHub API and stamps a **replay-verification comment** on the PR (proven live, not by unit tests). Uses string manipulation today; AST rewriting is the known next upgrade.
4. **The Dashboard:** React UI served at `/dashboard`, including the new **Ground Truth** tab rendering the breaking-change index.

## 🥷 The Moat (what Dependabot/Renovate/Merge structurally miss)
- **Two-sided convergence:** runtime heal (zero downtime) *and* permanent code fix (PR). Static bots only fix code later; middleware only translates traffic forever. Patchflow is the only agent that spans both.
- **Ground-Truth Breaking-Change Index:** every heal writes to `data/heals.jsonl` via the `healstore` package: `{vendor, endpoint, old_schema, new_schema, patch, verified, replay_status, pr_url, timestamp}`. This dataset is the compounding network effect — the more customers, the faster every future heal.
- **Replay-verified PRs:** we prove a fix by replaying the exact failing request. Dependabot-class tools can never do this because they never saw the failing traffic.

## 🗺️ Roadmap & Next Steps

### Phase 8: Replay-Verification & Ground-Truth Index ✅ (COMPLETED - 2026-09-06)
- Proxy replay-verifies every AI rule before caching; unverified rules are rejected.
- New `healstore` package: JSONL persistence, vendor inference, PR-URL backfill, vendor `Leaders()` stats.
- Agent: `POST /heal/record`, `GET /api/heals`; Proxy proxy-protects `GET /api/heals`; Dashboard **Ground Truth** tab.
- `CommentVerification` stamps verification results on opened PRs.
- PR opening moved off the proxy hot path into the agent's record handler; 20s agent-call timeout added.

### Phase 9: Predictive Proactive Heals - In Progress
Convert the index from reactive to **predictive**: watch vendor changelogs/OpenAPI diffs and pre-generate translation rules + PRs before customer traffic breaks.

**Done (2026-09-06):**
- `healstore.Predictions()` — recurring migration signals ranked by confidence (seen-count / index size); exposed at `GET /api/predictions` (agent + proxy).
- `get /api/vendors` exposes `Leaders()` per-vendor stats (total heals, verified, distinct endpoints).
- `./patchflow seed` idempotently seeds a multi-vendor labeled index (Stripe charge→amount, card→source; Twilio sid→account_sid; OpenAI model→deployed_model; Slack channel→channel_id) for immediate demos.
- Dashboard **Ground Truth** tab now renders **Proactive Heal Predictions** (confidence %, seen count, last heal) above the index.
- Separate main: `llmClient()` routes via Tkngate Zero-Trust sidecar → falls back to raw `OPENAI_API_KEY` → only mocks when neither exists.

**Next:** (1) run a real `GITHUB_TOKEN` end-to-end to prove the replay-verification comment on an actual PR, (2) watch vendor changelogs/OpenAPI diffs to trigger pre-emptive predictions, (3) expose predictions for a *single* customer's endpoints as "you will break next".

### Phase 3: Zero-Trust Security (Tkngate Integration) - Partially Done
The agent routes through `TKNGATE_PROXY_TOKEN` when set, but the real Tkngate sidecar wiring is incomplete. Enterprise CISOs will not allow raw API keys inside an AI agent due to Prompt Injection risks.
**Next Step:** Integrate `tkngate` (https://github.com/tkngate/tkngate) as a Zero-Trust sidecar so the real OpenAI key never lives in the agent.

### Phase 4: Customer Dashboard (Completed)
React frontend for log-in, GitHub repo connect, live telemetry, and the ground-truth index.

### Phase 5: Security / Dependabot (Simulated)
`POST /webhook/dependabot` exists; AI code-refactoring on dependency upgrades is simulated. Needs real compiler-error-driven refactoring.

### Phase 6: High-Performance Infrastructure (Completed)
Fiber router, fasthttp upstream client, Redis cache with in-memory fallback.

## ⚠️ AI Behavioral Rules
- **Do not ask trivial permission:** When building roadmap phases, bias towards execution. Write the code and push it to a new branch rather than waiting for plan approvals, unless the architectural change is destructive.
- **Branching:** each phase gets its own branch (`phase-N-<slug>`) matching the repo's existing convention; merge/push to `main` only when explicitly requested.
- **Code Language:** Stick to Go for backend services. Use React/Next.js for the frontend.