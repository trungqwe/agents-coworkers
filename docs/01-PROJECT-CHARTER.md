# 01 — Project Charter & Boundaries

## 1. Original Project Intent

The primary objective of this integration is to establish a practical, reliable, multi-agent software-development workforce (control plane) around **Agent Orchestrator (AO)**, utilizing **CLIProxyAPI** as a local routing gateway and **Codex CLI** as a unified agent client harness.

This repository (`agents-coworkers`) is the reusable workforce/control plane: it contains integration configs, patches, runbooks, and verification infrastructure that are independent of any single product codebase. **[AI-Auto-Video-Creator](https://github.com/trungqwe/AI-Auto-Video-Creator)** is the first intended product workload (currently `READ-ONLY` per `INV-001`; workload deployment requires explicit user authorization).

- **Orchestrator Role**: Plans, reviews, audits, and issues directives using `gpt-6-astra` (reasoning effort: `low`, harness: `codex`, kind: `orchestrator`, mode: `chat`).
- **Worker Roles**: Concurrently implement tasks in isolated git worktrees using `gemini-3.8-flash-high` (reasoning effort: `low`, harness: `codex`, kind: `worker`, mode: `chat`).
- **Target Scale**: Target worker range: 3–7; Initial runtime verification target: 3; Maximum proven live concurrency: 3 (empirically proven in Gate 10).
- **Core Value Proposition**: Minimize moving parts, eliminate manual coordination overhead, isolate code changes via native git worktrees, prevent merge collisions, and optimize token/context cost.

---

## 2. Engineering Priorities (Strictly Ordered)

1. **Runtime Correctness**: Real end-to-end tool execution and response parsing must work before any optimization.
2. **Minimum Moving Parts**: Prefer a single unified client harness (Codex) over juggling multiple disparate agent CLI adapters.
3. **Worktree & Session Isolation**: Native git worktree separation must prevent filesystem lock collisions and branch corruption.
4. **Clear Role Separation**: The orchestrator reviews and directs; workers execute focused implementation units.
5. **Fail-Closed Verification**: Scripts and integrations must fail closed on missing models, unauthenticated catalogs, or API errors.
6. **Token & Context Efficiency**: Keep prompt contexts compact and utilize low reasoning effort for high-frequency operational iterations.
7. **Maintainability**: Prefer zero-patch upstream operation; keep integration artifacts modular, traceable, and small.
8. **Scale Only After Empirical Proof**: Never assume 7 concurrent workers or 14 accounts function before verifying 1 worker and 2 accounts.

---

## 3. System Invariants

The following invariants must be preserved across all implementations, tests, and documentation:

- `INV-001` — **Product Repo Read-Only**: `AI-Auto-Video-Creator` at commit `4a7c8c921b7e05066505d51b168a02c3fde61317` is strictly READ-ONLY. Status: M2-P1..P7B `ACCEPTED/CLOSED`; P8/P9 `LOCKED`; M3 / Module A `NOT AUTHORIZED`. Zero files modified.
- `INV-002` — **AO Upstream Clean by Default**: Upstream `agent-orchestrator` (`1140dd62dc7bb588b987e2c44aa1ff4796fa732b`) remains unmodified in default operation (Path A).
- `INV-003` — **Unified Codex Client Harness**: Both orchestrator and worker roles use `agent = "codex"` talking to CLIProxyAPI via Responses-compatible API.
- `INV-004` — **Runtime Proof Outranks Static Inference**: No gate, candidate, or capability is marked verified based on code reading or static tests alone.
- `INV-005` — **Concurrency Target Starts at 3**: Target worker range: 3–7; Initial runtime verification target: 3; Maximum proven live concurrency: 3 (empirically proven in Gate 10 across isolated worktrees).
- `INV-006` — **Fail-Closed Daemon Discovery**: If multiple live AO daemons exist, scripts fail closed instead of guessing.
- `INV-007` — **Zero PII & Secret Leakage**: No tokens, private keys, passwords, user emails, raw account filenames, or OS usernames are printed or committed.

---

## 4. Explicit Non-Goals

- **Not** building a new orchestration framework or rewriting Agent Orchestrator.
- **Not** maintaining a permanent fork of Agent Orchestrator unless upstream permanently refuses essential fixes.
- **Not** modifying upstream AO source merely for syntactic CLI convenience when GUI or REST APIs suffice.
- **Not** designing or benchmarking hypothetical 7-worker scaling before a 3-worker wave is empirically proven.
- **Not** supporting every available LLM provider or harness in this repository.
- **Not** building an expansive general-purpose test framework beyond targeted integration verification.
- **Not** modifying product source in `AI-Auto-Video-Creator` until explicit user authorization changes its milestone boundaries. The workforce is designed to serve this workload once authorized, not after full pool expansion.
- **Not** using multiple credentials to evade provider terms of service, quotas, or rate limits. Accounts are used solely for authorized availability and distribution within provider terms.

---

## 5. The Anti-Drift Test

Before introducing any new feature, patch, test framework, provider adapter, abstraction layer, or documentation domain, every engineer or agent must evaluate:

> **Does this directly advance one of the active roadmap gates defined in [04-ROADMAP.md](04-ROADMAP.md)?**
> - If **NO**: Do NOT add it.
> - If **MAYBE**: Record it as deferred in [03-DECISIONS.md](03-DECISIONS.md), not current scope.
> - If **YES**: Explicitly identify which roadmap gate requires it.