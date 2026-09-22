# Agent Orchestrator Minimal Patch

- **Target Base SHA**: `1140dd62dc7bb588b987e2c44aa1ff4796fa732b`
- **Component**: `backend/internal/cli/`
- **File**: `0001-cli-support-agent-effort.patch`

---

## Critical Context: Zero-Patch CLI Effort Loss

At upstream commit `1140dd62dc7bb588b987e2c44aa1ff4796fa732b`, `backend/internal/cli/project.go` defines:
```go
type agentConfig struct {
    Model       string `json:"model,omitempty"`
    Mode        string `json:"mode,omitempty"`
    Permissions string `json:"permissions,omitempty"`
}
```
Because `agentConfig` in the unpatched CLI mirror struct lacks the `Effort` field:
1. `ao project set-config <id> --config-json <json>` parses `--config-json` into `projectConfig` -> `RoleOverride` -> `agentConfig`.
2. The `effort` field in JSON is silently discarded during `json.Unmarshal`.
3. The CLI marshals the request and sends it to the daemon without `effort`.
4. Furthermore, `ao spawn` has no `--effort` CLI flag.

---

## Supported Paths

### Path A: Zero Patch (Recommended / Default)
Upstream AO checkout is kept **100% clean** at `1140dd62dc7bb588b987e2c44aa1ff4796fa732b`.
- Configure orchestrator/worker models and effort levels via the AO Desktop GUI or direct daemon REST API (`PATCH /api/v1/projects/:id/config`).
- In CLI invocations (`ao spawn`), omit `--effort`. Sessions inherit effort from project/role configuration.
- Do NOT use unpatched `ao project set-config --config-json` expecting effort to be preserved.
- Verify configuration by reading back through daemon REST API:
  Assert:
  - `orchestrator.agentConfig.model == "gpt-6-astra"`
  - `orchestrator.agentConfig.effort == "low"`
  - `worker.agentConfig.model == "gemini-3.8-flash-high"`
  - `worker.agentConfig.effort == "low"`

### Path B: Headless CLI (Patch Applied)
If you require fully headless CLI operation with `ao project set-config --config-json` preserving effort and `ao spawn --effort`:
1. Apply the patch:
   ```powershell
   cd D:\TU_CODE\agent-orchestrator
   git apply D:\TU_CODE\agents-coworkers\patches\agent-orchestrator\0001-cli-support-agent-effort.patch
   ```
2. Validate CLI tests:
   ```powershell
   cd backend
   go test -v -run "TestSpawnCommand_EffortFlagForwarded|TestProjectSetConfig_ConfigJSON_PreservesEffort" ./internal/cli
   go test ./internal/cli
   ```
3. Rebuild the AO CLI using the repository build process (`go build ./cmd/ao`).
4. Verify flag presence:
   ```powershell
   ao spawn --help
   ```
   Confirm `--effort` is listed.
5. Apply project configuration via CLI:
   ```powershell
   $config = Get-Content "D:\TU_CODE\agents-coworkers\config\ao\project-config.example.json" -Raw
   ao project set-config <PROJECT_ID> --config-json $config
   ```
6. Read back project configuration and verify effort was preserved.

*Note: Do NOT permanently patch upstream until runtime Candidate A requires headless use.*
