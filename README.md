# Agents-Coworkers: AO + CLIProxyAPI Multi-Agent Integration

Integration templates, patches, tests, and audit evidence for operating a multi-agent development workforce with **Agent Orchestrator (AO)**, **CLIProxyAPI**, and **Codex CLI**.

---

## Architecture Overview

```text
Agent Orchestrator
|
+-- Orchestrator:  gpt-6-astra (effort: low, harness: codex, kind: orchestrator)
+-- Workers:       gemini-3.8-flash-high (effort: low, harness: codex, kind: worker)
|                  Target range: 3-7 parallel (initial verified target after auth: 3)
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

### Step 2: Interactive Account Authentication & Verification

#### Private Runtime Gateway Key:
For local runtime execution, copy the example config to a private gitignored file:
```powershell
Copy-Item "config\cliproxy\config.example.yaml" "config\cliproxy\config.runtime.yaml"
# Edit config.runtime.yaml to set your private gateway key
$env:CLIPROXY_KEY = "<YOUR_PRIVATE_KEY>"
```

#### Authentication Run:
Authenticate each account interactively:
```powershell
cd D:\TU_CODE\agent-orchestrator\CLIProxyAPI

# 1. Log in Codex accounts (repeat for each of your 6 ChatGPT Plus accounts):
.\cli-proxy-api.exe -config D:\TU_CODE\agents-coworkers\config\cliproxy\config.runtime.yaml -codex-login

# 2. Log in Antigravity accounts (repeat for each of your 8 Gemini Pro accounts):
.\cli-proxy-api.exe -config D:\TU_CODE\agents-coworkers\config\cliproxy\config.runtime.yaml -antigravity-login
```

#### Verification After Each Login:
Do NOT rely on process exit codes alone. CLIProxyAPI login functions have internal error paths that log and return without exiting non-zero. Verify BOTH:
1. Visible authentication success message in console output.
2. Run the sanitized auth inventory to verify count increases:
   ```powershell
   powershell -ExecutionPolicy Bypass -File D:\TU_CODE\agents-coworkers\scripts\auth-inventory.ps1
   ```
- **Login Verification Rule**:
  - `Success message AND credential count increases` -> New distinct account authenticated.
  - `Success message BUT count unchanged` -> Existing credential updated / possible repeated identity (e.g. logging into the same Google account again updates `antigravity-<email>.json`). This does NOT increment account pool capacity.
- **Target Counts**: Codex: 6 distinct accounts, Antigravity: 8 distinct accounts.
- **Privacy & Safety**:
  - `auth-inventory.ps1` outputs only safe identifiers (`ProviderType`, `CredentialHash`, `LastModified`). It NEVER dumps raw filenames, emails, account IDs, full paths, or token contents.
  - Never commit, share, or publish identity-bearing filenames or credential files.

### Step 3: Launch CLIProxyAPI Gateway
Start the local gateway:
```powershell
.\cli-proxy-api.exe -config D:\TU_CODE\agents-coworkers\config\cliproxy\config.runtime.yaml
```

Run fail-closed catalog smoke test (reuses running proxy or manages its own lifecycle if port is free):
```powershell
powershell -ExecutionPolicy Bypass -File D:\TU_CODE\agents-coworkers\scripts\smoke-cliproxy.ps1
```

Run Codex smoke test:
```powershell
# Before auth (strictly validates expected unauthenticated signature):
powershell -ExecutionPolicy Bypass -File D:\TU_CODE\agents-coworkers\scripts\smoke-codex-through-proxy.ps1 -ExpectAuthBlocked

# After auth (strict fail-closed verification):
powershell -ExecutionPolicy Bypass -File D:\TU_CODE\agents-coworkers\scripts\smoke-codex-through-proxy.ps1
```

---

### Step 4: Configuration & Spawning Paths

#### Path A: Zero Patch (Recommended / Default)
Upstream AO is kept clean at `1140dd62dc7bb588b987e2c44aa1ff4796fa732b`.
1. **Configure Role Model & Effort**:
   Configure orchestrator and worker roles via the **AO Desktop UI** or direct **daemon REST API** (`PUT /api/v1/projects/<PROJECT_ID>/config`).
   *(Do NOT use unpatched `ao project set-config --config-json` because unpatched CLI silently drops the `effort` field).*

   **Direct Daemon REST Example**:
   ```powershell
   # Dynamically discover daemon port from running.json handshake (default upstream port is 3001, but running.json is authoritative)
   $runFile = if ($env:AO_RUN_FILE) { $env:AO_RUN_FILE } else { Join-Path $HOME ".ao\running.json" }
   if (-not (Test-Path $runFile)) { throw "AO daemon run file not found: $runFile. Start the AO daemon first." }
   $runInfo = Get-Content $runFile -Raw | ConvertFrom-Json
   if (-not $runInfo.port) { throw "AO running.json does not contain a valid daemon port." }
   $aoBase = "http://127.0.0.1:$($runInfo.port)"
   $projectId = "<PROJECT_ID>"

   # Optional daemon sanity check
   Invoke-RestMethod -Uri "$aoBase/healthz" -Method Get

   $body = @'
   {
     "config": {
       "orchestrator": {
         "agent": "codex",
         "agentConfig": {
           "model": "gpt-6-astra",
           "effort": "low",
           "mode": "chat"
         }
       },
       "worker": {
         "agent": "codex",
         "agentConfig": {
           "model": "gemini-3.8-flash-high",
           "effort": "low",
           "mode": "chat"
         }
       }
     }
   }
   '@

   Invoke-RestMethod `
     -Uri "$aoBase/api/v1/projects/$projectId/config" `
     -Method Put `
     -Body $body `
     -ContentType "application/json"
   ```

2. **Verify Configuration**:
   The daemon endpoint `GET /api/v1/projects/<id>` returns a response envelope `{ "status": "ok", "project": { "config": { ... } } }`. Verify model and effort fail-closed:
   ```powershell
   $projectResponse = Invoke-RestMethod `
     -Uri "$aoBase/api/v1/projects/$projectId" `
     -Method Get

   if ($projectResponse.status -ne "ok") {
     throw "AO project read-back failed or returned degraded status: $($projectResponse.status)"
   }

   $config = $projectResponse.project.config
   if (-not $config) {
     throw "AO project read-back did not contain project.config"
   }

   if ($config.orchestrator.agentConfig.model -ne "gpt-6-astra") {
     throw "Unexpected orchestrator model: $($config.orchestrator.agentConfig.model)"
   }
   if ($config.orchestrator.agentConfig.effort -ne "low") {
     throw "Unexpected orchestrator effort: $($config.orchestrator.agentConfig.effort)"
   }
   if ($config.worker.agentConfig.model -ne "gemini-3.8-flash-high") {
     throw "Unexpected worker model: $($config.worker.agentConfig.model)"
   }
   if ($config.worker.agentConfig.effort -ne "low") {
     throw "Unexpected worker effort: $($config.worker.agentConfig.effort)"
   }

   Write-Host "[PASS] AO role model/effort configuration read-back verified."
   ```

3. **Spawn Sessions** (Omit `--effort` flag; session inherits role config):
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

#### Path B: Headless CLI (Optional / Patch Applied)
If headless CLI operation is strictly required:
1. Apply patch:
   ```powershell
   cd D:\TU_CODE\agent-orchestrator
   git apply D:\TU_CODE\agents-coworkers\patches\agent-orchestrator\0001-cli-support-agent-effort.patch
   cd backend
   go test -v -run "TestSpawnCommand_EffortFlagForwarded|TestProjectSetConfig_ConfigJSON_PreservesEffort" ./internal/cli
   go test ./internal/cli
   ```
2. Rebuild CLI and verify `ao spawn --help` contains `--effort`.
3. Apply project configuration via CLI:
   ```powershell
   $config = Get-Content "D:\TU_CODE\agents-coworkers\config\ao\project-config.example.json" -Raw
   ao project set-config <PROJECT_ID> --config-json $config
   ```
4. Spawn sessions with explicit `--effort`:
   ```powershell
   ao spawn `
     --project <PROJECT_ID> `
     --kind orchestrator `
     --agent codex `
     --name "Orchestrator" `
     --model "gpt-6-astra" `
     --effort low `
     --mode chat

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

## Sequential Post-Authentication Verification Gates

Do not scale directly to 7 workers upon logging in. The workforce must pass each gate sequentially:
1. `/v1/models` catalog validation
2. Direct Astra Responses verification
3. Direct Gemini Responses + tool call roundtrip
4. Codex -> Astra execution
5. Codex -> Gemini coding and tool loop
6. Real AO orchestrator + one Gemini worker
7. Orchestrator review / rework loop
8. Real 3-worker concurrency wave (initial verified target: 3; maximum live concurrency: TBD from runtime evidence)