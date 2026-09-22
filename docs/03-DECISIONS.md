# 03 — Architecture Decision Ledger

This ledger records architectural decisions, their current validation status, and revisit triggers.

## Status Definitions
- `PROPOSED`: Under initial evaluation; no implementation started.
- `PROVISIONAL`: Structurally feasible and selected for verification; awaiting runtime proof.
- `ACCEPTED`: Fully verified end-to-end against live runtime evidence.
- `REJECTED`: Disqualified due to technical or operational failure.
- `SUPERSEDED`: Replaced by a subsequent accepted decision.

---

## Decision Entries

### D001 — Candidate A Provisionally Selected
- **Status**: `PROVISIONAL`
- **Decision**: Select Candidate A (AO Orchestrator `gpt-6-astra` + AO Workers `gemini-3.8-flash-high`, unified Codex harness via CLIProxyAPI gateway) for runtime verification.
- **Reason**: Maximizes harness uniformity, minimizes client diversity, and leverages existing provider pools.
- **Evidence**: Source inspection of AO Codex adapter, CLIProxyAPI translators, schema tests.
- **Revisit Trigger**: Inability to pass any of the 10 sequential runtime verification gates.

### D002 — Unified Codex Client Harness
- **Status**: `PROVISIONAL`
- **Decision**: Use `agent = "codex"` for both orchestrator and worker roles (`INV-003`).
- **Reason**: Simplifies AO configuration, eliminates multi-adapter maintenance, and provides consistent tool execution.
- **Evidence**: AO Codex adapter unit tests (`0.336s`), local process execution.
- **Revisit Trigger**: Persistent failure in Codex -> Gemini multi-turn tool loops (Gate 6).

### D003 — CLIProxyAPI Local Routing Boundary
- **Status**: `PROVISIONAL`
- **Decision**: Route all LLM requests through a local loopback CLIProxyAPI instance on port `8317`.
- **Reason**: Centralizes OAuth credential rotation, account pooling (6 Plus + 8 Pro), and wire protocol translation.
- **Evidence**: Built executable starts, binds `127.0.0.1:8317`, handles `/v1/models`.
- **Revisit Trigger**: Routing deadlocks, memory leaks, or translation crashes during live multi-agent execution.

### D004 — AO Upstream Clean by Default
- **Status**: `ACCEPTED`
- **Decision**: Keep `agent-orchestrator` upstream tree clean at commit `1140dd62dc7bb588b987e2c44aa1ff4796fa732b` (`INV-002`).
- **Reason**: Avoids fork divergence, maintenance churn, and upstream merge overhead.
- **Evidence**: Upstream passes full test suites without local modifications.
- **Revisit Trigger**: Daemon architectural change that makes external configuration impossible.

### D005 — Zero-Patch Path A as Preferred Configuration
- **Status**: `ACCEPTED`
- **Decision**: Use AO Desktop GUI or direct daemon REST API (`PUT /api/v1/projects/<PROJECT_ID>/config`) to configure model and reasoning effort.
- **Reason**: Natively preserves `effort = "low"` without touching upstream Go source.
- **Evidence**: Controller routes verified (`projects.go:35`), schema tests prove unpatched CLI drops effort while daemon store preserves it.
- **Revisit Trigger**: Complete deprecation of Desktop GUI and daemon REST interfaces.

### D006 — Headless Effort Patch Maintained as Optional Fallback
- **Status**: `PROVISIONAL`
- **Decision**: Maintain `0001-cli-support-agent-effort.patch` in integration repo for headless CLI environments.
- **Reason**: Allows CLI-driven scripts to set effort and spawn sessions when GUI/REST are unavailable.
- **Evidence**: Unit tests (`TestSpawnCommand_EffortFlagForwarded`, `TestProjectSetConfig_ConfigJSON_PreservesEffort`) pass cleanly.
- **Revisit Trigger**: Upstream merges effort support into CLI mirror structs.

### D007 — Worker Concurrency Target Starts at 3, Not 7
- **Status**: `ACCEPTED`
- **Decision**: Target worker range is 3–7 parallel workers; initial verified target after auth is 3 (`INV-005`).
- **Reason**: Live multi-agent concurrency on this stack is unproven. Claiming 7 workers without empirical proof violates runtime verification principles.
- **Evidence**: Filesystem worktree scaling verified; live session concurrency pending Gate 10.
- **Revisit Trigger**: Successful execution of Gate 10 (3-worker wave) unblocks testing 5 and 7 workers.

### D008 — OpenCode and Agy as Dormant Fallbacks
- **Status**: `ACCEPTED`
- **Decision**: Maintain Candidate B (OpenCode) and Candidate C (Agy) as dormant fallbacks without active development.
- **Reason**: Upstream AO already includes native adapters for both. No custom adapter code is required.
- **Evidence**: Source inspection of `adapters/agent/opencode` and `adapters/agent/agy`.
- **Revisit Trigger**: Disqualification of Candidate A at Gate 4 or Gate 6.

### D009 — Runtime Proof Outranks Static Inference
- **Status**: `ACCEPTED`
- **Decision**: No component, candidate, or capability is labeled "PASS" without empirical runtime execution (`INV-004`).
- **Reason**: Prevents premature architectural commitments based on speculative code reading.
- **Evidence**: Pre-auth audit correctly halted at `BLOCKED_RUNTIME_AUTH`.
- **Revisit Trigger**: Fundamental change in verification policy (not permitted).

### D010 — Product Repository Remains Strictly Read-Only
- **Status**: `ACCEPTED`
- **Decision**: Keep `AI-Auto-Video-Creator` untouched and read-only (`INV-001`).
- **Reason**: Respects repository boundary and milestone locks (M2-P1..P7B closed; P8/P9 locked; M3 unauthorized).
- **Evidence**: `git status` clean at commit `4a7c8c921b7e05066505d51b168a02c3fde61317`.
- **Revisit Trigger**: Explicit user authorization changing product milestone boundaries.