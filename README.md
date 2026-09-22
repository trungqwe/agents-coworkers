# Agents Coworkers: Multi-Agent Orchestration & Integration

Integration, configuration templates, audit evidence, and verification scripts for orchestrating multi-agent systems using **Agent Orchestrator (AO)** and **CLIProxyAPI**.

## Core Architecture

```text
GPT Orchestrator (gpt-6-astra, effort=low)
  ├── Agent Orchestrator Supervisor (Session & Worktree Manager)
  ├── 3-7 Parallel Gemini Workers (gemini-3.8-flash-high)
  └── CLIProxyAPI Local Gateway (127.0.0.1:8317)
        ├── 6x ChatGPT Plus Accounts Pool (Codex OAuth)
        └── 8x Gemini Pro Accounts Pool (Antigravity OAuth)
```

## Key Documents

- [`AUDIT.md`](AUDIT.md) — Comprehensive audit report separating source-proven, runtime-proven, and blocked gates.
- [`DECISION.md`](DECISION.md) — Authoritative architecture decision record with exact startup sequences.
- [`upstream-lock.json`](upstream-lock.json) — Exact commit SHAs and divergence analysis across all tracked repositories.

## Directory Structure

```text
agents-coworkers/
├── README.md
├── AUDIT.md
├── DECISION.md
├── upstream-lock.json
├── .gitignore
├── config/
│   ├── cliproxy/
│   │   └── config.example.yaml
│   ├── ao/
│   │   └── project-config.example.json
│   └── codex/
│       └── config.example.toml
├── scripts/
│   ├── preflight.ps1
│   ├── smoke-cliproxy.ps1
│   ├── smoke-codex-through-proxy.ps1
│   ├── smoke-ao.ps1
│   ├── stress-workers.ps1
│   └── redact-evidence.ps1
├── patches/
│   └── agent-orchestrator/
│       ├── 0001-cli-support-agent-effort.patch
│       └── README.md
└── evidence/
    └── run-20260923-audit-ao-cliproxy/
        ├── environment.json
        ├── commands.jsonl
        ├── test-matrix.json
        ├── summary.md
        └── hashes.sha256
```

## Verification Scripts

Run from PowerShell:
```powershell
# 1. Environment & Revision Preflight
.\scripts\preflight.ps1

# 2. AO Multi-Worker Worktree Isolation
.\scripts\smoke-ao.ps1

# 3. Concurrency Waves Stress Test (1, 3, 5, 7 workers)
.\scripts\stress-workers.ps1

# 4. Redaction Scanner
.\scripts\redact-evidence.ps1
```
