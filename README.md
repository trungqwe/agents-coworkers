# Agents-Coworkers: AO + CLIProxyAPI Multi-Agent Integration

Integration templates, patches, tests, and audit evidence for operating a multi-agent development workforce with **Agent Orchestrator (AO)**, **CLIProxyAPI**, and **Codex CLI**.

---

## Architecture Overview

```text
Agent Orchestrator
|
+-- Orchestrator:  gpt-6-astra (effort: low, harness: codex, kind: orchestrator)
+-- Workers:       gemini-3.8-flash-high (effort: low, harness: codex, kind: worker, 3-7 parallel)
|
+--> CLIProxyAPI Gateway (127.0.0.1:8317)
       +-- 6x ChatGPT Plus accounts -> gpt-6-astra
       +-- 8x Gemini Pro accounts   -> gemini-3.8-flash-high
```

---

## Upstream Repository Alignment

| Repository | Reference SHA | Local HEAD | Status |
| :--- | :--- | :--- | :--- |
| [agent-orchestrator](https://github.com/Untrivial-ai/agent-orchestrator) | `1140dd62dc7bb588b987e2c44aa1ff4796fa732b` | `1140dd62dc7bb588b987e2c44aa1ff4796fa732b` | Clean Match |
| [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) | `2430354330af80b645f9ffb1a51e1e7c72c4cc8e` | `2430354330af80b645f9ffb1a51e1e7c72c4cc8e` | Exact Match |
| [AI-Auto-Video-Creator](https://github.com/trungqwe/AI-Auto-Video-Creator) | `4a7c8c921b7e05066505d51b168a02c3fde61317` | `4a7c8c921b7e05066505d51b168a02c3fde61317` | Read-Only Match |

- Built CLIProxyAPI binary: `D:\TU_CODE\agent-orchestrator\CLIProxyAPI\cli-proxy-api.exe`
- Binary SHA-256: `E9DA39B2491856BE2D3711E20BE89690F3465E2A5B7A469BA1A29AFF7AB342D2`

---

## Operational Runbook

### Step 1: Preflight Verification
Verify toolchains, paths, and repositories:
```powershell
powershell -ExecutionPolicy Bypass -File scripts\preflight.ps1
```

### Step 2: Interactive Account Authentication
Log in your 6 ChatGPT Plus accounts and 8 Gemini Pro accounts:
```powershell
cd D:\TU_CODE\agent-orchestrator\CLIProxyAPI

# 1. Log in Codex accounts (repeat for each of your 6 ChatGPT Plus accounts):
.\cli-proxy-api.exe -config D:\TU_CODE\agents-coworkers\config\cliproxy\config.example.yaml -codex-login

# 2. Log in Antigravity accounts (repeat for each of your 8 Gemini Pro accounts):
.\cli-proxy-api.exe -config D:\TU_CODE\agents-coworkers\config\cliproxy\config.example.yaml -antigravity-login
```
*Credentials are stored in `C:\Users\Admin\.cli-proxy-api` and must never be committed.*

### Step 3: Launch CLIProxyAPI Gateway
Start the local gateway:
```powershell
.\cli-proxy-api.exe -config D:\TU_CODE\agents-coworkers\config\cliproxy\config.example.yaml
```

Run fail-closed catalog smoke test:
```powershell
powershell -ExecutionPolicy Bypass -File D:\TU_CODE\agents-coworkers\scripts\smoke-cliproxy.ps1
```

Run Codex smoke test:
```powershell
powershell -ExecutionPolicy Bypass -File D:\TU_CODE\agents-coworkers\scripts\smoke-codex-through-proxy.ps1
```

### Step 4: Configure Agent Orchestrator Project
AO requires supplying the full configuration object using `--config-json`:
```powershell
$config = Get-Content "D:\TU_CODE\agents-coworkers\config\ao\project-config.example.json" -Raw
ao project set-config <PROJECT_ID> --config-json $config
```

### Step 5: Spawning Orchestrator & Workers

#### Configuration Paths:
- **Path 1: Zero Patch (Recommended)**: Use AO Desktop GUI or daemon REST configuration for effort levels. Omit `--effort` from CLI commands:
  ```powershell
  # Spawn Orchestrator
  ao spawn `
    --project <PROJECT_ID> `
    --kind orchestrator `
    --agent codex `
    --name "Orchestrator" `
    --model "gpt-6-astra" `
    --mode chat

  # Spawn Worker
  ao spawn `
    --project <PROJECT_ID> `
    --kind worker `
    --agent codex `
    --name "Worker-1" `
    --model "gemini-3.8-flash-high" `
    --mode chat
  ```

- **Path 2: Headless CLI (With Effort Patch Applied)**:
  Apply `patches/agent-orchestrator/0001-cli-support-agent-effort.patch` and rebuild AO CLI. Then pass `--effort low`:
  ```powershell
  # Spawn Orchestrator with explicit effort flag
  ao spawn `
    --project <PROJECT_ID> `
    --kind orchestrator `
    --agent codex `
    --name "Orchestrator" `
    --model "gpt-6-astra" `
    --effort low `
    --mode chat

  # Spawn Worker with explicit effort flag
  ao spawn `
    --project <PROJECT_ID> `
    --kind worker `
    --agent codex `
    --name "Worker-1" `
    --model "gemini-3.8-flash-high" `
    --effort low `
    --mode chat
  ```

---

## Verification Gates (Post-Authentication)

Do not scale directly to 7 workers upon logging in. The workforce must pass each gate sequentially:
1. `/v1/models` catalog validation
2. Direct Astra Responses verification
3. Direct Gemini Responses + tool call roundtrip
4. Codex -> Astra execution
5. Codex -> Gemini coding and tool loop
6. Real AO orchestrator + one Gemini worker
7. Orchestrator review / rework loop
8. Real 3-worker concurrency wave
