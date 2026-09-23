# 03 — Architecture Decision Ledger

This ledger records architectural decisions, their current validation status, evidence maturity, and revisit triggers.

- **Active Roadmap Phase**: **Phase 6 — 3-Worker Concurrency Proof (COMPLETE)**; **Phase 7 remains NOT AUTHORIZED**; **Phase 8 remains DEFERRED / OPTIONAL** (authoritative phase status and exit criteria are maintained in [04-ROADMAP.md](04-ROADMAP.md)).

## Status Definitions

### Decision Status
- `PROPOSED`: Under initial evaluation; no implementation started.
- `PROVISIONAL`: Structurally feasible and selected for verification; awaiting runtime proof.
- `ACCEPTED`: Formally adopted as current project policy/design. It does NOT by itself mean every runtime capability involved has been empirically verified.
- `REJECTED`: Disqualified due to technical or operational failure.
- `SUPERSEDED`: Replaced by a subsequent accepted decision.

### Evidence Maturity
- `STATIC`: Source-level verification, schema inspection, or static repository boundary audit (L0/L1).
- `UNIT_VERIFIED`: In-memory unit or integration test passing in development tree (L2/L3).
- `LOCAL_RUNTIME`: Local loopback or isolated process execution verified (L4).
- `LIVE_RUNTIME`: Verified end-to-end against live external authenticated provider or supervised session runtime (L5/L6/L7).
- `NOT_APPLICABLE`: Definitive policy, governance, or boundary rule with no direct runtime test requirement.

---

## Decision Entries

### D001 — Candidate A Accepted
- **Decision Status**: `ACCEPTED`
- **Evidence Maturity**: `LIVE_RUNTIME`
- **Decision**: Adopt Candidate A (AO Orchestrator `gpt-6-astra` + AO Workers `gemini-3.8-flash-high`, unified Codex harness via CLIProxyAPI gateway) as the accepted architectural baseline (empirically validated across Gates 1–10).
- **Reason**: Maximizes harness uniformity, minimizes client diversity, and leverages existing provider pools.
- **Evidence**: Source inspection of AO Codex adapter, CLIProxyAPI translators, schema tests.
- **Revisit Trigger**: Revisit Candidate A only after a runtime gate has an unresolved, reproducible failure that has been root-cause isolated to a Candidate A architectural boundary. Configuration mistakes, authentication mistakes, transient provider errors, or unrelated local failures are not architecture-revisit triggers. Candidate B/C must remain dormant until such a failure is proven.

### D002 — Unified Codex Client Harness
- **Decision Status**: `PROVISIONAL`
- **Evidence Maturity**: `LIVE_RUNTIME`
- **Decision**: Use `agent = "codex"` for both orchestrator and worker roles (`INV-003`).
- **Reason**: Simplifies AO configuration, eliminates multi-adapter maintenance, and provides consistent tool execution.
- **Evidence**: AO Codex adapter unit tests (`0.336s`), local process execution.
- **Revisit Trigger**: Persistent failure in Codex -> Gemini multi-turn tool loops (Gate 6) root-cause isolated to harness incompatibility.

### D003 — CLIProxyAPI Local Routing Boundary
- **Decision Status**: `PROVISIONAL`
- **Evidence Maturity**: `LIVE_RUNTIME`
- **Decision**: Route all LLM requests through a local loopback CLIProxyAPI instance on port `8317`.
- **Reason**: Centralizes credential selection, provider routing, availability/failover, account pooling (target: 6 Plus + 8 Pro), and wire protocol translation. Multiple credentials are for authorized availability/distribution within provider terms; they are not used to evade rate limits, quotas, or provider restrictions.
- **Evidence**: Built executable starts, binds `127.0.0.1:8317`, handles `/v1/models`.
- **Revisit Trigger**: Unresolved routing deadlocks, memory leaks, or translation crashes during live multi-agent execution root-cause isolated to CLIProxyAPI gateway boundary.

### D004 — AO Upstream Clean by Default
- **Decision Status**: `ACCEPTED`
- **Evidence Maturity**: `UNIT_VERIFIED`
- **Decision**: Keep `agent-orchestrator` upstream tree clean at commit `1140dd62dc7bb588b987e2c44aa1ff4796fa732b` (`INV-002`).
- **Reason**: Avoids fork divergence, maintenance churn, and upstream merge overhead.
- **Evidence**: Upstream passes full test suites without local modifications.
- **Revisit Trigger**: Upstream architectural change that makes external configuration impossible.

### D005 — Zero-Patch Path A as Preferred Configuration
- **Decision Status**: `ACCEPTED`
- **Evidence Maturity**: `UNIT_VERIFIED`
- **Decision**: Use AO Desktop GUI or direct daemon REST API (`PUT /api/v1/projects/<PROJECT_ID>/config`) to configure model and reasoning effort.
- **Reason**: Natively preserves `effort = "low"` without touching upstream Go source.
- **Evidence**: Controller routes verified (`projects.go:35`), schema tests prove unpatched CLI drops effort while daemon store preserves it.
- **Revisit Trigger**: Complete deprecation of Desktop GUI and daemon REST interfaces.

### D006 — Headless Effort Patch Maintained as Optional Fallback
- **Decision Status**: `PROVISIONAL`
- **Evidence Maturity**: `UNIT_VERIFIED`
- **Decision**: Maintain `0001-cli-support-agent-effort.patch` in integration repo for headless CLI environments.
- **Reason**: Allows CLI-driven scripts to set effort and spawn sessions when GUI/REST are unavailable.
- **Evidence**: Unit tests (`TestSpawnCommand_EffortFlagForwarded`, `TestProjectSetConfig_ConfigJSON_PreservesEffort`) pass cleanly.
- **Revisit Trigger**: Upstream merges effort support into CLI mirror structs.

### D007 — Worker Concurrency Target Starts at 3, Not 7
- **Decision Status**: `ACCEPTED`
- **Evidence Maturity**: `LIVE_RUNTIME`
- **Decision**: Target worker range: 3–7; Initial runtime verification target: 3; Maximum proven live concurrency: 3 (`INV-005`).
- **Reason**: Live multi-agent concurrency on this stack is proven for 3 simultaneous workers. Claiming 7 workers without empirical proof violates runtime verification principles.
- **Evidence**: Live runtime concurrent execution verified in Gate 10 across 3 isolated git worktrees with 38.848216s simultaneous running overlap.
- **Revisit Trigger**: Revisit only upon capacity expansion under Phase 8 when workload demands require evaluating 5 to 7 concurrent workers.

### D008 — OpenCode and Agy as Dormant Fallbacks
- **Decision Status**: `ACCEPTED`
- **Evidence Maturity**: `STATIC`
- **Decision**: Maintain Candidate B (OpenCode) and Candidate C (Agy) as dormant fallbacks without active development.
- **Reason**: Upstream AO already includes native adapters for both. No custom adapter code is required.
- **Evidence**: Source inspection of `adapters/agent/opencode` and `adapters/agent/agy`.
- **Revisit Trigger**: Candidate A experiences an unresolved, reproducible failure root-cause isolated to Candidate A architectural boundary.

### D009 — Runtime Proof Outranks Static Inference
- **Decision Status**: `ACCEPTED`
- **Evidence Maturity**: `NOT_APPLICABLE`
- **Decision**: No component, candidate, or capability is labeled "PASS" without empirical runtime execution (`INV-004`).
- **Reason**: Prevents premature architectural commitments based on speculative code reading.
- **Evidence**: Core governance policy. Pre-auth audit correctly halted at `BLOCKED_RUNTIME_AUTH`.
- **Revisit Trigger**: Fundamental change in verification policy (not permitted).

### D010 — Product Repository Remains Strictly Read-Only
- **Decision Status**: `ACCEPTED`
- **Evidence Maturity**: `STATIC`
- **Decision**: Keep `AI-Auto-Video-Creator` untouched and read-only (`INV-001`).
- **Reason**: Respects repository boundary and milestone locks (M2-P1..P7B closed; P8/P9 locked; M3 unauthorized).
- **Evidence**: `git status` clean at commit `4a7c8c921b7e05066505d51b168a02c3fde61317`.
- **Revisit Trigger**: Explicit user authorization changing product milestone boundaries.
