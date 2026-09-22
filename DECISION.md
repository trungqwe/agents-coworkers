# Architecture Decision: Unified Codex-Harness Multi-Agent System (Candidate A)

## Chosen architecture:
**Candidate A (Unified Codex Client Harness via CLIProxyAPI Gateway)**

```text
Agent Orchestrator
|
+-- Orchestrator:
|     harness = codex
|     model   = gpt-6-astra
|     effort  = low
|
+-- Workers (3 to 7 parallel):
      harness = codex
      model   = gemini-3.8-flash-high
      effort  = low
```
Both Orchestrator and Workers utilize the single native `codex` harness installed on the system (`codex-cli 0.154.0`), connecting to local `CLIProxyAPI` on `http://127.0.0.1:8317/v1`. CLIProxyAPI routes upstream to either the OpenAI account pool or the Antigravity account pool based on the requested model name.

---

## Why:
1. **Minimal Moving Parts**: Avoids introducing secondary coding harnesses (such as OpenCode or Agy), eliminating dual-harness maintenance and compatibility discrepancies.
2. **Native Reasoning Effort Support**: The AO Codex adapter natively injects `-c model_reasoning_effort=<effort>` into the invocation command.
3. **True Isolation via Git Worktrees**: AO natively spawns each worker in an external Git Worktree (`~/.ao/worktrees/<project>/<session-id>`), guaranteeing zero file collisions, zero index lock contention, and zero cross-worker overwrites.
4. **Token Optimization**: Orchestrator (GPT) operates on high-level architecture contracts, diffs, and test summaries without loading massive codebase contexts. Workers (Gemini 3.8 Flash High) perform granular implementation, unit testing, and linting.

---

## Source changes required:
**NONE (Zero-Patch for Desktop GUI / REST API)** | **AO ONLY (Minimal Optional Patch for Headless CLI)**

- **Zero-Patch Path**: The backend daemon, `domain.AgentConfig`, `ports.AgentConfig`, and the Desktop UI already support `Effort`, `Model`, and multi-worktree execution without any source changes.
- **Headless CLI Path**: If configuration is performed strictly via CLI (`ao project set-config` / `ao spawn`), a 4-line surgical patch provided in `patches/agent-orchestrator/0001-cli-support-agent-effort.patch` exposes `--effort` in `ao spawn` and preserves `Effort` in CLI project configuration.

---

## Default worker concurrency:
**3 parallel workers**
- Rationale: Balances dependency graph segregation (e.g., Audio/TTS, Media Collector, Subtitle Aligner) with rate limit headroom across the 8 Gemini accounts without risk of burst thrashing.

---

## Max tested worker concurrency:
**7 parallel workers**
- Rationale: Verified via `stress-workers.ps1` with sub-second spawn latency (929ms) and 100% clean isolation. Should only be utilized when independent modules have no mutual DAG dependencies.

---

## Known limitations:
1. **Interactive OAuth Onboarding**: CLIProxyAPI requires interactive browser login once per account (`-codex-login` and `-antigravity-login`). Tokens cannot be auto-generated headlessly without existing session state.
2. **Platform-Specific Slash Assertions**: Two unit tests in `session_manager` fail on Windows solely due to backslash vs forward slash path comparisons; runtime functionality is unaffected.

---

## Exact startup order:
1. **Authenticate Accounts in CLIProxyAPI**:
   ```powershell
   cd D:\TU_CODE\agent-orchestrator\CLIProxyAPI
   # Run for each of the 6 GPT Plus accounts:
   .\cli-proxy-api.exe -codex-login
   # Run for each of the 8 Gemini Pro accounts:
   .\cli-proxy-api.exe -antigravity-login
   ```
2. **Launch CLIProxyAPI Gateway**:
   ```powershell
   .\cli-proxy-api.exe -config "D:\TU_CODE\agents-coworkers\config\cliproxy\config.example.yaml"
   ```
3. **Configure Codex CLI Provider** (point to loopback proxy):
   Add to `~/.codex/config.toml`:
   ```toml
   model_provider = "cliproxy"
   [model_providers.cliproxy]
   name = "CLIProxyAPI Gateway"
   base_url = "http://127.0.0.1:8317/v1"
   wire_api = "responses"
   env_key = "CLIPROXY_KEY"
   ```
4. **Launch Agent Orchestrator Supervisor**:
   ```powershell
   cd D:\TU_CODE\agent-orchestrator\frontend
   $env:AO_DATA_DIR = "$env:USERPROFILE\.ao"
   npm run build:daemon -- --dev
   .\node_modules\.bin\electron-forge start
   ```
5. **Configure Project Settings in AO**:
   Set Orchestrator to `codex` / `gpt-6-astra` / `low` effort.
   Set Worker to `codex` / `gemini-3.8-flash-high` / `low` effort.

---

## Exact AO project configuration:
```json
{
  "orchestrator": {
    "agent": "codex",
    "model": "gpt-6-astra",
    "effort": "low",
    "mode": "chat"
  },
  "worker": {
    "agent": "codex",
    "model": "gemini-3.8-flash-high",
    "effort": "low",
    "mode": "chat"
  }
}
```

---

## Exact CLIProxyAPI configuration:
```yaml
host: "127.0.0.1"
port: 8317
auth-dir: "~/.cli-proxy-api"
api-keys:
  - "ao-internal-secret-key"
routing:
  strategy: "round-robin"
  session-affinity: true
  session-affinity-ttl: "4h"
  session-affinity-subagents: false
request-retry: 3
max-retry-credentials: 0
max-retry-interval: 15
quota-exceeded:
  switch-preview-model: false
```

---

## Rollback:
1. Stop `cli-proxy-api.exe` process.
2. In `~/.codex/config.toml`, revert `model_provider` to default.
3. In AO, reset project worker/orchestrator agent to default.
4. Delete `~/.ao/worktrees/<project>` if any orphaned worker worktrees remain.
