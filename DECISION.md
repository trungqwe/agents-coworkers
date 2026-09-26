# Architectural Decision Record: Candidate A

> [!IMPORTANT]
> **Authoritative Decision Ledger**: [docs/03-DECISIONS.md](docs/03-DECISIONS.md)
> This root document is a concise status summary. All architectural decisions, invariants, roadmap phases, and verification gates are authoritatively governed under [/docs](docs/README.md).

- **Selected Candidate**: **Candidate A** (Unified Codex Client Harness via CLIProxyAPI Gateway)
- **Architectural Status**: `SOURCE-FEASIBLE`
- **Runtime Decision**: `ACCEPTED` (All 10 Canonical Verification Gates verified at runtime; initial target of 3 live concurrent workers proven)
- **Active Roadmap Phase**: **Phase 7 — đang thực thi theo checkpoint**; Phase 8 vẫn `PARTIAL_EVIDENCE / OPTIONAL` (trạng thái authoritative tại [docs/04-ROADMAP.md](docs/04-ROADMAP.md)).

---

## Architecture Summary

- **Orchestrator**: role `orchestrator`; model/effort do user chọn theo từng run sau catalog, credential eligibility và session readback.
- **Workers**: role `worker`; model/effort do user chọn theo từng run với cùng preflight.
- **Historical profiles**: GPT-6 Astra/low, GPT-5.5/low và Gemini high là bằng chứng của các run cụ thể, không phải khóa role cố định. Lỗi một profile không suy thành toàn bộ provider/pool mất capacity; profile thay thế phải được ghi tường minh, không fallback âm thầm.
- **Target Worker Range**: Target worker range: 3–7; Initial runtime verification target: 3; Maximum proven live concurrency: 3 (empirically proven in Gate 10 with 3 concurrent workers, 38.848216s overlap)
- **Gateway**: CLIProxyAPI on `127.0.0.1:8317` managing ChatGPT Plus (6 accounts target) and Gemini Pro (8 accounts target) pools.

---

## Canonical Documentation References

- **Full Architectural Design & Boundaries**: [docs/02-ARCHITECTURE.md](docs/02-ARCHITECTURE.md)
- **Formal Decision Ledger (D001–D010)**: [docs/03-DECISIONS.md](docs/03-DECISIONS.md)
- **Sequential 10 Runtime Gates & Roadmap**: [docs/04-ROADMAP.md](docs/04-ROADMAP.md)
- **Evidence Hierarchy & Verification Rules**: [docs/05-VERIFICATION.md](docs/05-VERIFICATION.md)
- **Operational Runbook & Spawning Commands**: [docs/06-OPERATIONS.md](docs/06-OPERATIONS.md)
