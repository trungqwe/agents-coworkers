# Agent Orchestrator Minimal Patch

- **Target Base SHA**: `1140dd62dc7bb588b987e2c44aa1ff4796fa732b`
- **Component**: `backend/internal/cli/`
- **File**: `0001-cli-support-agent-effort.patch`

---

## Patch Precondition & Supported Paths

Upstream AO checkout is intentionally kept **clean** at `1140dd62dc7bb588b987e2c44aa1ff4796fa732b` and does NOT contain this patch out of the box.

### Path 1: Zero Patch (Recommended / Default)
The AO daemon and Desktop UI natively support reasoning effort without any modifications:
- Configure orchestrator/worker models and effort levels via the AO Desktop GUI or direct daemon REST API calls.
- In CLI invocations (`ao spawn`), omit `--effort` (the session inherits the effort defined in the project/role config).
- Zero code changes required in the upstream AO repository.

### Path 2: Headless CLI (Optional)
If you require setting `--effort` as an explicit override flag on headless `ao spawn` CLI commands:
1. Apply the patch:
   ```powershell
   cd D:\TU_CODE\agent-orchestrator
   git apply D:\TU_CODE\agents-coworkers\patches\agent-orchestrator\0001-cli-support-agent-effort.patch
   ```
2. Validate CLI tests:
   ```powershell
   cd backend
   go test -v -run TestSpawnCommand_EffortFlagForwarded ./internal/cli
   go test ./internal/cli
   ```
3. Rebuild the AO CLI/daemon using the repository build process.
4. Verify flag presence:
   ```powershell
   ao spawn --help
   ```
   Confirm `--effort` is listed in the available flags.

*Note: Do NOT permanently patch upstream until runtime Candidate A requires headless use.*
