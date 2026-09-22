# Architecture Decision: Unified Codex-Harness Multi-Agent System (Candidate A)

## Status: PROVISIONALLY SELECTED
- **Candidate A**: SOURCE-FEASIBLE
- **Runtime Decision**: PENDING AUTH GATES

Candidate A only transitions from **PROVISIONALLY SELECTED** to **ACCEPTED** after satisfying the 10 sequential runtime verification gates:
1. **CLIProxy Catalog**: `/v1/models` returns 200 OK with both `gpt-6-astra` and `gemini-3.8-flash-high` active.
2. **Direct Astra Responses**: Successful non-stream and stream responses from upstream OpenAI via CLIProxyAPI.
3. **Direct Gemini Responses**: Successful non-stream and stream responses from upstream Antigravity via CLIProxyAPI.
4. **Gemini Tool Roundtrip**: Verified tool calling fidelity through CLIProxyAPI translation layer.
5. **Codex -> Astra**: Successful `codex exec` invocation targeting `gpt-6-astra` through CLIProxyAPI.
6. **Codex -> Gemini Tool Loop**: Successful multi-turn coding and tool execution for `gemini-3.8-flash-high` through CLIProxyAPI.
7. **Real AO Orchestrator**: Verified live session launch on Agent Orchestrator with role `orchestrator`.
8. **Real AO Worker**: Verified live session launch on Agent Orchestrator with role `worker`.
9. **AO Rework Loop**: Verified orchestrator review, audit, and directive issuance to worker sessions.
10. **Real 3-Worker Wave**: Parallel execution of 3 concurrent live worker sessions without lock collisions or token exhaustion.

---

## 1. Chosen Architecture: Candidate A

**Unified Codex Client Harness via CLIProxyAPI Gateway**

```text
                  +-------------------------------------------------------+
                  |               Agent Orchestrator (AO)                 |
                  |  - Local REST API Daemon                              |
                  |  - Git Worktree Management (~/.ao/worktrees/...)      |
                  +---------------------------+---------------------------+
                                              |
                   +--------------------------+--------------------------+
                   |                                                     |
                   v                                                     v
      +-------------------------+                           +-------------------------+
      |  Orchestrator Session   |                           |    Worker Sessions      |
      |  Kind:    orchestrator  |                           |  Kind:    worker        |
      |  Harness: codex         |                           |  Harness: codex         |
      |  Model:   gpt-6-astra   |                           |  Model:   gemini-3.8-   |
      |  Effort:  low           |                           |           flash-high    |
      |  Mode:    chat          |                           |  Effort:  low           |
      |  Role:    Plan, Review, |                           |  Mode:    chat          |
      |           Audit, Issue  |                           |  Role:    Parallel      |
      |           Directives    |                           |           Module Dev    |
      |                         |                           |  Count:   3 to 7        |
      +------------+------------+                           +------------+------------+
                   |                                                     |
                   +--------------------------+--------------------------+
                                              |
                                              | HTTP POST /v1/responses (Wire: Responses)
                                              v
                             +---------------------------------+
                             |    CLIProxyAPI (Port 8317)      |
                             |  - Account Pool & Round-Robin   |
                             |  - Model Catalog & Translation  |
                             |  - Quota Refresh & Failover     |
                             +----------------+----------------+
                                              |
                     +------------------------+------------------------+
                     |                                                 |
                     v                                                 v
        +--------------------------+                     +--------------------------+
        |  OpenAI Codex Upstream   |                     |   Antigravity Upstream   |
        |  6x ChatGPT Plus Pool    |                     |   8x Gemini Pro Pool     |
        |  Serving: gpt-6-astra    |                     |  Serving: gemini-3.8-    |
        |                          |                     |          flash-high      |
        +--------------------------+                     +--------------------------+
```

---

## 2. Comparison Against Fallback Candidates

| Criteria | Candidate A (Codex Unified) | Candidate B (OpenCode Dual) | Candidate C (Agy Dual) |
| :--- | :--- | :--- | :--- |
| **AO Adapter Status** | Native adapter (`backend/internal/adapters/agent/codex`) | Native adapter exists (`backend/internal/adapters/agent/opencode`) | Native adapter exists (`backend/internal/adapters/agent/agy`) |
| **Client Harnesses** | 1 harness (`codex-cli 0.154.0`) | 2 harnesses (`codex` + `opencode`) | 2 harnesses (`codex` + `agy`) |
| **Adapter Modifications** | Zero adapter modifications needed | Zero adapter modifications needed | Zero adapter modifications needed |
| **Provider Routing** | Transparent via single `config.toml` to CLIProxyAPI | Separate OpenCode provider config & auth | Separate Agy CLI flags & provider config |
| **Moving Parts** | **Fewest moving parts** | Higher config maintenance surface | Higher config maintenance surface |

**Key Finding**:
AO upstream already includes native adapters for both OpenCode and Agy. Neither fallback requires a new AO adapter merely to exist. Candidate A is provisionally selected because it unifies all agent roles under a single client harness, minimizing system complexity. Fallback candidates B or C will only be considered if Candidate A encounters an unresolvable failure at a runtime boundary.

---

## 3. Concrete Component Specifications

### A. Orchestrator
- **Kind**: `orchestrator`
- **Harness**: `codex`
- **Model**: `gpt-6-astra`
- **Effort**: `low`
- **Mode**: `chat`
- **Spawn Command**:
  ```powershell
  ao spawn `
    --project <PROJECT_ID> `
    --kind orchestrator `
    --agent codex `
    --name "Orchestrator" `
    --model "gpt-6-astra" `
    --effort low `
    --mode chat
  ```

### B. Worker
- **Kind**: `worker`
- **Harness**: `codex`
- **Model**: `gemini-3.8-flash-high`
- **Effort**: `low`
- **Mode**: `chat`
- **Spawn Command**:
  ```powershell
  ao spawn `
    --project <PROJECT_ID> `
    --kind worker `
    --agent codex `
    --name "Worker-1" `
    --model "gemini-3.8-flash-high" `
    --effort low `
    --mode chat
  ```

### C. Local Gateway (CLIProxyAPI)
- **Port**: `8317` (Loopback `127.0.0.1`)
- **Config**: `config/cliproxy/config.example.yaml`
- **Account Pools**:
  - 6x ChatGPT Plus accounts for `gpt-6-astra`
  - 8x Gemini Pro accounts for `gemini-3.8-flash-high`
- **Routing Strategy**: Round-robin with automatic cooldown and quota failover.
