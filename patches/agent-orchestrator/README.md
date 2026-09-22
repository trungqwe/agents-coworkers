# Agent Orchestrator Minimal Patch

- **Target Base SHA**: `1140dd62dc7bb588b987e2c44aa1ff4796fa732b`
- **Component**: `backend/internal/cli/`
- **Reason**:
  - `backend/internal/domain/agentconfig.go` and `backend/internal/adapters/agent/codex/codex.go` natively support reasoning effort (`Effort` and `-c model_reasoning_effort=...`).
  - However, the CLI mirror struct `agentConfig` in `project.go` lacked the `Effort` field, causing project configs set/read via CLI to drop effort settings.
  - Furthermore, `ao spawn` exposed `--model` but did not expose `--effort`.
  - While desktop GUI and direct daemon API can configure effort in a zero-patch manner, this minimal patch enables fully headless CLI configuration.
- **Verification**:
  - `go test ./internal/cli`
  - `go test ./internal/adapters/agent/codex`
