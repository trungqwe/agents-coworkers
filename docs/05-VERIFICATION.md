# 05 — Verification Strategy & Evidence Hierarchy

This document governs test definitions, evidence tiers, and test creation rules to prevent test bloat and maintain rigorous empirical standards.

---

## 1. Evidence Hierarchy Levels

| Level | Evidence Tier | Description | Typical Execution Target |
| :--- | :--- | :--- | :--- |
| **L0** | Source Inspection | Direct static audit of source files, ASTs, and route tables. | Controller definitions, CLI mirror structs |
| **L1** | Schema Validation | JSON schema, YAML parsing, and structural invariant checks. | Config templates, OpenAPI contracts |
| **L2** | Unit Tests | In-memory unit test suites with mock dependencies. | Effort preservation, CLI flag forwarding |
| **L3** | Integration Tests | Multi-package boundary validation without network I/O. | Schema regression tests (`tests/integration`) |
| **L4** | Local Process Smoke | Local process lifecycle, port binding, and loopback probes. | `smoke-cliproxy.ps1`, `smoke-codex` (negative) |
| **L5** | Live Provider Runtime | End-to-end execution against real authenticated endpoints. | Gates 1–6 (Catalog, Responses, Tool Loop) |
| **L6** | Real AO Session Runtime | Supervised execution inside real Agent Orchestrator sessions. | Gates 7–9 (Live orchestrator, worker, rework) |
| **L7** | Concurrent Workforce | Concurrent multi-agent execution across git worktrees. | Gate 10 (Parallel 3-worker wave) |

---

## 2. Invariant & Minimum Evidence Requirements

| Requirement / Invariant | Minimum Level | Current Level Achieved | Verification Method |
| :--- | :--- | :--- | :--- |
| `INV-001` Product Repo Read-Only | **L0** | **L0 (PASS)** | `git status` clean at commit `4a7c8c92` |
| `INV-002` AO Upstream Clean | **L0** | **L0 (PASS)** | `git status` clean at commit `1140dd62` |
| `INV-003` Unified Codex Harness | **L2** | **L2 (PASS)** | AO Codex adapter tests (`adapters/agent/codex`) |
| `INV-004` Runtime Proof Required | **L5** | **L6 (PASS: Gates 1–9)** | Gates 1–9 live runtime verified (catalog, responses, tool roundtrip, Codex->Astra, Codex->Gemini, AO orchestrator, AO worker, AO rework loop); Gate 10 pending |
| `INV-005` Concurrency Target Starts at 3 | **L7** | **L1 (STRUCTURAL)** | Target worker range: 3–7; initial runtime verification target: 3; maximum proven live concurrency: NOT YET PROVEN; worktree scaling primitives verified; live concurrency pending Gate 10 |
| `INV-006` Daemon Fail-Closed Discovery | **L1** | **L0 (DOCUMENTED / PENDING_RUNTIME)** | Fail closed if multiple AO daemons are valid; documented runbook logic reviewed; runtime verification pending AO runtime phase |
| `INV-007` Zero PII / Secret Leakage | **L2** | **L2 (PASS)** | Sanitized inventory mock PII assertion tests |
| AO Internal Effort Support | **L0/L2** | **L2 (PASS)** | Daemon store & service tests pass natively |
| Zero-Patch CLI Effort Loss | **L2** | **L2 (PASS)** | `TestZeroPatch_UnpatchedAgentConfigDropsEffort` |
| Patched CLI Effort Preservation | **L2** | **L2 (PASS)** | `TestHeadlessPatch_PatchedAgentConfigPreservesEffort` |
| CLIProxy Server Executable | **L4** | **L4 (PASS)** | Binds `127.0.0.1:8317`, handles `/v1/models` |
| Pre-Auth Negative Smoke Signature | **L4** | **L4 (PASS)** | Strictly requires `model_not_found` signature |

---

## 3. Test Creation Rule (Anti-Bloat Governance)

A permanent test or test script is permitted **only** if it satisfies at least one of the following criteria:
1. **Guards an observed regression**: Prevents recurrence of a confirmed bug (e.g. unpatched CLI dropping `effort`).
2. **Protects an architectural invariant**: Directly asserts one of `INV-001` through `INV-007`.
3. **Required for a roadmap exit criterion**: Serves as the definitive acceptance check for a Phase in [04-ROADMAP.md](04-ROADMAP.md).
4. **Protects security or privacy boundaries**: Validates secret sanitization or fail-closed network behavior.

### Prohibited Reasons for Adding Tests:
- Adding tests merely because an API or flag exists.
- Adding tests because an external auditor casually suggested "more coverage."
- Adding tests to satisfy hypothetical future features.

**Mandatory Pre-Test Check**: Before writing any new test, the author must explicitly state:
1. Which Requirement or Invariant ID does this test protect?
2. What specific failure does it prevent?
3. Which roadmap gate relies on this test?

If those three questions cannot be answered concretely, the test must **not** be committed.

---

## 4. Evidence Storage Policy
- **Historical Evidence**: Stored under `/evidence/run-<date>-<name>/` containing per-command raw logs, environment snapshots, and summary records. Evidence runs represent frozen historical captures and are never modified retrospectively.
- **Current Truth**: Authoritatively documented under `/docs/`. Do not convert every routine test run into a permanent documentation update unless system invariants or roadmap phase statuses have changed.
