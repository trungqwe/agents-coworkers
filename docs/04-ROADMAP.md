# 04 — Development Roadmap & Runtime Gates

This roadmap defines the single authoritative execution sequence for the multi-agent integration. Work proceeds strictly through ordered phases.

---

## 1. Roadmap Overview & Status Matrix

| Phase | Description | Status | Primary Gate / Deliverable |
| :--- | :--- | :--- | :--- |
| **Phase 0** | Source Feasibility | **COMPLETE** | AO adapters, CLIProxyAPI translators, schema verified |
| **Phase 1** | Pre-Auth Integration Foundation | **COMPLETE** | Configs, sanitized tools, discovery, regression tests |
| **Phase 2** | Minimal Authentication Proof | **COMPLETE** | Exactly 1 Codex + 1 Antigravity account logged in |
| **Phase 3** | Direct Provider Runtime Proof | **COMPLETE** | Gates 1, 2, 3, 4 (Catalog, Astra, Gemini, Tool Roundtrip) |
| **Phase 4** | Codex Harness Runtime Proof | **COMPLETE** | Gates 5, 6 (Codex -> Astra, Codex -> Gemini Tool Loop) |
| **Phase 5** | AO Runtime Proof | **NEXT** | Gates 7, 8, 9 (Real Orchestrator, Real Worker, Rework Loop) |
| **Phase 6** | 3-Worker Concurrency Proof | **BLOCKED** *(P5)* | Gate 10 (Real 3-Worker Concurrency Wave) |
| **Phase 7** | First Real Product Workload | **NOT AUTHORIZED** | Apply workforce to `AI-Auto-Video-Creator` tasks (requires explicit user authorization) |
| **Phase 8** | Capacity Expansion & Hardening | **DEFERRED / OPTIONAL** | Scale toward 6 Plus + 8 Pro; evaluate 5–7 workers when workload demands |

---

## 2. Canonical 10 Runtime Verification Gates

Candidate A transitions from `PROVISIONAL` to `ACCEPTED` only upon sequential verification of all 10 gates:

1. **Gate 1: CLIProxy Catalog**: `GET /v1/models` contains both `gpt-6-astra` and `gemini-3.8-flash-high`. (**VERIFIED - L5**)
2. **Gate 2: Direct Astra Responses**: Successful non-stream and stream response parsing via CLIProxyAPI gateway. (**VERIFIED - L5**)
3. **Gate 3: Direct Gemini Responses**: Successful non-stream and stream response parsing via CLIProxyAPI gateway. (**VERIFIED - L5**)
4. **Gate 4: Gemini Tool Roundtrip**: Function call translation, execution, and response synthesis fidelity verified. (**VERIFIED - L5**)
5. **Gate 5: Codex -> Astra**: Successful `codex exec` invocation through CLIProxyAPI targeting `gpt-6-astra`. (**VERIFIED - L5**)
6. **Gate 6: Codex -> Gemini Tool Loop**: Successful multi-turn coding and file editing loop via `codex exec` targeting `gemini-3.8-flash-high`. (**VERIFIED - L5**)
7. **Gate 7: Real AO Orchestrator**: Live session launched on Agent Orchestrator with `kind = "orchestrator"`. (**PENDING - P5**)
8. **Gate 8: Real AO Worker**: Live session launched on Agent Orchestrator with `kind = "worker"` using Gemini. (**PENDING - P5**)
9. **Gate 9: AO Rework Loop**: Orchestrator reviews worker worktree output and successfully issues a rework directive. (**PENDING - P5**)
10. **Gate 10: Real 3-Worker Wave**: Three concurrent live worker sessions complete isolated tasks without worktree, locking, or routing failures (verifying initial runtime verification target of 3 workers; maximum proven live concurrency becomes 3 only after this gate passes). (**PENDING - P6**)

---

## 3. Phase Details & Exit Criteria

### Phase 2: Minimal Authentication Proof (COMPLETE)
- **Objective**: Authenticate the minimum viable credential set (1 ChatGPT Plus + 1 Gemini Pro) to enable runtime testing.
- **Entry Criteria**: Phase 1 complete; sanitized inventory and smoke scripts operational.
- **Required Work**:
  1. Interactive login of exactly **one** ChatGPT Plus account (`-codex-login`).
  2. Interactive login of exactly **one** Gemini Pro Google account (`-antigravity-login`).
  3. Verify with `scripts/auth-inventory.ps1` that counts reach Codex: 1, Antigravity: 1.
- **Exit Criteria**: Sanitized inventory confirms 1 Codex and 1 Antigravity credential present.
- **Evidence Required**: Sanitized inventory output (hashes only; zero emails or raw filenames).
- **Explicit Non-Goals**: Do NOT log in all 14 accounts in this phase. Test with minimal credentials first.

### Phase 3: Direct Provider Runtime Proof (COMPLETE)
- **Objective**: Verify raw HTTP API translation and tool roundtrips through CLIProxyAPI gateway.
- **Entry Criteria**: Phase 2 complete (1+1 accounts loaded).
- **Required Work**: Execute Gates 1, 2, 3, and 4.
- **Exit Criteria**: Both models respond to `/v1/models` and direct Responses requests; tool call roundtrip passes.
- **Evidence Required**: HTTP responses and tool roundtrip test output recorded in `/evidence`.
- **Explicit Non-Goals**: Do not invoke Codex CLI or Agent Orchestrator yet.

### Phase 4: Codex Harness Runtime Proof (COMPLETE)
- **Objective**: Verify that Codex CLI interacts correctly with both models via CLIProxyAPI wire translation.
- **Entry Criteria**: Phase 3 complete.
- **Required Work**: Execute Gates 5 and 6 using isolated `CODEX_HOME`.
- **Exit Criteria**: `codex exec` succeeds on `gpt-6-astra` and performs multi-turn file edits on `gemini-3.8-flash-high`.
- **Evidence Required**: Execution transcripts with sanitized outputs.
- **Explicit Non-Goals**: Do not launch AO sessions.

### Phase 5: AO Runtime Proof (NEXT ACTION)
- **Objective**: Verify that Agent Orchestrator successfully provisions and supervises single sessions.
- **Entry Criteria**: Phase 4 complete.
- **Required Work**: Execute Gates 7, 8, and 9 on a live AO daemon.
- **Exit Criteria**: Orchestrator session runs; worker session runs; orchestrator reviews worker worktree and directs rework.
- **Evidence Required**: AO daemon session logs and git worktree commit logs.
- **Explicit Non-Goals**: Do not run concurrent workers yet.

### Phase 6: 3-Worker Concurrency Proof
- **Objective**: Prove concurrent multi-agent execution at the initial runtime verification target (3 workers). Target worker range is 3–7; maximum proven live concurrency remains NOT YET PROVEN until Gate 10 passes.
- **Entry Criteria**: Phase 5 complete.
- **Required Work**: Execute Gate 10 (parallel 3-worker wave).
- **Exit Criteria**: 3 live workers execute in parallel across separate git worktrees without race conditions, token starvation, or session crashes.
- **Evidence Required**: Parallel session execution logs and worktree integrity verification.
- **Explicit Non-Goals**: Do not claim 5 or 7 workers until Phase 8 (Capacity Expansion).

### Phase 7: First Real Product Workload (NOT AUTHORIZED)
- **Objective**: Deploy the multi-agent workforce to implementation tasks in `AI-Auto-Video-Creator`.
- **Entry Criteria**: Phase 6 complete (Gate 10 passed) AND explicit user authorization changing product milestone boundaries (`INV-001`). Minimal credentials sufficient for runtime gates (Phase 2 onward) already satisfy the auth requirement; full pool expansion is NOT a prerequisite for starting product workload.
- **Required Work**: Register `AI-Auto-Video-Creator` as an AO project (`ao project add --path`), configure roles, and execute at least one orchestrator-directed worker task producing a verified commit.
- **Exit Criteria**: At least one complete orchestrator→worker→review cycle produces a merged or reviewable commit in the product repository.
- **Evidence Required**: AO session logs, worktree diff, commit hash in product repo.
- **Status**: Strictly **NOT AUTHORIZED**. `INV-001` remains in effect; P8/P9 `LOCKED`; M3 / Module A `NOT AUTHORIZED`.

### Phase 8: Capacity Expansion & Hardening (DEFERRED / OPTIONAL)
- **Objective**: Scale account pool toward full target (6 Plus + 8 Pro) and evaluate higher worker concurrency (5–7 workers) when workload demands it.
- **Entry Criteria**: Phase 7 complete or in-progress AND runtime evidence shows current credential/worker count is insufficient for workload throughput.
- **Required Work**: Add credentials through the interactive login procedure in [06-OPERATIONS.md](06-OPERATIONS.md) when runtime evidence demonstrates that current capacity is insufficient; verify counts after each login with `scripts/auth-inventory.ps1`. To prove higher concurrency (5 or 7 workers), run real AO worker sessions in parallel across separate worktrees—not `scripts/stress-workers.ps1`, which tests only git worktree primitives.
- **Exit Criteria**: Prove the capacity level actually deployed: record the maximum number of live concurrent AO worker sessions that completed successfully. Reaching full 6+8 pool or 5–7 workers is not required if the workload does not demand it.
- **Evidence Required**: AO session logs for each concurrent worker wave, worktree integrity verification, and auth inventory snapshot confirming credential counts at time of test.
- **Status**: **DEFERRED / OPTIONAL**. Full 6+8 pool is NOT a prerequisite for Candidate A acceptance or for starting product workload.