# 02 — Current Technical Architecture

## 1. Canonical Architecture Diagram

```text
Agent Orchestrator
│
├── Orchestrator Session
│   ├── Role / Kind: orchestrator
│   ├── Agent Harness: codex
│   ├── Model: gpt-6-astra
│   └── Reasoning Effort: low
│
├── Worker Sessions (Target worker range: 3–7, initial runtime verification target: 3, maximum proven live concurrency: NOT YET PROVEN)
│   ├── Role / Kind: worker
│   ├── Agent Harness: codex
│   ├── Model: gemini-3.8-flash-high
│   └── Reasoning Effort: low
│
└── Worktree & Session Isolation
      │
      ▼
   Codex CLI (v0.154.0)
      │
      ▼ (Responses Wire API)
CLIProxyAPI Gateway (127.0.0.1:8317)
      │
      ├── ChatGPT Plus Pool (OAuth) ────> gpt-6-astra
      │
      └── Gemini Pro Pool (OAuth)   ────> gemini-3.8-flash-high
```

---

## 2. Component Boundaries & Responsibilities

| Component | Responsibility Domain | Non-Responsibilities |
| :--- | :--- | :--- |
| **Agent Orchestrator (AO)** | Projects, sessions, git worktree lifecycle, role configuration, process supervision, session isolation. | Does not implement LLM translation or provider OAuth token refresh. |
| **Codex CLI** | Local agent harness execution, tool loop (read, edit, execute), Responses API client behavior. | Does not manage multi-agent worktrees or project-level delegation. |
| **CLIProxyAPI** | Local proxy gateway (`127.0.0.1:8317`), model catalog `/v1/models`, Responses translation, credential pool management, round-robin routing, failover. | Does not execute git commands or manage file worktrees. |
| **Integration Repo (`agents-coworkers`)** | Locked upstream references, integration configs, regression tests, minimal compatibility patches, runbooks, technical backbone. | Does not host product source code. |
| **Product Repo (`AI-Auto-Video-Creator`)** | Production application codebase. Strictly **READ-ONLY** (`INV-001`). | Has no knowledge of AO or CLIProxyAPI integration details. |

---

## 3. Operational Paths: Zero-Patch vs Headless

### Path A: Zero Patch (Recommended / Default)
- **Status**: Default supported operational path (`INV-002`).
- **Mechanism**: Upstream AO checkout remains completely unmodified (`1140dd62dc7bb588b987e2c44aa1ff4796fa732b`).
- **Configuration**: Role models and reasoning efforts (`effort = "low"`) are configured via the **AO Desktop GUI** or direct daemon REST API (`PUT /api/v1/projects/<PROJECT_ID>/config`).
- **Spawning**: CLI `ao spawn` omits `--effort`. Sessions inherit role effort from daemon project configuration.
- **Verification**: Verified via read-back `GET /api/v1/projects/<PROJECT_ID>` asserting `projectResponse.project.config`.

### Path B: Headless CLI (Optional Fallback)
- **Status**: Optional fallback for headless CI/CD environments where GUI or REST tooling cannot be used.
- **Gaps Addressed**: Upstream unpatched `backend/internal/cli/project.go` mirror struct `agentConfig` lacks `Effort`, silently dropping effort when running `ao project set-config --config-json`. Unpatched `ao spawn` also lacks `--effort`.
- **Patch**: [0001-cli-support-agent-effort.patch](../patches/agent-orchestrator/0001-cli-support-agent-effort.patch) adds `Effort` to the CLI mirror struct and adds `--effort` to `ao spawn`.
- **Policy**: Must remain an optional patch outside the upstream checkout until runtime necessity is established.

---

## 4. Dormant Fallback Architectures

Should Candidate A encounter an unresolved, reproducible failure root-cause isolated to Candidate A architectural boundaries (such as persistent Responses tool loop translation failures), the following alternatives are evaluated in order:

1. **Candidate B — OpenCode Worker Fallback**:
   - Upstream AO already includes a native adapter: `backend/internal/adapters/agent/opencode/`.
   - Requires zero new adapter code in AO.
2. **Candidate C — Agy Worker Fallback**:
   - Upstream AO already includes a native adapter: `backend/internal/adapters/agent/agy/`.
   - Requires zero new adapter code in AO.

Both fallback adapters exist natively upstream. They remain strictly **dormant** unless Candidate A fails an explicit runtime acceptance gate.