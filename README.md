# Patchflow

Patchflow is a self-healing API gateway and autonomic infrastructure layer. 
When an upstream API breaks due to a schema change or deprecation, Patchflow dynamically translates the payload in real-time to keep your application online, **replays the exact failing request to prove the fix works**, and asynchronously opens a Pull Request in your repository with a permanent fix — stamped with the replay-verification result.

Every healed request is recorded to `data/heals.jsonl` as a **ground-truth breaking-change index**: which vendor broke, the old→new schema migration, the applied patch, and whether the replay verified it. This dataset is Patchflow's compounding moat.

## Architecture

Patchflow consists of two planes:
1. **The Data Plane (Proxy):** A high-speed Go reverse proxy that intercepts traffic, catches HTTP 400s (schema breaking changes), applies cached translation rules, and replays requests.
2. **The Control Plane (AI Agent & PR Engine):** A heavy-lifting AI service that analyzes breaking API changes using LLMs, generates translation rules for the proxy, and patches the customer's codebase via the GitHub API.
3. **The Dashboard (UI):** A React-based web interface to monitor intercepted errors, track live autonomic healing, the ground-truth breaking-change index, and manage services. 

## Running Locally

Patchflow now ships as a single, unified binary for maximum developer experience.

1. Build the binary:
   ```bash
   go build -o patchflow
   ```
2. Run the interactive CLI:
   ```bash
   ./patchflow
   ```
   *Select `Start Backend (Proxy & Agent)` or `Start Dashboard UI` to boot the infrastructure.*

3. Send a failing request to the Proxy (runs on port 8080):
```bash
curl -X POST http://localhost:8080/v1/payments -d '{"charge": 15}'
```

Patchflow will intercept the failure, heal the request mid-flight, replay-verify the fix (HTTP 200), and return a 200 OK. The translation rule is cached, and the ground-truth event lands in the breaking-change index:

```bash
curl http://localhost:8080/api/heals
# [{"vendor":"payments","endpoint":"/v1/payments","old_schema":{"charge":"number"},
#   "new_schema":{"amount":"number"},"patch":{"charge":"amount"},
#   "verified":true,"replay_status":200,...}]
```

## Documentation
- [Frontend Integration](docs/frontend-integration.md): Details on how the UI is built and served.
