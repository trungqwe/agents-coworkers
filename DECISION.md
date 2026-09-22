# Architectural Decision Record: Candidate A

> [!IMPORTANT]
> **Authoritative Decision Ledger**: [docs/03-DECISIONS.md](docs/03-DECISIONS.md)
> This root document is a concise status summary. All architectural decisions, invariants, roadmap phases, and verification gates are authoritatively governed under [/docs](docs/README.md).

- **Selected Candidate**: **Candidate A** (Unified Codex Client Harness via CLIProxyAPI Gateway)
- **Architectural Status**: `SOURCE-FEASIBLE`
- **Runtime Decision**: `PROVISIONALLY SELECTED` (Pending live runtime gates)
- **Active Roadmap Phase**: Phase 2 — Minimal Authentication Proof (see [docs/04-ROADMAP.md](docs/04-ROADMAP.md))

---

## Architecture Summary

- **Orchestrator**: `gpt-6-astra` (effort: `low`, harness: `codex`, kind: `orchestrator`, mode: `chat`)
- **Workers**: `gemini-3.8-flash-high` (effort: `low`, harness: `codex`, kind: `worker`, mode: `chat`)
- **Target Worker Range**: 3–7 parallel workers (Initial verified target after auth: 3; maximum live concurrency: TBD from runtime evidence)
- **Gateway**: CLIProxyAPI on `127.0.0.1:8317` managing ChatGPT Plus (6 accounts target) and Gemini Pro (8 accounts target) pools.

---

## Canonical Documentation References

- **Full Architectural Design & Boundaries**: [docs/02-ARCHITECTURE.md](docs/02-ARCHITECTURE.md)
- **Formal Decision Ledger (D001–D010)**: [docs/03-DECISIONS.md](docs/03-DECISIONS.md)
- **Sequential 10 Runtime Gates & Roadmap**: [docs/04-ROADMAP.md](docs/04-ROADMAP.md)
- **Evidence Hierarchy & Verification Rules**: [docs/05-VERIFICATION.md](docs/05-VERIFICATION.md)
- **Operational Runbook & Spawning Commands**: [docs/06-OPERATIONS.md](docs/06-OPERATIONS.md)
