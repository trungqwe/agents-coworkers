# 05 — Verification Strategy & Evidence Hierarchy

This document governs test definitions, evidence tiers, and test creation rules to prevent test bloat and maintain rigorous empirical standards.

---

## 1. Evidence Hierarchy Levels

| Level | Evidence Tier | Description | Typical Execution Target |
| :--- | :--- | :--- | :--- |
| **L0** | Source Inspection | Direct static audit of source files, ASTs, and route tables. | Controller definitions, CLI mirror structs |
| **L1** | Schema Validation | JSON schema, YAML parsing, and structural invariant checks. | Config templates, OpenAPI contracts |
| **L2** | Unit Tests | In-memory unit test suites with mock dependencies. | Effort preservation, CLI flag forwarding |
| **L3** | Integration Tests | Multi-package boundary validation without network I/O. | Schema regression tests (`tests/integration`) |
| **L4** | Local Process Smoke | Local process lifecycle, port binding, and loopback probes. | `smoke-cliproxy.ps1`, `smoke-codex` (negative) |
| **L5** | Live Provider Runtime | End-to-end execution against real authenticated endpoints. | Gates 1–6 (Catalog, Responses, Tool Loop) |
| **L6** | Real AO Session Runtime | Supervised execution inside real Agent Orchestrator sessions. | Gates 7–9 (Live orchestrator, worker, rework) |
| **L7** | Concurrent Workforce | Concurrent multi-agent execution across git worktrees. | Gate 10 (Parallel 3-worker wave) |

---

## 2. Invariant & Minimum Evidence Requirements (ảnh chụp component baseline)

| Requirement / Invariant | Minimum Level | Current Level Achieved | Verification Method |
| :--- | :--- | :--- | :--- |
| `INV-001` Product Repo Read-Only | **L0** | **L0 (PASS)** | `git status` clean at commit `4a7c8c92` |
| `INV-002` AO Upstream Clean | **L0** | **L0 (PASS)** | `git status` clean at commit `1140dd62` |
| `INV-003` Unified Codex Harness | **L2** | **L2 (PASS)** | AO Codex adapter tests (`adapters/agent/codex`) |
| `INV-004` Runtime Proof Required | **L5** | **L7 (PASS: Gates 1–10)** | Gates 1–10 live runtime verified (catalog, responses, tool roundtrip, Codex->Astra, Codex->Gemini, AO orchestrator, AO worker, AO rework loop, 3-worker concurrency wave) |
| `INV-005` Concurrency Target Starts at 3 | **L7** | **L7 (PASS)** | Target worker range: 3–7; initial runtime verification target: 3; maximum proven live concurrency: 3 (Gate 10 verified live concurrent execution with 38.848216s simultaneous overlap from max(started_at) to min(completed_at)) |
| `INV-006` Daemon Fail-Closed Discovery | **L1** | **L0 (DOCUMENTED / PENDING_RUNTIME)** | Fail closed if multiple AO daemons are valid; documented runbook logic reviewed; runtime verification pending AO runtime phase |
| `INV-007` Zero PII / Secret Leakage | **L2** | **L2 (PASS)** | Sanitized inventory mock PII assertion tests |
| AO Internal Effort Support | **L0/L2** | **L2 (PASS)** | Daemon store & service tests pass natively |
| Zero-Patch CLI Effort Loss | **L2** | **L2 (PASS)** | `TestZeroPatch_UnpatchedAgentConfigDropsEffort` |
| Patched CLI Effort Preservation | **L2** | **L2 (PASS)** | `TestHeadlessPatch_PatchedAgentConfigPreservesEffort` |
| CLIProxy Server Executable | **L4** | **L4 (PASS)** | Binds `127.0.0.1:8317`, handles `/v1/models` |
| Pre-Auth Negative Smoke Signature | **L4** | **L4 (PASS)** | Strictly requires `model_not_found` signature |

---

## 3. Test Creation Rule (Anti-Bloat Governance)

A permanent test or test script is permitted **only** if it satisfies at least one of the following criteria:
1. **Guards an observed regression**: Prevents recurrence of a confirmed bug (e.g. unpatched CLI dropping `effort`).
2. **Protects an architectural invariant**: Directly asserts one of `INV-001` through `INV-007`.
3. **Required for a roadmap exit criterion**: Serves as the definitive acceptance check for a Phase in [04-ROADMAP.md](04-ROADMAP.md).
4. **Protects security or privacy boundaries**: Validates secret sanitization or fail-closed network behavior.

### Prohibited Reasons for Adding Tests:
- Adding tests merely because an API or flag exists.
- Adding tests because an external auditor casually suggested "more coverage."
- Adding tests to satisfy hypothetical future features.

**Mandatory Pre-Test Check**: Before writing any new test, the author must explicitly state:
1. Which Requirement or Invariant ID does this test protect?
2. What specific failure does it prevent?
3. Which roadmap gate relies on this test?

If those three questions cannot be answered concretely, the test must **not** be committed.

---

## 4. Evidence storage và giới hạn claim

Evidence lịch sử ở `evidence/run-<date>-<name>/` bất biến; correction ghi trong governance hoặc run tiếp nối, không sửa capture cũ. Chỉ lưu metadata/log đã redaction, command/exit code, timestamp, source/artifact hash và provenance. Không thu raw request/auth/environment/cookie hoặc toàn error object. Hash khớp chứng minh integrity, không thay việc chạy test hay review code.

Gates 1–10 và bảng §2 là component baseline, không bị chạy lại chỉ vì đổi docs. INV-001 hiện bảo vệ root, không cấm ngoại lệ worktree đã duyệt; kiểm cleanliness phải gắn thời điểm và baseline, không khẳng định mọi upstream worktree hiện sạch từ evidence cũ.

## 5. Acceptance matrix của workforce

| Capability | Oracle cần đạt | Evidence hiện có | Trạng thái / giới hạn |
|---|---|---|---|
| Harness/role/tool-loop | AO → Codex → provider đọc/sửa/tool round-trip | [Gates 1–6](../evidence/run-20260923-phase2-4-gates1-6/summary.md), [7–9](../evidence/run-20260923-phase5-gates7-9/summary.md) | Component VERIFIED; không là autonomous loop |
| Profile worker mục tiêu | Per-session model/effort readback và tool-loop | [worker-high-profile](../evidence/run-20260923-worker-high-profile/summary.md) | Readback/tool-loop đạt; upstream thinkingLevel NOT OBSERVED, không nâng Gate 10 |
| Parallel ownership | Worker thực sự overlap, file/resource không ghi chéo | [Gate 10](../evidence/run-20260923-phase6-gate10/summary.md); [pilot 7B–7C](../evidence/run-20260924-phase7bc-integration-continuation/summary.md) | Gate 10: 3 worker low; continuation: A/B Gemini high code độc lập có overlap, C chạy sau ACCEPT trên worktree riêng. Chưa proof 3-worker high code wave |
| Autonomous delegation | Orchestrator lập task graph, gửi việc/nhận artifact qua AO, không human relay | [Pilot 7A](../evidence/run-20260924-phase7a-native-pilot/summary.md): GPT-6-Astra/low review-only; [7B–7C](../evidence/run-20260924-phase7bc-integration-continuation/summary.md): GPT-5.5/low điều phối A/B/C | OBSERVED cho fixture code/review/integration; không chứng minh rework thực tế, daemon restart recovery hoặc portability |
| Review/rework | Orchestrator chấp nhận hoặc yêu cầu sửa từ diff/test; worker rework và review lại nếu có lỗi thật | Continuation 7B–7C, verdict native AO message | A/B/C `ACCEPT` quan sát được; không có lỗi cần sửa nên rework thực tế `NOT OBSERVED` |
| Integration/dependency | Chỉ ghép task khi dependency/review đạt, một integration owner, test chung | Continuation: C được tạo sau A/B ACCEPT, cherry-pick A rồi B, combined unittest 8 tests | OBSERVED trên fixture; không push/merge và không phải integration Product |
| Routing/affinity | Giữ binding khi eligible; quan sát selected IDs theo session/model | [pool onboarding](../evidence/run-20260923-phase8-pool-onboarding/summary.md) | LIVE_ROUTING 14/14, 5 IDs; không khẳng định cả pool usable hoặc phân phối đều |
| Failover/cooldown | Credential unavailable được selector xử lý, retry hữu hạn | Cùng run: SOURCE/UNIT/SIMULATED | LIVE_FAILOVER NOT OBSERVED; 429 cũ UNKNOWN; không suy quota remaining từ usage |
| Pool unavailable | Persist task/next action, tôn trọng cooldown, bounded backoff, tiếp tục khi có capacity | Chưa có end-to-end workflow proof | OPEN; không replay mù request/tool, không retry storm |
| Session interruption | Resume đúng session/checkpoint, phân biệt explicit wake, user Stop và provider recovery; không lặp side effect | [7D live proof](../evidence/run-20260924-phase7d-wakeup-proof/summary.md): exit/resume native; T1/T2 expected cancellation do interrupt; T3 explicit delivery. [Prototype](../evidence/run-20260925-phase7d-recovery-prototype/summary.md) và [continuation hardening](../evidence/run-20260925-phase7d-recovery-hardening/summary.md) | Same-session resume và explicit wake `LIVE`; terminal states, Observe-state preservation, recover-only, paged receipt lookup, idle/owner preflight, run-owned lease, Stop serialization và task receipt `SIMULATED`; provider failure, daemon restart và unattended recovery live `NOT OBSERVED` |
| Lost-acceptance cancellation | Persist Stop trước I/O; recover đúng delivery; chỉ interrupt recovered live turn thuộc session; không ordinary resend | [Hardening continuation](../evidence/run-20260925-phase7d-recovery-hardening-continuation/summary.md) | `SIMULATED PASS`: restart/recover-only, interrupt response-loss reconciliation và unconfirmed isolation; `LIVE NOT OBSERVED` |
| Delivery response-loss recovery | AO/provider nhận task; adapter làm mất response; wrapper restart và recover-only cùng ID; receipt đúng task/artifact; 0 resend | [7D LIVE correction](../evidence/run-20260925-phase7d-stop-live-recovery/durable-correction.md) | AO/provider delivery `LIVE PASS`; transport response-loss `SIMULATED` injection |
| Cancellation / Stop | Persist Stop, reconcile target state, không ordinary resend; exact-turn safety | [7D LIVE continuation](../evidence/run-20260925-phase7d-stop-live-recovery/summary.md) | Logic Stop `SIMULATED PASS`; Stop LIVE `NOT OBSERVED`; exact-turn Stop trên shared session unsupported và fail-closed |
| Repo portability | Cùng quy trình trên repo khác, không logic đặc thù video | Chưa có | OPEN; 7E sau pilot có giới hạn |

Phân biệt SOURCE (đọc code), UNIT/SIMULATED (fake executor), LIVE_ROUTING, LIVE_FAILOVER và AO_TOOL_LOOP. Không có 429 tự nhiên thì mô phỏng, không đốt quota để tạo lỗi. Toàn pool unavailable phải kiểm persistence/backoff/resume, không yêu cầu provider luôn sẵn sàng.

Để chấp nhận một workflow: ghi task/dependency/owner, AO message/turn IDs, artifact revision/hash, review verdict và rework, integration/test receipt, interruption/recovery cùng mọi can thiệp coordinator. Worker tự báo PASS hoặc tên test trong stdout không đủ.

CP1 browser UPSTREAM_PATH_RED chỉ chứng minh thiếu control sau prerequisite PASS; không chứng minh command/SSE/modal. CP2A chưa accepted; inventory và E3/Temporal giới hạn ở [kế hoạch workload](phase7-first-workload-plan.md). Không nâng thành gate chặn workforce. Lượt docs-only chỉ kiểm diff/link/nhất quán, không runtime/test lại.
