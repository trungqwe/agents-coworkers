# Agents-Coworkers: AO + CLIProxyAPI Multi-Agent Integration

Integration templates, patches, tests, and audit evidence for operating a multi-agent development workforce with **Agent Orchestrator (AO)**, **CLIProxyAPI**, and **Codex CLI**.

---

## Architecture Overview

```text
Agent Orchestrator
|
+-- Orchestrator:  gpt-6-astra (effort: low, harness: codex)
+-- Workers:       gemini-3.8-flash-high (effort: low, harness: codex, 3-7 parallel)
|
+--> CLIProxyAPI Gateway (127.0.0.1:8317)
       +-- 6x ChatGPT Plus accounts -> gpt-6-astra
       +-- 8x Gemini Pro accounts   -> gemini-3.8-flash-high
```

---

## Upstream Repository Lock

| Repository | Reference SHA | Local HEAD | Status |
| :--- | :--- | :--- | :--- |
| [agent-orchestrator](https://github.com/Untrivial-ai/agent-orchestrator) | `1140dd62dc7bb588b987e2c44aa1ff4796fa732b` | `1140dd62dc7bb588b987e2c44aa1ff4796fa732b` | Exact Match |
| [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) | `2430354330af80b645f9ffb1a51e1e7c72c4cc8e` | `2430354330af80b645f9ffb1a51e1e7c72c4cc8e` | Exact Match |
| [AI-Auto-Video-Creator](https://github.com/trungqwe/AI-Auto-Video-Creator) | `4a7c8c921b7e05066505d51b168a02c3fde61317` | `4a7c8c921b7e05066505d51b168a02c3fde61317` | Read-Only Match |

- Rebuilt CLIProxyAPI binary: `D:\TU_CODE\agent-orchestrator\CLIProxyAPI\cli-proxy-api.exe`
- Binary SHA-256: `E9DA39B2491856BE2D3711E20BE89690F3465E2A5B7A469BA1A29AFF7AB342D2`

---

## Directory Structure

```text
agents-coworkers/
|-- AUDIT.md                                    # Comprehensive audit report & status matrix
|-- DECISION.md                                 # Architectural decision record (Candidate A)
|-- upstream-lock.json                          # Pinned SHA hashes and divergence records
|-- config/
|   |-- ao/project-config.example.json          # Nested RoleOverride configuration for AO
|   |-- cliproxy/config.example.yaml            # CLIProxyAPI pool routing configuration
|   `-- codex/config.example.toml               # Codex CLI provider configuration
|-- patches/
|   `-- agent-orchestrator/
|       `-- 0001-cli-support-agent-effort.patch # Minimal CLI effort flag wiring patch
|-- tests/
|   `-- integration/
|       `-- project_config_schema_test.go       # Schema regression test for AO project config
|-- scripts/
|   |-- preflight.ps1                           # Toolchain, git, and path validator
|   |-- smoke-cliproxy.ps1                      # Fail-closed CLIProxyAPI catalog smoke test
|   |-- smoke-ao.ps1                            # Git worktree primitive isolation test
|   |-- stress-workers.ps1                      # Git worktree concurrency scaling test (1, 3, 5, 7)
|   |-- smoke-codex-through-proxy.ps1           # Codex CLI through CLIProxyAPI test
|   `-- redact-evidence.ps1                     # Sensitive credential scrubber
`-- evidence/
    |-- run-20260923-audit-ao-cliproxy/         # Initial audit run
    `-- run-20260923-audit-correction/          # Corrected audit run with verified SHA-256
```

---

## Operational Runbook

### Step 1: Preflight Verification
Verify that Git, Go, and all required repositories are in place:
```powershell
powershell -ExecutionPolicy Bypass -File scripts\preflight.ps1
```

### Step 2: Interactive Account Authentication
Before starting the proxy, login to your OpenAI and Gemini accounts:
```powershell
cd D:\TU_CODE\agent-orchestrator\CLIProxyAPI

# Log in your 6 ChatGPT Plus accounts:
.\cli-proxy-api.exe -codex-login

# Log in your 8 Gemini Pro accounts:
.\cli-proxy-api.exe -antigravity-login
```
*Note: Credentials are saved securely to `C:\Users\Admin\.cli-proxy-api` and are never committed.*

### Step 3: Launch CLIProxyAPI
Start the gateway using the provided template configuration:
```powershell
.\cli-proxy-api.exe -config D:\TU_CODE\agents-coworkers\config\cliproxy\config.example.yaml
```

Verify that both models are active:
```powershell
powershell -ExecutionPolicy Bypass -File D:\TU_CODE\agents-coworkers\scripts\smoke-cliproxy.ps1
```

### Step 4: Configure Agent Orchestrator
Apply project configuration:
```powershell
ao project set-config --project <project-id> --file D:\TU_CODE\agents-coworkers\config\ao\project-config.example.json
```

### Step 5: Spawn Orchestrator & Workers
```powershell
# Spawn Orchestrator (GPT-6 Astra)
ao spawn --project <project-id> --agent codex --name "Orchestrator" --model "gpt-6-astra" --effort "low"

# Spawn Worker (Gemini 3.8 Flash High)
ao spawn --project <project-id> --agent codex --name "Worker-1" --model "gemini-3.8-flash-high" --effort "low"
```
