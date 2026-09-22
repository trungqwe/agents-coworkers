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
| **Decision Ledger (D001–D010)** | [docs/03-DECISIONS.md](docs/03-DECISIONS.md) |
| **Canonical Roadmap & 10 Gates** | [docs/04-ROADMAP.md](docs/04-ROADMAP.md) |
| **Verification & Evidence Hierarchy** | [docs/05-VERIFICATION.md](docs/05-VERIFICATION.md) |
| **Operational Runbook** | [docs/06-OPERATIONS.md](docs/06-OPERATIONS.md) |
| **Maintenance & Anti-Drift Governance**| [docs/07-MAINTENANCE.md](docs/07-MAINTENANCE.md) |

---

## 2. Architecture Overview

```text
Agent Orchestrator
|
+-- Orchestrator:  gpt-6-astra (effort: low, harness: codex, kind: orchestrator)
+-- Workers:       gemini-3.8-flash-high (effort: low, harness: codex, kind: worker)
|                  Target worker range: 3–7 | Initial runtime verification target: 3 | Maximum proven live concurrency: NOT YET PROVEN
|
+--> CLIProxyAPI Gateway (127.0.0.1:8317)
       +-- 6x ChatGPT Plus accounts -> gpt-6-astra
       +-- 8x Gemini Pro accounts   -> gemini-3.8-flash-high
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

Candidate A remains `PROVISIONAL` until the canonical 10 runtime verification gates pass.

See:
- [Canonical Roadmap & Runtime Gates](docs/04-ROADMAP.md)
- [Verification Strategy & Evidence Hierarchy](docs/05-VERIFICATION.md)
