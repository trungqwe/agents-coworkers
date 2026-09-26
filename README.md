# Agents-Coworkers: AO + CLIProxyAPI Multi-Agent Integration

Integration templates, patches, tests, and audit evidence for operating a multi-agent development workforce with **Agent Orchestrator (AO)**, **CLIProxyAPI**, and **Codex CLI**.

> [!IMPORTANT]
> **Authoritative Technical Backbone**: All formal architecture, invariants, decisions, roadmap phases, and runbooks are maintained in the [/docs](docs/README.md) directory. This root README serves as a quickstart and navigation guide.

---

## 1. Documentation Index

| Topic | Authoritative Document |
| :--- | :--- |
| **Project Charter & Invariants** | [docs/01-PROJECT-CHARTER.md](docs/01-PROJECT-CHARTER.md) |
| **Technical Architecture** | [docs/02-ARCHITECTURE.md](docs/02-ARCHITECTURE.md) |
| **Decision Ledger (D001–D015)** | [docs/03-DECISIONS.md](docs/03-DECISIONS.md) |
| **Canonical Roadmap & 10 Gates** | [docs/04-ROADMAP.md](docs/04-ROADMAP.md) |
| **Verification & Evidence Hierarchy** | [docs/05-VERIFICATION.md](docs/05-VERIFICATION.md) |
| **Operational Runbook** | [docs/06-OPERATIONS.md](docs/06-OPERATIONS.md) |
| **Maintenance & Anti-Drift Governance**| [docs/07-MAINTENANCE.md](docs/07-MAINTENANCE.md) |

---

## 2. Architecture Overview

```text
Agent Orchestrator
|
+-- Orchestrator:  model/effort do user chọn theo từng run (harness: codex, kind: orchestrator)
+-- Workers:       model/effort do user chọn theo từng run (harness: codex, kind: worker)
|                  Concurrency: `maximum proven live concurrency = 3` | Scheduler mặc định: 1–3 workers | 4–7: EXPERIMENTAL_CAPACITY (cần authority riêng)
|
+--> CLIProxyAPI Gateway (127.0.0.1:8317)
       +-- Credential pool được lọc theo catalog, eligibility và profile của run
       +-- Không fallback âm thầm giữa model/provider
```

---

## 3. Upstream Repository Alignment

| Repository | Reference SHA | Local HEAD | Status |
| :--- | :--- | :--- | :--- |
| [agent-orchestrator](https://github.com/Untrivial-ai/agent-orchestrator) | `1140dd62dc7bb588b987e2c44aa1ff4796fa732b` | `1140dd62dc7bb588b987e2c44aa1ff4796fa732b` | Clean Match (`INV-002`) |
| [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) | `2430354330af80b645f9ffb1a51e1e7c72c4cc8e` | `2430354330af80b645f9ffb1a51e1e7c72c4cc8e` | Exact Match |
| [AI-Auto-Video-Creator](https://github.com/trungqwe/AI-Auto-Video-Creator) | `4a7c8c921b7e05066505d51b168a02c3fde61317` | `4a7c8c921b7e05066505d51b168a02c3fde61317` | Read-Only Match (`INV-001`) |

- Built CLIProxyAPI binary: `D:\TU_CODE\agent-orchestrator\CLIProxyAPI\cli-proxy-api.exe`
- Binary SHA-256: `E9DA39B2491856BE2D3711E20BE89690F3465E2A5B7A469BA1A29AFF7AB342D2`

---

## 4. Quickstart Guide

For complete step-by-step procedures, consult [docs/06-OPERATIONS.md](docs/06-OPERATIONS.md).

### Preflight & Config Setup:
```powershell
powershell -ExecutionPolicy Bypass -File scripts\preflight.ps1
Copy-Item "config\cliproxy\config.example.yaml" "config\cliproxy\config.runtime.yaml"
$env:CLIPROXY_KEY = "<YOUR_PRIVATE_KEY>"
```

### Interactive Login (Phase 2 Minimal: 1 Codex + 1 Antigravity):
```powershell
cd D:\TU_CODE\agent-orchestrator\CLIProxyAPI
.\cli-proxy-api.exe -config D:\TU_CODE\agents-coworkers\config\cliproxy\config.runtime.yaml -codex-login
.\cli-proxy-api.exe -config D:\TU_CODE\agents-coworkers\config\cliproxy\config.runtime.yaml -antigravity-login
powershell -ExecutionPolicy Bypass -File D:\TU_CODE\agents-coworkers\scripts\auth-inventory.ps1
```

### Gateway & Smoke Testing:
```powershell
# Launch gateway in dedicated terminal:
.\cli-proxy-api.exe -config D:\TU_CODE\agents-coworkers\config\cliproxy\config.runtime.yaml

# Run smoke tests:
powershell -ExecutionPolicy Bypass -File D:\TU_CODE\agents-coworkers\scripts\smoke-cliproxy.ps1
powershell -ExecutionPolicy Bypass -File D:\TU_CODE\agents-coworkers\scripts\smoke-codex-through-proxy.ps1
```

### AO Daemon Dynamic Discovery & Path A Configuration:
See [docs/06-OPERATIONS.md](docs/06-OPERATIONS.md) for complete dynamic daemon discovery and PUT configuration snippet.

---

## 5. Runtime Acceptance

Candidate A is **ACCEPTED** following sequential runtime verification of all 10 canonical gates (Gates 1–10). `maximum proven live concurrency = 3` (Phase 6 COMPLETE). Scheduler mặc định: 1–3 workers; mức 4–7 là EXPERIMENTAL_CAPACITY cần authority và capacity proof riêng. Mục tiêu danh nghĩa ban đầu (6 Plus + 8 Pro) chỉ là mục tiêu lịch sử đã superseded; pool hiện tại là historical inventory và việc mở rộng phụ thuộc dữ liệu telemetry thực tế. Phase 7 đạt **COMPLETE_WITH_ACCEPTED_LIMITATIONS**; Phase 8 là active roadmap phase tiếp theo ở trạng thái **ACTIVE_IMPLEMENTATION**: Slice 8A đã hoàn thành và nghiệm thu tại Gate 8A (**VERIFIED — L4**; `cmd/coworkers` implemented cho doctor/attach/status; `coworkers run` vẫn UNSUPPORTED, exit 1); Slice 8B ở trạng thái **NEXT / IMPLEMENTATION_NOT_AUTHORIZED**; Phase 8 chưa COMPLETE và không tuyên bố production-ready. Mỗi run chỉ dùng profile sau khi catalog, credential eligibility và AO session readback đều khớp.

See:
- [Canonical Roadmap & Runtime Gates](docs/04-ROADMAP.md)
- [Verification Strategy & Evidence Hierarchy](docs/05-VERIFICATION.md)
