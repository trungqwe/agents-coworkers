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
| `INV-005` Concurrency Target Starts at 3 | **L7** | **L7 (PASS)** | Initial runtime verification target: 3; maximum proven live concurrency: 3 (Gate 10 verified live concurrent execution with 38.848216s simultaneous overlap from max(started_at) to min(completed_at)) |
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
| Review/rework | Orchestrator chấp nhận hoặc yêu cầu sửa từ diff/test; worker rework và review lại nếu có lỗi thật | Continuation 7B–7C; [7E self-host pass](../evidence/run-20260926-phase7e-selfhost-pass/summary.md) | **OBSERVED**: Worker B thực hiện rework thật về exit-code matrix, CLI subprocess tests và timeout handling sau test failure ban đầu, được orchestrator ACCEPT |
| Integration/dependency | Chỉ ghép task khi dependency/review đạt, một integration owner, test chung | Continuation: C được tạo sau A/B ACCEPT, cherry-pick A rồi B, combined unittest 8 tests | OBSERVED trên fixture; không push/merge và không phải integration Product |
| Routing/affinity | Giữ binding khi eligible; quan sát selected IDs theo session/model | [pool onboarding](../evidence/run-20260923-phase8-pool-onboarding/summary.md) | LIVE_ROUTING 14/14, 5 IDs; không khẳng định cả pool usable hoặc phân phối đều |
| Failover/cooldown | Credential unavailable được selector xử lý, retry hữu hạn | Cùng run: SOURCE/UNIT/SIMULATED | LIVE_FAILOVER NOT OBSERVED; 429 cũ UNKNOWN; không suy quota remaining từ usage |
| Pool unavailable | Persist task/next action, tôn trọng cooldown, bounded backoff, tiếp tục khi có capacity | Chưa có end-to-end workflow proof | OPEN; không replay mù request/tool, không retry storm |
| Session interruption | Resume đúng session/checkpoint, phân biệt explicit wake, user Stop và provider recovery; không lặp side effect | [7D live proof](../evidence/run-20260924-phase7d-wakeup-proof/summary.md): exit/resume native; T1/T2 expected cancellation do interrupt; T3 explicit delivery. [Prototype](../evidence/run-20260925-phase7d-recovery-prototype/summary.md) và [continuation hardening](../evidence/run-20260925-phase7d-recovery-hardening/summary.md) | Same-session resume và explicit wake `LIVE`; terminal states, Observe-state preservation, recover-only, paged receipt lookup, idle/owner preflight, run-owned lease, Stop serialization và task receipt `SIMULATED`; provider failure, daemon restart và unattended recovery live `NOT OBSERVED` |
| Lost-acceptance cancellation | Persist Stop trước I/O; recover đúng delivery; chỉ interrupt recovered live turn thuộc session; không ordinary resend | [Hardening continuation](../evidence/run-20260925-phase7d-recovery-hardening-continuation/summary.md) | `SIMULATED PASS`: restart/recover-only, interrupt response-loss reconciliation và unconfirmed isolation; `LIVE NOT OBSERVED` |
| Delivery response-loss recovery | AO/provider nhận task; adapter làm mất response; wrapper restart và recover-only cùng ID; receipt đúng task/artifact; 0 resend | [7D LIVE correction](../evidence/run-20260925-phase7d-stop-live-recovery/durable-correction.md) | AO/provider delivery `LIVE PASS`; transport response-loss `SIMULATED` injection |
| Cancellation / Stop | Persist Stop, reconcile target state, không ordinary resend; exact-turn safety | [7D LIVE continuation](../evidence/run-20260925-phase7d-stop-live-recovery/summary.md) | Logic Stop `SIMULATED PASS`; Stop LIVE `NOT OBSERVED`; exact-turn Stop trên shared session unsupported và fail-closed |
| Repo portability | Cùng quy trình trên repo khác, không logic đặc thù video | [7E self-host pass](../evidence/run-20260926-phase7e-selfhost-pass/summary.md) | **OBSERVED**: Quy trình workforce và executable `recovery.exe` self-host LIVE thành công trên repo thứ hai `agents-coworkers` với profile Gemini 3.7. Giữ các giới hạn: provider outage LIVE, daemon restart LIVE, Stop LIVE đều NOT OBSERVED; exact-turn Stop trên shared session unsupported/fail-closed |

Phân biệt SOURCE (đọc code), UNIT/SIMULATED (fake executor), LIVE_ROUTING, LIVE_FAILOVER và AO_TOOL_LOOP. Không có 429 tự nhiên thì mô phỏng, không đốt quota để tạo lỗi. Toàn pool unavailable phải kiểm persistence/backoff/resume, không yêu cầu provider luôn sẵn sàng.

Để chấp nhận một workflow: ghi task/dependency/owner, AO message/turn IDs, artifact revision/hash, review verdict và rework, integration/test receipt, interruption/recovery cùng mọi can thiệp coordinator. Worker tự báo PASS hoặc tên test trong stdout không đủ.

CP1 browser UPSTREAM_PATH_RED chỉ chứng minh thiếu control sau prerequisite PASS; không chứng minh command/SSE/modal. CP2A chưa accepted; inventory và E3/Temporal giới hạn ở [kế hoạch workload](phase7-first-workload-plan.md). Không nâng thành gate chặn workforce. Lượt docs-only chỉ kiểm diff/link/nhất quán, không runtime/test lại.

---

## 6. Canonical Gates for Phase 8 (Gates 8A–8E — Proposed / Not Authorized)

> [!IMPORTANT]
> Toàn bộ các Gate 8A–8E dưới đây là quy chuẩn nghiệm thu được đề xuất cho Phase 8. Hiện tại **CHƯA ĐƯỢC CẤP QUYỀN THỰC THI (IMPLEMENTATION NOT AUTHORIZED)**. Khi bắt đầu từng slice, các gate này đóng vai trò là oracle bắt buộc để nghiệm thu trước khi chuyển giao.

| Gate | Tên Gate / Năng lực | Oracle nghiệm thu bắt buộc | Bằng chứng yêu cầu (Evidence Tier) | Trạng thái |
| :--- | :--- | :--- | :--- | :--- |
| **Gate 8A** | Control Surface & Preflight CLI | Lệnh nhị phân CLI thực thi (`coworkers doctor`, `coworkers attach`, `coworkers status`) hoạt động chính xác; exit code chuẩn hóa; stderr/stdout không rò rỉ secret; `coworkers run` bị từ chối rõ ràng với mã thoát 1 | L4 (Local Process Smoke; unit tests L2 và integration giả lập L3 chỉ là bằng chứng bổ trợ; Gate 8A không gọi live provider) | **PROPOSED** |
| **Gate 8B** | Autonomous Task Graph & Concurrency | Vòng lặp điều phối đa tác nhân (1–3 workers) tự phân rã task DAG, review diff, chỉ thị rework thật (dùng fixture cô lập seed sẵn lỗi cho state-machine test) và tích hợp commit vào local branch; không race condition | L7 (Multi-agent concurrent wave execution logs, checkpoint records, task review/rework transcripts, integration commit receipts) | **PROPOSED** |
| **Gate 8C** | Routing Observability & Telemetry | Thu thập và xử lý downstream telemetry (`X-CPA-TRACE-ID`, `Retry-After`, status classification); nhận diện absent header là `NOT_OBSERVED`; phân loại lỗi không suy diễn chủ quan; không lộ secret | L5 (Redacted telemetry logs, trace correlation records, error classifier verification reports) | **PROPOSED** |
| **Gate 8D** | Real Product Workload Isolation | Workforce tự vận hành trên dự án thật (`AI Auto Video Creator`); Product ROOT checkout giữ nguyên sạch (`INV-001`); worker ghi trên worktree cô lập; tích hợp dừng tại local integration branch, không push main | L7 (Worktree isolation verification, read-only root integrity proof, local integration branch diff/test results) | **PROPOSED** |
| **Gate 8E** | Capacity Expansion & Reliability (Conditional) | Telemetry chứng minh pool đủ năng lực (`NOT_TRIGGERED_WITH_EVIDENCE`) HOẶC hoàn thành onboarding và capacity proof nếu thực sự thiếu hụt | L5/L7 (Pool capacity telemetry analysis, or credential onboarding logs + concurrent wave proof) | **PROPOSED (CONDITIONAL)** |

### Chi tiết hợp đồng kiểm chứng Gate 8A (Control Surface CLI)
- **Oracle bắt buộc & Evidence Tier**:
  * Tier đóng gate: **L4 (Local Process Smoke)**.
  * Unit tests (L2) và mock integration (L3) chỉ là bằng chứng bổ trợ trong quá trình phát triển; **tuyệt đối không dùng unit/module test để thay thế việc thực thi CLI khi đóng gate**.
  * Tier đóng gate cho Gate 8A là L4 Local Process Smoke; do slice này không gọi live provider nên unit tests L2 và integration giả lập L3 đóng vai trò bằng chứng bổ trợ.
- **Ma trận mã thoát chuẩn hóa (Exit Codes)**:
  * `0`: Thành công hoàn toàn; trạng thái hệ thống/phiên/manifest được xác nhận hợp lệ.
  * `1`: Lỗi sử dụng CLI (CLI usage error), thiếu tham số bắt buộc, cờ lệnh không hợp lệ, hoặc gọi lệnh chưa hỗ trợ (`coworkers run` trong Slice 8A luôn trả về mã thoát `1`).
  * `2`: Lỗi kiểm tra / xác thực (validation error): baseline commit mismatch, Product root checkout bị dirty, không thể ràng buộc worktree vật lý một cách authoritative, hoặc phát hiện nhiều daemon AO hợp lệ cùng lúc.
  * `3`: Lỗi hạ tầng / thời gian chờ: không thể kết nối tới daemon/gateway, identity probe thất bại, hoặc timeout context.
- **Phân tách Hợp đồng Dữ liệu (RunSpec vs RunManifest)**:
  * `RunSpec` (định dạng JSON, input do người dùng cung cấp, `schemaVersion = "run-spec/v1-draft"`): bất biến sau khi run bắt đầu; phân biệt rõ `targetRoot` (checkout gốc Product) và `executionWorkspace` (AO worktree); `expectedBranch` sử dụng placeholder `<AO_SESSION_BRANCH>` (đại diện cho nhánh AO session/worktree dự kiến); chứa baseline SHA, profiles, policies, và delegated authority. Tuyệt đối không chứa `aoUrl` hay `gatewayUrl`; việc phân giải endpoint được định nghĩa qua `endpointPolicy` (không chứa URL). Chứa chính sách công cụ `requiredTools` (`git` luôn bắt buộc cho Slice 8A; các công cụ khác chỉ kiểm khi khai báo, kiểm tra trực tiếp qua `exec.LookPath`, tên công cụ không chứa path hoặc shell metacharacters; Go/Node không phải runtime prerequisite mặc định của target repo) và chính sách workspace `workspacePolicy` (`productRootMustBeClean: true`, `executionWorkspaceDirtyPolicy`: `require_clean` [mặc định] hoặc `allow_dirty_recorded`).
  * `RunManifest` (định dạng JSON, output sinh ra duy nhất từ `coworkers attach` qua atomic write, `schemaVersion = "run-manifest/v1-draft"`): chứa `runSpecSha256`, `generatedBy: "coworkers attach"`, resolved endpoints (aoUrl, aoIdentity, aoDiscoverySource, aoPid, aoPidStatus), `gatewayProbe` (url, catalogPath: "/v1/models", catalogStatus: "VERIFIED", observedModels, `providerCallPerformed: false`, `credentialEligibility: "NOT_OBSERVED"`), target repository identity, baseline SHA, current HEAD, `sessionBranch` và `worktreeBranch` (hai giá trị phải khớp nhau), `worktreeBinding` (canonicalPath, worktreeBranch, head, isClean, dirtyPolicy, porcelainSha256, verifiedPorcelain), đối tượng cấu hình duy nhất `attachedSessionProfile` (kind, harness, model, reasoningEffort, status), `lease` (workspaceRoot, runOwner, taskId: "__run__", ownerId, observedState: "FREE", observedPid: 0; không có trạng thái ACQUIRED), source provenance, và timestamps. Không chứa secret (token, cookie, raw email, hoặc raw credential filename) và không chứa endpoint input dư thừa.
  * Vòng đời thống nhất của RunSpec và RunManifest:
    - `coworkers doctor`: Chuẩn hóa cú pháp: `coworkers doctor --spec <run-spec.json> [--ao-url <url>] [--gateway-url <url>] [--json]`. Cờ `--spec` là bắt buộc (thiếu, hỏng hoặc sai schema RunSpec: exit 2). Lệnh `doctor` không nhận cờ `--manifest`, không tạo hoặc sửa `RunManifest`, và không inspect hay acquire lease. Kiểm tra gateway reachable, `CLIPROXY_KEY` được chấp nhận, `/v1/models` trả catalog hợp lệ và model yêu cầu xuất hiện trong catalog (tuyệt đối không đồng nhất catalog với credential eligibility hay usable capacity; `providerCallPerformed: false`, `credentialEligibility: "NOT_OBSERVED"`). Kiểm tra runtime tools theo `requiredTools` (`git` bắt buộc; Go/Node không phải prerequisite mặc định của target repo).
    - `coworkers attach`: Đọc `RunSpec`. Nhận `--session`, `--workspace` và `--manifest` (đây là output path). Kiểm tra chính sách workspace dirty: Product root dùng raw `git status --porcelain=v1 -z` không có ngoại lệ (bất kỳ tệp nào kể cả dưới `.agents-coworkers/**` đều làm Product root dirty và exit code 2). Execution workspace đánh giá source-dirty từ raw porcelain (`git status --porcelain=v1 -uall -z`) sau khi loại duy nhất exact untracked porcelain record đã tính từ `RunSpec.runId` (`?? .agents-coworkers/recovery-leases/<sha256(runId + "\n__run__")>.lease`). Mọi trạng thái tracked, staged (`A `), modified (` M`, `M `), deleted (` D`, `D `), type changed, unmerged, rename hoặc copy tại cùng path vẫn được xem là source-dirty; không gọi mọi nội dung `.agents-coworkers/**` là expected; ngoại lệ lease duy nhất này chỉ tồn tại để giữ tính lũy đẳng (idempotency) khi cùng run chuyển trạng thái lease từ FREE→OWNED. `require_clean` + dirty worktree: exit code 2; `allow_dirty_recorded` + dirty worktree: PASS và ghi `porcelainSha256` là SHA-256 của canonical filtered source-porcelain này (không tự suy mọi dirty state là "expected"). Xác minh session/worktree theo thuật toán 9 bước bắt buộc. Gọi API inspect read-only kiểm tra lease (`workspaceRoot`, `runOwner`, `taskId: "__run__"`); tuyệt đối không gọi `Acquire` hoặc `Release` vì không có process run dài hạn giữ lease (`FREE`: tiếp tục; `OWNED` cùng run: idempotent readback; `LOCKED`: exit 2). Tạo `RunManifest` bằng cơ chế ghi nguyên tử (atomic write: temp file -> fsync/close -> rename). Nếu manifest chưa tồn tại: tạo mới (lệnh attach tự động tạo mới, không đòi hỏi tệp có sẵn từ trước). Nếu manifest đã tồn tại và khớp hoàn toàn (`runId`, `runSpecSha256`, session, repository và worktree binding): trả về thành công lũy đẳng (idempotent success), không viết lại. Nếu manifest bị hỏng hoặc sai lệch binding: dừng lại ngay (fail-closed) và thoát với exit code `2`.
    - `coworkers status`: Chỉ đọc `RunManifest` và trạng thái live read-only. Luôn re-inspect lease thực tế từ `workspaceRoot`, `runOwner`, `taskId: "__run__"`; tuyệt đối không tin `observedState` cũ trong manifest. Không tự ý phá stale lease, không kiểm PID còn sống để tự phá lease. Nếu manifest thiếu hoặc hỏng: thoát với exit code `2`. Không đọc `RunSpec` để tự tái tạo binding. Không chỉnh sửa manifest. Tuyệt đối không có trường Task DAG nào trong Slice 8A.
  * Cú pháp lệnh: `coworkers doctor --spec <run-spec.json> [--ao-url <url>] [--gateway-url <url>] [--json]`, `coworkers attach --spec <run-spec.json> --session <id> --workspace <path> --manifest <output-path> [--ao-url <url>] [--gateway-url <url>]`, `coworkers status --manifest <path-to-run-manifest.json> [--ao-url <url>] [--gateway-url <url>] [--json]` (triển khai thuần túy bằng thư viện chuẩn Go `encoding/json`, không thêm YAML dependency).
- **Hợp đồng JSON Output & Ranh giới an toàn thông tin**:
  * Khi truyền cờ `--json`, CLI bắt buộc phải xuất JSON hợp lệ ra `stdout` theo đúng cấu trúc dữ liệu đã định nghĩa.
  * Mọi thông báo tiến trình, cảnh báo hoặc lỗi người dùng được ghi ra `stderr`.
  * Tuyệt đối không xuất raw API keys, bearer tokens, OAuth refresh tokens, session cookies, hoặc địa chỉ email cá nhân ra cả `stdout` và `stderr`. Các định danh tài khoản bắt buộc phải được ẩn danh qua chuỗi băm `authIndex` (16-hex) hoặc alias đã che mờ.
- **Từ chối lệnh `coworkers run`**: Lệnh `coworkers run` trong Slice 8A bắt buộc luôn trả về mã thoát `1` với thông báo lỗi rõ ràng rằng lệnh chạy tự động chưa được hỗ trợ cho tới khi Slice 8B hoàn thành và được nghiệm thu.
- **Thuật toán Worktree Binding bắt buộc cho `coworkers attach`**:
  Do endpoint `GET /api/v1/sessions/{id}` của AO (SessionView) không trả về đường dẫn thư mục vật lý của worktree, lệnh `attach` không được tuyên bố đã kiểm chứng worktree chỉ dựa trên kết quả trả về của session API. Lệnh `attach` bắt buộc thực thi 9 bước tuần tự:
  1. Đọc session qua AO API và lấy project ID + branch;
  2. Canonicalize target root và explicit execution workspace path;
  3. Chạy `git worktree list --porcelain -z` trên đúng repository;
  4. Tìm đúng một worktree có branch bằng session branch;
  5. Path canonical của worktree phải bằng execution workspace;
  6. HEAD phải bằng expected HEAD/baseline theo trạng thái run;
  7. Repository common-dir và identity phải khớp target repo;
  8. Nếu không có worktree, có nhiều kết quả, detached ngoài contract hoặc mismatch -> lập tức thoát với exit code `2`;
  9. Tuyệt đối không đọc `ao.db` và không suy physical path từ `SessionView`.
- **Ranh giới Run Lease**: `FileLease` theo source thật đặt tại `<executionWorkspace>/.agents-coworkers/recovery-leases`, tên file suy từ SHA-256(`runOwner + "\n" + taskId`). Slice 8A dùng run-level lease identity cố định: `runOwner = RunSpec.runId`, `taskId = "__run__"`, `ownerId = RunSpec.runId`, `workspaceRoot = canonical executionWorkspace`. Trạng thái quan sát được: `FREE` (lease file không tồn tại), `OWNED` (lease tồn tại và khớp run), `LOCKED` (lease thuộc owner/task khác). Tuyệt đối không dùng trạng thái `ACQUIRED` trong manifest Slice 8A. `FileLease` dùng `O_EXCL` để phối hợp độc quyền giữa các process `coworkers`/`recovery` cùng tuân thủ wrapper contract. Nó không phải global AO lock và không ngăn client AO bên ngoài sử dụng session. Không có TTL và không tự phá stale lease. Lệnh `status` luôn re-inspect trực tiếp từ `workspaceRoot`, `runOwner`, `taskId: "__run__"`, không tin `observedState` cũ trong manifest. Lệnh `attach` chỉ gọi API inspect read-only, không gọi Acquire hoặc Release. PID chỉ là metadata giám sát, không kiểm tra PID còn sống để tự phá lease. Slice 8A chỉ được bổ sung API inspect read-only nhận đủ `workspaceRoot`, `runOwner`, `taskId` cần thiết cho status và attach, không thay đổi ngữ nghĩa Acquire/Release.

### Quy định Concurrency & Rework cho Gate 8B
- Mức concurrency mặc định tối đa cho scheduler là **3 workers đồng thời** (mức tối đa đã được kiểm chứng thực tế tại Gate 10).
- Các mức từ **4–7 workers** là `EXPERIMENTAL_CAPACITY`, không được phép sử dụng trong cấu hình mặc định và chỉ được mở khi có telemetry Slice 8C chứng minh, có authority riêng từ user, và có bằng chứng cô lập/năng lực độc lập.
- **Rework Oracle**: Không yêu cầu tạo lỗi nhân tạo trong Product repository. State-machine rework oracle dùng **fixture cô lập có contract violation được seed trước**. Khi chạy trên workload thật (Slice 8D), orchestrator chỉ phát rework khi tìm thấy lỗi thật trong mã nguồn hoặc test của worker.

### Ranh giới An toàn Thông tin cho Gate 8C
- Header `X-CPA-TRACE-ID`: Bắt buộc ghi nhận rõ nguồn gốc mã nguồn CLIProxyAPI tại commit `2430354330af80b645f9ffb1a51e1e7c72c4cc8e` (lưu ý: việc audit mã nguồn không chứng minh binary đang chạy được build từ SHA đó).
- `authIndex` là chuỗi 16-hex pseudonymous có rủi ro liên kết tài khoản (correlation risk), không chứa raw token nhưng là metadata vận hành nhạy cảm cần được lọc (redaction) và giới hạn lưu trữ (bounded retention); không được tuyên bố là an toàn tuyệt đối.
- Khi `X-CPA-TRACE-ID` vắng mặt (chưa chọn credential hoặc response không đi qua trace callback), bắt buộc ghi nhận trạng thái là `NOT_OBSERVED` hoặc `UNKNOWN`.
- `Retry-After` chỉ được ghi nhận khi thực sự tồn tại trong response headers; không giả định mọi mã 429/503 đều có. Bộ phân loại lỗi phải dựa trên status code + body code/reason + headers; nếu thiếu dữ liệu thì phân loại `UNKNOWN`.

### Ranh giới Sản phẩm cho Gate 8D
- Product ROOT checkout của `AI Auto Video Creator` luôn ở trạng thái read-only (`INV-001`). Các worktree cô lập được phép ghi theo allowlist cụ thể của từng run.
- Gate 8D chỉ được coi là đạt khi toàn bộ commit tích hợp nằm trên local integration branch (`integration/run-<id>`); tuyệt đối không tự động merge hoặc push vào `main` của Product repository.

### Điều kiện Nghiệm thu Gate 8E (Conditional)
- Nếu dữ liệu telemetry từ Slice 8C/8D chứng minh pool hiện tại đáp ứng tốt workload mà không cạn kiệt hạn ngạch: Gate 8E được nghiệm thu với trạng thái `NOT_TRIGGERED_WITH_EVIDENCE`, không cần thực hiện onboarding thêm tài khoản.
- Việc đóng Phase 8 không phụ thuộc vào số lượng tài khoản trong kho inventory; chỉ số lượng tài khoản sử dụng được (usable capacity) dựa trên dữ liệu thực tế mới có giá trị.

### Exact Authority Delta cho Implementation 8A (Chưa thực hiện)
Nhằm chuẩn bị cho việc xin cấp quyền thực thi mã nguồn ở bước tiếp theo, phạm vi mã nguồn được giới hạn chính xác trong 17 tệp sau (tuyệt đối không dùng ký tự đại diện hay quy định tương đương):
- `cmd/coworkers/main.go`
- `cmd/coworkers/main_test.go`
- `internal/workforce/control/types.go`
- `internal/workforce/control/run_spec.go`
- `internal/workforce/control/run_spec_test.go`
- `internal/workforce/control/manifest.go`
- `internal/workforce/control/manifest_test.go`
- `internal/workforce/control/discovery.go`
- `internal/workforce/control/discovery_test.go`
- `internal/workforce/control/doctor.go`
- `internal/workforce/control/doctor_test.go`
- `internal/workforce/control/attach.go`
- `internal/workforce/control/attach_test.go`
- `internal/workforce/control/status.go`
- `internal/workforce/control/status_test.go`
- `internal/recovery/lease.go` (chỉ bổ sung API inspect read-only nhận workspaceRoot, runOwner, taskId cần cho status và attach; không đổi Acquire/Release semantics; không TTL, không auto-break stale lease)
- `internal/recovery/lease_test.go`

Ranh giới kỹ thuật:
- Không sửa `go.mod` và `go.sum`. Chỉ sử dụng thư viện chuẩn của Go (Go standard library).
- Nghiêm cấm: task DAG execution, scheduler, worker dispatch, can thiệp Product root, gọi provider workload LLM, onboarding tài khoản, git merge hoặc push lên remote trong Slice 8A.

### Behavior Matrix 8A
Ma trận hành vi bắt buộc kiểm chứng cho Slice 8A:
1. **Doctor**:
   - Cờ `--spec` là bắt buộc; thiếu, hỏng hoặc sai schema `RunSpec`: exit code 2;
   - Không nhận cờ `--manifest`; không tạo hoặc sửa tệp `RunManifest`; không inspect hay acquire lease;
   - Model catalog presence không nâng thành credential eligibility hay usable capacity; `providerCallPerformed` luôn `false`, `credentialEligibility` luôn `"NOT_OBSERVED"`;
   - Git là runtime tool bắt buộc; các công cụ khác kiểm tra theo `requiredTools` qua `exec.LookPath`; tên công cụ không hợp lệ (chứa path/metacharacter) fail-closed exit code 2; Go/Node không phải runtime prerequisite mặc định của target repo;
   - Kiểm tra redaction bí mật `CLIPROXY_KEY` trong stdout/stderr/logs;
   - Kiểm tra cờ endpoint tường minh hoặc biến môi trường không hợp lệ sẽ fail-closed ngay lập tức, không fallback;
   - Kiểm tra mã thoát: exit code 2 (validation/catalog/auth lỗi) và exit code 3 (unreachable/timeout) đúng contract;
   - Xác nhận Product root checkout sạch (`git status` clean).
2. **Attach**:
   - Thiếu hoặc hỏng `RunSpec`: exit code 2;
   - Kiểm tra workspace dirty policy: Product root dùng raw `git status --porcelain=v1 -z` không ngoại lệ (bất kỳ tệp nào kể cả `.agents-coworkers/**` đều làm Product root dirty -> exit code 2); Execution workspace đánh giá source-dirty từ raw porcelain (`git status --porcelain=v1 -uall -z`) sau khi loại duy nhất exact untracked porcelain record đã tính từ `runId` (`?? .agents-coworkers/recovery-leases/<sha256(runId + "\n__run__")>.lease`); mọi trạng thái tracked, staged, modified, deleted, rename hoặc copy tại cùng path vẫn được xem là source-dirty; không gọi mọi nội dung `.agents-coworkers/**` là expected; ngoại lệ lease chỉ tồn tại để giữ idempotency khi cùng run chuyển FREE→OWNED; `require_clean` + dirty worktree: exit code 2; `allow_dirty_recorded` + dirty worktree: PASS và ghi `porcelainSha256` (SHA-256 của canonical filtered source-porcelain này; không tự suy mọi dirty state là "expected");
   - Inspect lease `FREE`, `OWNED`, `LOCKED` từ `workspaceRoot`, `runOwner`, `taskId: "__run__"`; chỉ gọi API inspect read-only, tuyệt đối không gọi `Acquire` hoặc `Release`; `FREE`: tiếp tục; `OWNED` cùng run: idempotent readback; `LOCKED`: exit code 2;
   - Manifest chưa tồn tại (manifest absent): tạo mới bằng cơ chế atomic write thành công;
   - Manifest đã tồn tại và khớp hoàn toàn (`runId`, `runSpecSha256`, session, repo, worktree): trả về thành công lũy đẳng (idempotent success);
   - Manifest đã tồn tại nhưng hỏng cấu trúc hoặc sai lệch binding: fail-closed, exit code 2;
   - Chỉ xác nhận đối tượng duy nhất `attachedSessionProfile`;
   - Kiểm tra băm toàn vẹn `runSpecSha256`;
   - Tuyệt đối không truy cập trực tiếp `ao.db`.
3. **Status**:
   - Xử lý các tình huống manifest: hợp lệ, thiếu, hoặc hỏng cấu trúc (thiếu/hỏng: exit code 2);
   - Không tự động phục hồi hay tái tạo manifest từ `RunSpec`; không đọc `RunSpec` để tái tạo binding;
   - Re-inspect lease từ `workspaceRoot`, `runOwner`, `taskId: "__run__"`; không tin `observedState` cũ trong manifest; báo cáo `FREE`, `OWNED`, hoặc `LOCKED`; không phá stale lease; PID chỉ là metadata giám sát, không kiểm tra PID còn sống để tự phá lease;
   - Tuyệt đối không xuất hiện bất kỳ trường Task DAG nào trong Slice 8A.
4. **CLI**:
   - Lệnh `doctor` chuẩn hóa cú pháp: `coworkers doctor --spec <run-spec.json> [--ao-url <url>] [--gateway-url <url>] [--json]`, bắt buộc có `--spec`, không nhận cờ `--manifest`;
   - Lệnh `attach` bắt buộc nhận cả `--spec` và `--manifest` (output path);
   - Lệnh `status` bắt buộc nhận `--manifest`;
   - Lệnh `run` bị từ chối rõ ràng với mã thoát `1` trong toàn bộ Slice 8A;
   - Ma trận mã thoát chuẩn hóa: `0` (thành công), `1` (lỗi usage/lệnh chưa hỗ trợ), `2` (lỗi validation/conflict), `3` (endpoint unavailable/timeout);
   - Xuất dữ liệu JSON có cấu trúc qua `stdout` và thông điệp chẩn đoán qua `stderr`.
