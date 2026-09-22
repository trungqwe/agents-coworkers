# Documentation Control Plane

This directory (`/docs`) contains the canonical, authoritative technical backbone for the **agents-coworkers** multi-agent workforce integration.

---

## 1. Reading Order

New engineers and agents must read the documentation in this exact sequence:

1. [01-PROJECT-CHARTER.md](01-PROJECT-CHARTER.md) — Original intent, problem statement, engineering priorities, invariants, and non-goals.
2. [02-ARCHITECTURE.md](02-ARCHITECTURE.md) — Current technical design, component boundaries, Path A vs Path B, and dormant fallbacks.
3. [03-DECISIONS.md](03-DECISIONS.md) — Lightweight architectural decision ledger (D001–D010).
4. [04-ROADMAP.md](04-ROADMAP.md) — Single canonical roadmap (Phases 0–8) and the 10 runtime verification gates.
5. [05-VERIFICATION.md](05-VERIFICATION.md) — Evidence levels (L0–L7), proof requirements, and test creation governance.
6. [06-OPERATIONS.md](06-OPERATIONS.md) — Executable runbook for setup, daemon discovery, testing, and sequential gate execution.
7. [07-MAINTENANCE.md](07-MAINTENANCE.md) — Anti-drift governance contract, change sequence, stop conditions, and single-home rules.

---

## 2. Documentation Authority Model

| Question | Authoritative Document |
| :--- | :--- |
| **Why does this project exist?** | [01-PROJECT-CHARTER.md](01-PROJECT-CHARTER.md) |
| **What architecture are we using?** | [02-ARCHITECTURE.md](02-ARCHITECTURE.md) |
| **Why was it chosen?** | [03-DECISIONS.md](03-DECISIONS.md) |
| **What happens next?** | [04-ROADMAP.md](04-ROADMAP.md) |
| **What has actually been proven?** | [05-VERIFICATION.md](05-VERIFICATION.md) |
| **How do I run it?** | [06-OPERATIONS.md](06-OPERATIONS.md) |
| **How may the design change?** | [07-MAINTENANCE.md](07-MAINTENANCE.md) |

---

## 3. Scope & Precedence Boundary

`README.md`, `AUDIT.md`, and `DECISION.md` at the repository root serve as quick-start, point-in-time snapshot, and legacy compatibility summaries. **They must not independently redefine architecture, acceptance gates, roadmap status, or proof criteria when an authoritative `/docs` document exists.** In any case of divergence, files in `/docs` are the sole source of truth.