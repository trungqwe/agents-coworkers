# Architectural Decision Record: Candidate A

> [!IMPORTANT]
> **Authoritative Decision Ledger**: [docs/03-DECISIONS.md](docs/03-DECISIONS.md)
> This root document is a concise status summary. All architectural decisions, invariants, roadmap phases, and verification gates are authoritatively governed under [/docs](docs/README.md).

- **Selected Candidate**: **Candidate A** (Unified Codex Client Harness via CLIProxyAPI Gateway)
- **Architectural Status**: `SOURCE-FEASIBLE`
- **Runtime Decision**: `ACCEPTED` (All 10 Canonical Verification Gates verified at runtime; initial target of 3 live concurrent workers proven)
- **Active Roadmap Phase**: **Phase 8 — Productization Roadmap & Design Governance (DESIGN_PHASE / IMPLEMENTATION_NOT_AUTHORIZED)**; Phase 7 đạt **COMPLETE_WITH_ACCEPTED_LIMITATIONS** (trạng thái authoritative tại [docs/04-ROADMAP.md](docs/04-ROADMAP.md)).

---

## Architecture Summary

- **Orchestrator**: role `orchestrator`; model/effort do user chọn theo từng run sau catalog, credential eligibility và session readback.
- **Workers**: role `worker`; model/effort do user chọn theo từng run với cùng preflight.
- **Historical profiles**: GPT-6 Astra/low, GPT-5.5/low và Gemini high là bằng chứng của các run cụ thể, không phải khóa role cố định. Lỗi một profile không suy thành toàn bộ provider/pool mất capacity; profile thay thế phải được ghi tường minh, không fallback âm thầm.
- **Worker Concurrency**: `maximum proven live concurrency = 3` (empirically proven in Gate 10 with 3 concurrent workers, 38.848216s overlap). Scheduler mặc định chọn từ 1–3 workers; dải 4–7 workers là `EXPERIMENTAL_CAPACITY`, đòi hỏi authority và capacity proof riêng.
- **Gateway & Pool**: CLIProxyAPI trên loopback (mặc định lịch sử: `http://127.0.0.1:8317`). Mục tiêu danh nghĩa ban đầu (6 Plus + 8 Pro) chỉ là mục tiêu lịch sử đã superseded; pool hiện tại (1 Codex + 7 Gemini) được ghi nhận là historical inventory và việc mở rộng dung lượng phụ thuộc vào dữ liệu telemetry thực tế (Slice 8E).

---

## Canonical Documentation References

- **Full Architectural Design & Boundaries**: [docs/02-ARCHITECTURE.md](docs/02-ARCHITECTURE.md)
- **Formal Decision Ledger (D001–D015)**: [docs/03-DECISIONS.md](docs/03-DECISIONS.md)
- **Sequential 10 Runtime Gates & Roadmap**: [docs/04-ROADMAP.md](docs/04-ROADMAP.md)
- **Evidence Hierarchy & Verification Rules**: [docs/05-VERIFICATION.md](docs/05-VERIFICATION.md)
- **Operational Runbook & Spawning Commands**: [docs/06-OPERATIONS.md](docs/06-OPERATIONS.md)
