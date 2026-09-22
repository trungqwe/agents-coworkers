# 06 — Operational Runbook

This document is the authoritative operational runbook for executing the multi-agent integration.

---

## 1. Toolchain & Environment Preflight

Verify prerequisites (Go 1.24+, PowerShell 7+, Git, Codex CLI, Node.js/npm):
```powershell
powershell -ExecutionPolicy Bypass -File scripts\preflight.ps1
```

---

## 2. Private Runtime Configuration

To prevent exposing gateway secrets in version control (`INV-007`):
```powershell
# Copy template to private gitignored file
Copy-Item "config\cliproxy\config.example.yaml" "config\cliproxy\config.runtime.yaml"

# Set runtime gateway key for local sessions
$env:CLIPROXY_KEY = "<YOUR_PRIVATE_KEY>"
```

---

## 3. Interactive Account Authentication (Phase 2)

Authenticate the minimum viable credential set (**1 Codex + 1 Antigravity**) before scaling:

```powershell
cd D:\TU_CODE\agent-orchestrator\CLIProxyAPI

# 1. Authenticate one ChatGPT Plus account:
.\cli-proxy-api.exe -config D:\TU_CODE\agents-coworkers\config\cliproxy\config.runtime.yaml -codex-login

# 2. Authenticate one Gemini Pro Google account:
.\cli-proxy-api.exe -config D:\TU_CODE\agents-coworkers\config\cliproxy\config.runtime.yaml -antigravity-login
```

### Verification After Each Login:
Do NOT rely on exit codes alone. CLIProxyAPI error paths may log and return without exiting non-zero. Verify:
1. Visible success message in the interactive terminal.
2. Run the sanitized inventory to verify count increases:
   ```powershell
   powershell -ExecutionPolicy Bypass -File D:\TU_CODE\agents-coworkers\scripts\auth-inventory.ps1
   ```
- **Login Verification Rule**:
  - `Success message AND credential count increases` -> New distinct account authenticated.
  - `Success message BUT count unchanged` -> Existing credential updated (relogin with same Google identity). This does NOT increment pool size.
- **Privacy Assurance**: The inventory script prints only `ProviderType`, `CredentialHash`, and `LastModified`. It never prints raw filenames, emails, full paths, or tokens.

---

## 4. Local Gateway Operations

### Launch Gateway:
```powershell
cd D:\TU_CODE\agent-orchestrator\CLIProxyAPI
.\cli-proxy-api.exe -config D:\TU_CODE\agents-coworkers\config\cliproxy\config.runtime.yaml
```

### Run Catalog Smoke:
```powershell
# Reuses running gateway or launches an isolated instance; fails closed if target models missing
powershell -ExecutionPolicy Bypass -File D:\TU_CODE\agents-coworkers\scripts\smoke-cliproxy.ps1
```

### Run Codex CLI Smoke:
```powershell
# Pre-auth check (requires documented unauthenticated error signature):
powershell -ExecutionPolicy Bypass -File D:\TU_CODE\agents-coworkers\scripts\smoke-codex-through-proxy.ps1 -ExpectAuthBlocked

# Post-auth check (strict fail-closed verification):
powershell -ExecutionPolicy Bypass -File D:\TU_CODE\agents-coworkers\scripts\smoke-codex-through-proxy.ps1
```

---

## 5. Agent Orchestrator Daemon Discovery & Configuration

### Dynamic Daemon Discovery Policy (`INV-006`):
The documented daemon discovery runbook logic (PowerShell snippet below) searches candidates in strict priority order, validates each via `/healthz`, confirms PID equality, and **fails closed** if multiple live daemons are running:

```powershell
$candidates = if ($env:AO_RUN_FILE) { @($env:AO_RUN_FILE) } else { @(
    (Join-Path $HOME ".ao\dev\running.json"),
    (Join-Path $HOME ".ao\running.json")
) }
$liveDaemons = @()
foreach ($candidate in ($candidates | Select-Object -Unique)) {
    if (-not (Test-Path $candidate)) { continue }
    try {
        $runInfo = Get-Content $candidate -Raw | ConvertFrom-Json
        $port = [int]$runInfo.port
        if ($port -lt 1 -or $port -gt 65535) { continue }
        $base = "http://127.0.0.1:$port"
        $health = Invoke-RestMethod `
            -Uri "$base/healthz" `
            -Method Get `
            -TimeoutSec 2
        if ($health.status -ne "ok") { continue }
        if ($health.service -ne "agent-orchestrator-daemon") { continue }
        if ($runInfo.pid -and $health.pid -and ([int]$runInfo.pid -ne [int]$health.pid)) { continue }
        $liveDaemons += [PSCustomObject]@{
            RunFile = $candidate
            BaseUrl = $base
            Port    = $port
            PID     = $health.pid
        }
    } catch { continue }
}
if ($liveDaemons.Count -eq 0) {
    throw "No valid live AO daemon found. Start AO or set AO_RUN_FILE explicitly."
}
if ($liveDaemons.Count -gt 1) {
    throw "Multiple live AO daemons found. Set AO_RUN_FILE explicitly to the intended daemon before mutating project configuration."
}
$aoBase = $liveDaemons[0].BaseUrl
Write-Host "Connected to AO Daemon on: $aoBase" -ForegroundColor Green
```

### Path A: Zero-Patch Configuration (Default)
Apply project role configuration via daemon REST `PUT`:
```powershell
$projectId = "<PROJECT_ID>"

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

# Read-back verification (response envelope: { status: "ok", project: { config: ... } })
$projectResponse = Invoke-RestMethod `
  -Uri "$aoBase/api/v1/projects/$projectId" `
  -Method Get

if ($projectResponse.status -ne "ok") {
  throw "AO project read-back failed: $($projectResponse.status)"
}
$config = $projectResponse.project.config
if (-not $config) {
  throw "Missing project.config in response"
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
Write-Host "[PASS] AO role configuration read-back verified." -ForegroundColor Green
```

### Spawning Sessions (Zero-Patch Path A):
```powershell
# Spawn Orchestrator (inherits effort='low' from project config)
ao spawn `
  --project <PROJECT_ID> `
  --kind orchestrator `
  --agent codex `
  --name "Orchestrator" `
  --model "gpt-6-astra" `
  --mode chat

# Spawn Worker (inherits effort='low' from project config)
ao spawn `
  --project <PROJECT_ID> `
  --kind worker `
  --agent codex `
  --name "Worker-1" `
  --model "gemini-3.8-flash-high" `
  --mode chat
```

### Path B: Headless CLI Spawning (Optional Patch)
If headless CLI operation is required:
```powershell
cd D:\TU_CODE\agent-orchestrator
git apply D:\TU_CODE\agents-coworkers\patches\agent-orchestrator\0001-cli-support-agent-effort.patch
cd backend
go test -v -run "TestSpawnCommand_EffortFlagForwarded|TestProjectSetConfig_ConfigJSON_PreservesEffort" ./internal/cli
go build -o ao.exe ./cmd/ao

# Apply config via CLI:
$config = Get-Content "D:\TU_CODE\agents-coworkers\config\ao\project-config.example.json" -Raw
.\ao.exe project set-config <PROJECT_ID> --config-json $config

# Spawn with explicit --effort:
.\ao.exe spawn --project <PROJECT_ID> --kind orchestrator --agent codex --name "Orchestrator" --model "gpt-6-astra" --effort low --mode chat
.\ao.exe spawn --project <PROJECT_ID> --kind worker --agent codex --name "Worker-1" --model "gemini-3.8-flash-high" --effort low --mode chat
```
