# 04 — Development Roadmap & Runtime Gates

Roadmap này là nguồn trạng thái duy nhất của sản phẩm workforce. Phase 0–6 và Gates 1–10 là các mốc lịch sử được giữ nguyên; Phase 7–8 được hiệu chỉnh theo D012. Tích hợp component đã đạt không có nghĩa vòng tự vận hành đã hoàn tất.

---

## 1. Roadmap Overview & Status Matrix

| Phase | Description | Status | Primary Gate / Deliverable |
| :--- | :--- | :--- | :--- |
| **Phase 0** | Source Feasibility | **COMPLETE** | AO adapters, CLIProxyAPI translators, schema verified |
| **Phase 1** | Pre-Auth Integration Foundation | **COMPLETE** | Configs, sanitized tools, discovery, regression tests |
| **Phase 2** | Minimal Authentication Proof | **COMPLETE** | Exactly 1 Codex + 1 Antigravity account logged in |
| **Phase 3** | Direct Provider Runtime Proof | **COMPLETE** | Gates 1, 2, 3, 4 (Catalog, Astra, Gemini, Tool Roundtrip) |
| **Phase 4** | Codex Harness Runtime Proof | **COMPLETE** | Gates 5, 6 (Codex -> Astra, Codex -> Gemini Tool Loop) |
| **Phase 5** | AO Runtime Proof | **COMPLETE** | Gates 7, 8, 9 (Real Orchestrator, Real Worker, Rework Loop) |
| **Phase 6** | 3-Worker Concurrency Proof | **COMPLETE** | Gate 10 (Real 3-Worker Concurrency Wave) |
| **Phase 7** | Vòng workforce tự vận hành trên workload có giới hạn | **COMPLETE_WITH_ACCEPTED_LIMITATIONS** | 7A pilot; 7B/7C code-review-integration; 7D delivery recovery LIVE; 7E repo portability và executable self-host LIVE với Worker B rework OBSERVED; accepted limitations: provider outage LIVE, daemon restart LIVE, Stop LIVE đều NOT OBSERVED, exact-turn Stop trên shared session unsupported/fail-closed |
| **Phase 8** | Lộ trình thương phẩm hóa workforce (8A–8E) | **DESIGN_PHASE — IMPLEMENTATION_NOT_AUTHORIZED** | Lộ trình 5 slice: 8A Control surface CLI & preflight, 8B Task graph & loop (1–3 workers), 8C Telemetry quan sát routing, 8D Workload trên dự án thật (read-only root), 8E Mở rộng dung lượng (conditional); implementation chưa được cấp quyền |

---

## 2. Canonical 10 Runtime Verification Gates

Candidate A has transitioned from `PROVISIONAL` to `ACCEPTED` following sequential verification of all 10 canonical gates:

1. **Gate 1: CLIProxy Catalog**: `GET /v1/models` contains both `gpt-6-astra` and `gemini-3.8-flash-high`. (**VERIFIED - L5**)
2. **Gate 2: Direct Astra Responses**: Successful non-stream and stream response parsing via CLIProxyAPI gateway. (**VERIFIED - L5**)
3. **Gate 3: Direct Gemini Responses**: Successful non-stream and stream response parsing via CLIProxyAPI gateway. (**VERIFIED - L5**)
4. **Gate 4: Gemini Tool Roundtrip**: Function call translation, execution, and response synthesis fidelity verified. (**VERIFIED - L5**)
5. **Gate 5: Codex -> Astra**: Successful `codex exec` invocation through CLIProxyAPI targeting `gpt-6-astra`. (**VERIFIED - L5**)
6. **Gate 6: Codex -> Gemini Tool Loop**: Successful multi-turn coding and file editing loop via `codex exec` targeting `gemini-3.8-flash-high`. (**VERIFIED - L5**)
7. **Gate 7: Real AO Orchestrator**: Live session launched on Agent Orchestrator with `kind = "orchestrator"`. (**VERIFIED - L6**)
8. **Gate 8: Real AO Worker**: Live session launched on Agent Orchestrator with `kind = "worker"` using Gemini. (**VERIFIED - L6**)
9. **Gate 9: AO Rework Loop**: Orchestrator reviews worker worktree output and successfully issues a rework directive. (**VERIFIED - L6**)
10. **Gate 10: Real 3-Worker Wave**: Three concurrent live worker sessions complete isolated tasks without worktree, locking, or routing failures (verifying initial runtime verification target of 3 workers; maximum proven live concurrency: 3). (**VERIFIED - L7**)

---

## 3. Phase Details & Exit Criteria

### Phase 2: Minimal Authentication Proof (COMPLETE)
- **Objective**: Authenticate the minimum viable credential set (1 ChatGPT Plus + 1 Gemini Pro) to enable runtime testing.
- **Entry Criteria**: Phase 1 complete; sanitized inventory and smoke scripts operational.
- **Required Work**:
  1. Interactive login of exactly **one** ChatGPT Plus account (`-codex-login`).
  2. Interactive login of exactly **one** Gemini Pro Google account (`-antigravity-login`).
  3. Verify with `scripts/auth-inventory.ps1` that counts reach Codex: 1, Antigravity: 1.
- **Exit Criteria**: Sanitized inventory confirms 1 Codex and 1 Antigravity credential present.
- **Evidence Required**: Sanitized inventory output (hashes only; zero emails or raw filenames).
- **Explicit Non-Goals**: Do NOT log in all 14 accounts in this phase. Test with minimal credentials first.

### Phase 3: Direct Provider Runtime Proof (COMPLETE)
- **Objective**: Verify raw HTTP API translation and tool roundtrips through CLIProxyAPI gateway.
- **Entry Criteria**: Phase 2 complete (1+1 accounts loaded).
- **Required Work**: Execute Gates 1, 2, 3, and 4.
- **Exit Criteria**: Both models respond to `/v1/models` and direct Responses requests; tool call roundtrip passes.
- **Evidence Required**: HTTP responses and tool roundtrip test output recorded in `/evidence`.
- **Explicit Non-Goals**: Do not invoke Codex CLI or Agent Orchestrator yet.

### Phase 4: Codex Harness Runtime Proof (COMPLETE)
- **Objective**: Verify that Codex CLI interacts correctly with both models via CLIProxyAPI wire translation.
- **Entry Criteria**: Phase 3 complete.
- **Required Work**: Execute Gates 5 and 6 using isolated `CODEX_HOME`.
- **Exit Criteria**: `codex exec` succeeds on `gpt-6-astra` and performs multi-turn file edits on `gemini-3.8-flash-high`.
- **Evidence Required**: Execution transcripts with sanitized outputs.
- **Explicit Non-Goals**: Do not launch AO sessions.

### Phase 5: AO Runtime Proof (COMPLETE)
- **Objective**: Verify that Agent Orchestrator successfully provisions and supervises single sessions.
- **Entry Criteria**: Phase 4 complete.
- **Required Work**: Execute Gates 7, 8, and 9 on a live AO daemon.
- **Exit Criteria**: Orchestrator session runs; worker session runs; orchestrator reviews worker worktree and directs rework.
- **Evidence Required**: AO daemon session logs and git worktree commit logs.
- **Explicit Non-Goals**: Do not run concurrent workers yet.

### Phase 6: 3-Worker Concurrency Proof (COMPLETE)
- **Objective**: Prove concurrent multi-agent execution at the initial runtime verification target (3 workers). Initial runtime verification target: 3 workers; maximum proven live concurrency: 3 (verified at runtime in Gate 10).
- **Entry Criteria**: Phase 5 complete.
- **Required Work**: Execute Gate 10 (parallel 3-worker wave).
- **Exit Criteria**: 3 live workers execute in parallel across separate git worktrees without race conditions, token starvation, or session crashes (VERIFIED: maximum proven live concurrency = 3).
- **Evidence Required**: Parallel session execution logs and worktree integrity verification.
- **Explicit Non-Goals**: Do not claim 5 or 7 workers until Phase 8 (Capacity Expansion).

### Phase 7: Vòng workforce tự vận hành (COMPLETE_WITH_ACCEPTED_LIMITATIONS)

Candidate A và concurrency 3 đã có component proof. Fixture 7B–7C chứng minh hai worker code song song, GPT-5.5/low orchestrator review A/B, integration worker C và test chung; rework thực tế `NOT OBSERVED`. 7D chứng minh same-session native resume, explicit message wake và delivery recovery trên AO/provider thật; lỗi mất response tại adapter được inject `SIMULATED`. T1/T2 bị user/coordinator interrupt là expected cancellation, không phải auto-wake failure. Provider outage, daemon restart, unattended recovery và Stop LIVE vẫn `NOT OBSERVED`. CP1/CP2A chỉ là tiến độ workload mẫu; inventory tại [kế hoạch workload](phase7-first-workload-plan.md), không là lộ trình sản phẩm workforce.

| Bước tiếp theo | Đầu ra | Acceptance và điểm dừng |
|---|---|---|
| 7A — Capability-gap audit/pilot native | Native source + binary/runtime observation, task/authority contract | Pilot review ba worker [đã quan sát](../evidence/run-20260924-phase7a-native-pilot/summary.md), với profile lịch sử GPT-6-Astra/low; không viết lại primitive AO. |
| 7B — Orchestrator tự phân việc/nhận kết quả | Task graph, file/resource ownership, AO session/worktree/profile readback và messages | [Pilot ban đầu](../evidence/run-20260924-phase7bc-native-code-pilot/summary.md) giữ lần gián đoạn; [continuation](../evidence/run-20260924-phase7bc-integration-continuation/summary.md) dùng GPT-5.5/low, nhận receipts A/B qua AO và ACCEPT. Rework không cần thiết, `NOT OBSERVED`. |
| 7C — Review + integration/test | Diff review có lý do, kết quả ghép và test chung | Astra/GPT-6-Astra là profile của run lịch sử trước, không phải profile continuation. Orchestrator GPT-5.5/low tạo C sau ACCEPT, cherry-pick A rồi B không conflict, ACCEPT combined diff; `unittest` 8/8 PASS. |
| 7D — Recovery | Durable task/progress/delivery IDs, interruption/resume trace và side-effect safety | **`DELIVERY_RECOVERY_LIVE_PASS__STOP_SIMULATED_LIMITED`**. [Proof live](../evidence/run-20260924-phase7d-wakeup-proof/summary.md) xác nhận resume/wake; [delivery recovery](../evidence/run-20260925-phase7d-stop-live-recovery/durable-correction.md) xác nhận AO/provider LIVE với response-loss inject SIMULATED, cùng delivery ID và 0 resend. Stop chỉ SIMULATED; provider outage, daemon restart và unattended recovery `NOT OBSERVED`. |

[Continuation hardening 7D](../evidence/run-20260925-phase7d-recovery-hardening/summary.md) bổ sung state-preserving Observe backoff, `steered` contamination fail-closed, session idle/owner preflight, cross-process run-owned lease, phân trang history và Stop terminal. Controlled acceptance-response-loss đạt `SIMULATED`; daemon AO hiện dừng nên không tạo session disposable bằng cách khởi động lại daemon dùng chung. Live provider/daemon recovery vẫn `NOT OBSERVED`.

[Cancellation/preflight continuation](../evidence/run-20260925-phase7d-recovery-hardening-continuation/summary.md) sửa AO session envelope, khóa effort/branch/artifact worktree, recover-before-interrupt cho lost acceptance và trạng thái Stop chưa xác nhận. 36 top-level tests cùng race/vet đạt; proof AO session disposable vẫn `LIVE NOT OBSERVED` vì không có daemon cô lập đang chạy và daemon dùng chung đang dừng.

[7D recovery LIVE continuation](../evidence/run-20260925-phase7d-stop-live-recovery/summary.md) chứng minh AO/provider thật nhận một task read-only qua daemon/project/session disposable; adapter làm mất response sau AO acceptance, wrapper restart/recover-only cùng ID và receipt oracle hoàn tất với ordinary resend=0. Stop vẫn chỉ `SIMULATED`: route interrupt session-wide còn race, nên exact-turn Stop chưa được claim.
| 7E — Tái sử dụng | Cùng prompt/runbook áp dụng repo thứ hai có tài liệu | **`PHASE_7E_PORTABILITY_SELFHOST_PASS`**. Quy trình workforce và executable self-host `recovery.exe` [đã quan sát LIVE](../evidence/run-20260926-phase7e-selfhost-pass/summary.md) trên `agents-coworkers` với Gemini 3.7. Rework thực tế của Worker B đã OBSERVED. Deterministic test amendment trên isolated fixture repo đã ACCEPT. Accepted limitations: provider outage LIVE, daemon restart LIVE, Stop LIVE đều NOT OBSERVED; exact-turn Stop unsupported/fail-closed. |

Lượt self-host đầu tiên trên `agents-coworkers` dừng `BLOCKED_PROFILE_ELIGIBILITY_GPT55`: session orchestrator `gpt-5.5/low` có readback đúng nhưng gateway trả `503 auth_unavailable`, với upstream `402 deactivated_workspace`. Đây không phải 429/quota hoặc bằng chứng toàn bộ pool hết capacity. Không worker nào được dispatch và không có source Phase 7E bị sửa; xem [evidence](../evidence/run-20260925-phase7e-selfhost/summary.md). Continuation dùng profile Gemini high được user duyệt riêng, không phải fallback âm thầm.

Continuation Gemini high dừng `BLOCKED_PROFILE_POOL_UNAVAILABLE_GEMINI_3_8_FLASH_HIGH` trước task graph: catalog/readback đạt, nhưng gateway quan sát chọn đủ 7 credential cho model 3.8 và nhận 4 lượt `403 VALIDATION_REQUIRED` cùng 6 lượt `429 RESOURCE_EXHAUSTED`; xem [evidence](../evidence/run-20260926-phase7e-gemini-continuation/summary.md). Claim chỉ áp dụng `gemini-3.8-flash-high`, không suy toàn bộ Gemini/provider unavailable. Không retry nóng hoặc đổi profile trong run đó; Phase 7E vẫn chưa có portability proof.

Bounded resolution ban đầu ghi nhận candidate `gemini-3.7-flash-high/high` là `GEMINI_3_7_DIRECT_PASS__AO_TOOL_LOOP_FAIL` (xem [evidence](../evidence/run-20260926-phase7e-profile-resolution/summary.md)).

Continuation ban đầu với gateway config đã hiệu chỉnh đạt AO tool-loop trên `gemini-3.7-flash-high/high` và đã dispatch workload self-host. Worker A và B có commit được orchestrator ACCEPT sau rework thật ở B; integration worker C cherry-pick sạch bốn commit nhưng `go test -count=1 ./...` FAIL do test 7D `TestGitArtifactReaderBindsHashAndHead` hardcode branch `phase7d-recovery-prototype`, không portable sang AO worktree (`BLOCKED_AUTHORITY_TEST_PORTABILITY_DISPATCHER_TEST`; xem [evidence blocked](../evidence/run-20260926-phase7e-integration-blocked/summary.md)).

Continuation tiếp theo đã hoàn tất sửa test portability bằng fixture repo cô lập trong `t.TempDir()`, orchestrator review ACCEPT commit `e809583`, build thành công binary `recovery.exe` và hoàn tất self-host smoke LIVE trên AO session disposable với profile Gemini 3.7 (`PHASE_7E_PORTABILITY_SELFHOST_PASS`; xem [evidence selfhost pass](../evidence/run-20260926-phase7e-selfhost-pass/summary.md)). Toàn bộ Exit Criteria của Phase 7 đã hoàn thành; Phase 7 chuyển sang trạng thái **COMPLETE** với các accepted limitations: provider outage recovery LIVE, daemon restart recovery LIVE, và Stop LIVE đều `NOT OBSERVED`; exact-turn Stop trên shared session là unsupported và fail-closed.

**Authority và trạng thái hiện hành:** [7A](../evidence/run-20260924-phase7a-native-pilot/summary.md) là review-only với profile GPT-6-Astra/low. Pilot 7B–7C trên AO fixture dùng GPT-5.5/low; review/integration/test được quan sát theo [continuation evidence](../evidence/run-20260924-phase7bc-integration-continuation/summary.md), không phát lại A/B. [7D live](../evidence/run-20260924-phase7d-wakeup-proof/summary.md) xác nhận session-level native resume/explicit wake; [prototype](../evidence/run-20260925-phase7d-recovery-prototype/summary.md) xác nhận logic checkpoint/recovery bằng test cô lập. Rework thực tế ở Worker B và repo portability cùng executable self-host LIVE đã OBSERVED tại [7E](../evidence/run-20260926-phase7e-selfhost-pass/summary.md). Provider failure/daemon restart recovery và Stop LIVE giữ trạng thái NOT OBSERVED. Phase 7 đạt COMPLETE với accepted limitations đã ghi rõ; Product/CP2A giữ nguyên, P9/M3/Phân hệ A khóa.

**Checkpoint có ý nghĩa:** thay contract/scope; tích hợp thay đổi quan trọng; kết thúc capability proof; bàn giao. Debug/test/đọc trong gói authority đã cấp không đòi user duyệt lại từng lệnh. Nếu API vẫn phát approval, xử lý theo delegated scope và policy trong [06](06-OPERATIONS.md), không tắt cơ chế approval.

**Exit:** các oracle workflow ở [05](05-VERIFICATION.md) có evidence, giới hạn được công bố, đầu ra có code/test/review/integration và recovery. P8 UI hoặc sản xuất video hoàn tất không thuộc exit này.

### Phase 8: Lộ trình thương phẩm hóa workforce (DESIGN_PHASE — IMPLEMENTATION_NOT_AUTHORIZED)

> [!IMPORTANT]
> **Trạng thái phê duyệt**: Phase 8 hiện ở cấp độ **thiết kế kiến trúc và quy chuẩn governance (DOCS-ONLY)**. Mọi hoạt động viết mã nguồn (`cmd/coworkers`, `internal/workforce/...`), chạy runtime, gọi provider LLM, tạo AO session hoặc worktree Product đều **CHƯA ĐƯỢC CẤP QUYỀN (IMPLEMENTATION NOT AUTHORIZED)**.

Mục tiêu của Phase 8 là hoàn thiện ranh giới sản phẩm của `agents-coworkers`: chuyển đổi từ tập hợp script/proof rời rạc sang một hệ thống workforce tự vận hành trên dự án thật, với một entrypoint CLI duy nhất, task graph bền vững, khả năng quan sát định tuyến dựa trên telemetry an toàn, và cô lập triệt để đối với mã nguồn sản phẩm. Lộ trình Phase 8 gồm 5 slice triển khai tuần tự:

#### Slice 8A: Control Surface & Preflight (`cmd/coworkers` foundation)
- **Mục tiêu**: Thiết lập giao diện điều khiển dòng lệnh mỏng `coworkers`, cơ chế discovery daemon/gateway linh hoạt, kiểm tra sức khỏe môi trường (doctor), gắn kết phiên hiện hữu (attach), báo cáo trạng thái (status), và quản lý run lease.
- **Ranh giới điều khiển**:
  * AO Endpoint Discovery Policy: Endpoint của Agent Orchestrator được phân giải theo thứ tự ưu tiên nghiêm ngặt (nguyên tắc fail-closed):
    1. Cờ lệnh tường minh: `--ao-url <URL>` (bắt buộc host loopback `localhost`, `127.0.0.1`, hoặc `::1`; nếu truyền URL không hợp lệ hoặc không phải loopback thì fail-closed ngay lập tức, không fallback; xác minh `GET /api/v1/identity`; không dùng OS port inspection để suy PID; ghi nhận `aoDiscoverySource: "explicit_url"`, `aoPid: 0`, `aoPidStatus: "NOT_OBSERVED"`);
    2. Biến môi trường: `AO_BASE_URL` (bắt buộc host loopback; nếu tồn tại nhưng không hợp lệ hoặc không phải loopback thì fail-closed, không fallback; xác minh `GET /api/v1/identity`; ghi nhận `aoDiscoverySource: "environment_url"`, `aoPid: 0`, `aoPidStatus: "NOT_OBSERVED"`);
    3. Biến môi trường: `AO_RUN_FILE` (đường dẫn tệp run metadata, ví dụ `running.json`, không phải URL; parse `running.json`, kiểm tra PID > 0 và tiến trình còn sống, tạo loopback URL từ port, xác minh `GET /api/v1/identity`; ghi nhận `aoDiscoverySource: "run_file"`, `aoPid`, `aoPidStatus: "VERIFIED"`);
    4. Các tệp candidate mặc định: `~/.ao/dev/running.json`, `~/.ao/running.json` (chỉ quét khi không có cả ba nguồn ưu tiên trên; quy tắc xác minh tương tự `AO_RUN_FILE`);
    5. Quy tắc PID: `/api/v1/identity` chỉ trả `hostId` và `apiVersion` (hoặc `contractVersion`), không trả PID. Chỉ có `running.json` mới chứa PID. Do đó, chỉ ghi nhận `aoPidStatus: "VERIFIED"` khi endpoint đến từ `running.json`. Tuyệt đối không suy PID bằng OS port inspection khi URL được cấp trực tiếp;
    6. Nếu quét các candidate mặc định mà phát hiện nhiều hơn một daemon hợp lệ đang chạy thì **dừng lại ngay và báo lỗi (fail-closed, exit 2)**, không tự động chọn daemon; nếu không tìm thấy daemon hợp lệ nào: exit 3; cổng 3005 không phải là cổng kiến trúc cố định. Nếu endpoint hợp lệ nhưng không kết nối được hoặc timeout: exit 3. Nếu sai lệch parse/schema/identity/PID: exit 2.
    Cổng gateway CLIProxyAPI được phân giải theo: (1) `--gateway-url`; (2) `COWORKERS_GATEWAY_URL`; (3) mặc định lịch sử `http://127.0.0.1:8317` (áp dụng cùng nguyên tắc fail-closed: nếu URL tường minh hoặc biến môi trường không hợp lệ thì dừng lại ngay, không fallback).
  * Gateway Authentication & Probe:
    Thăm dò catalog gateway sử dụng `GET /v1/models`. Khi endpoint yêu cầu xác thực, khóa gateway được lấy từ biến môi trường `CLIPROXY_KEY` trong process environment. Tuyệt đối không đưa khóa vào RunSpec, RunManifest, argv, stdout, stderr hoặc evidence; không ghi raw Authorization header; thiếu credential bắt buộc hoặc 401/403: exit 2; không kết nối được hoặc timeout: exit 3; gateway trả catalog/JSON không hợp lệ: exit 2.
  * Phân tách Hợp đồng Dữ liệu (RunSpec vs RunManifest - chuẩn JSON, dùng Go standard library):
    - `RunSpec`: Input do user cung cấp, bất biến sau khi bắt đầu run; phân biệt rõ `targetRoot` (checkout gốc Product) và `executionWorkspace` (AO worktree); `expectedBranch` sử dụng placeholder `<AO_SESSION_BRANCH>` (đại diện cho nhánh AO session/worktree dự kiến); chứa baseline SHA, profiles, policies và authority (`schemaVersion: "run-spec/v1-draft"`). Tuyệt đối không chứa `aoUrl` hay `gatewayUrl`; việc phân giải endpoint được định nghĩa qua `endpointPolicy` (không chứa URL). Chứa chính sách công cụ `requiredTools` (`git` luôn bắt buộc cho Slice 8A; các công cụ khác chỉ kiểm khi khai báo, kiểm tra trực tiếp qua `exec.LookPath`, tên công cụ không chứa path hoặc shell metacharacters; Go/Node không phải runtime prerequisite mặc định của target repo) và chính sách workspace `workspacePolicy` (`productRootMustBeClean: true`, `executionWorkspaceDirtyPolicy`: `require_clean` [mặc định] hoặc `allow_dirty_recorded`).
    - `RunManifest`: Output do `coworkers attach` sinh ra duy nhất (`schemaVersion: "run-manifest/v1-draft"`); chứa `runSpecSha256`, `generatedBy: "coworkers attach"`, resolved endpoints (aoUrl, aoIdentity, aoDiscoverySource, aoPid, aoPidStatus), `gatewayProbe` (url, catalogPath: "/v1/models", catalogStatus: "VERIFIED", observedModels, `providerCallPerformed: false`, `credentialEligibility: "NOT_OBSERVED"`), target repository identity, baseline SHA, current HEAD, `sessionBranch` và `worktreeBranch` (hai giá trị phải khớp nhau), `worktreeBinding` (canonicalPath, worktreeBranch, head, isClean, dirtyPolicy, porcelainSha256, verifiedPorcelain), đối tượng cấu hình duy nhất `attachedSessionProfile` (kind, harness, model, reasoningEffort, status), `lease` (workspaceRoot, runOwner, taskId: "__run__", ownerId, observedState: "FREE", observedPid: 0; không có trạng thái ACQUIRED), source provenance, và timestamps. Không chứa secret (token, cookie, email, raw credential filename) và không chứa endpoint input dư thừa.
    - Vòng đời thống nhất:
      * `coworkers doctor`: Chuẩn hóa cú pháp: `coworkers doctor --spec <run-spec.json> [--ao-url <url>] [--gateway-url <url>] [--json]`. Cờ `--spec` là bắt buộc (thiếu, hỏng hoặc sai schema RunSpec: exit 2). Lệnh `doctor` không nhận cờ `--manifest`, không tạo hoặc sửa `RunManifest`, và không inspect hay acquire lease. Kiểm tra gateway reachable, `CLIPROXY_KEY` được chấp nhận, `/v1/models` trả catalog hợp lệ và model yêu cầu xuất hiện trong catalog (tuyệt đối không đồng nhất catalog với credential eligibility hay usable capacity; `providerCallPerformed: false`, `credentialEligibility: "NOT_OBSERVED"`). Kiểm tra runtime tools theo `requiredTools` (`git` bắt buộc; Go/Node không phải prerequisite mặc định của target repo).
      * `coworkers attach`: Đọc `RunSpec`. Nhận `--session`, `--workspace` và `--manifest` (đây là output path). Kiểm tra chính sách workspace dirty (`productRootMustBeClean: true`, Product root dirty luôn exit 2; `require_clean` + dirty worktree: exit 2; `allow_dirty_recorded` + dirty worktree: PASS và ghi `porcelainSha256` là SHA-256 của `git status --porcelain=v1 -z`). Xác minh session/worktree theo thuật toán 9 bước bắt buộc. Gọi API inspect read-only kiểm tra lease (`workspaceRoot`, `runOwner`, `taskId: "__run__"`); tuyệt đối không gọi `Acquire` hoặc `Release` vì không có process run dài hạn giữ lease (`FREE`: tiếp tục; `OWNED` cùng run: idempotent readback; `LOCKED`: exit 2). Tạo `RunManifest` bằng cơ chế ghi nguyên tử (atomic write: temp file -> fsync/close -> rename). Nếu manifest chưa tồn tại: tạo mới (lệnh attach tự động tạo mới, không đòi hỏi tệp có sẵn từ trước). Nếu manifest đã tồn tại và khớp hoàn toàn (`runId`, `runSpecSha256`, session, repository và worktree binding): trả về thành công lũy đẳng (idempotent success), không viết lại. Nếu manifest bị hỏng hoặc sai lệch binding: dừng lại ngay (fail-closed) và thoát với exit code `2`.
      * `coworkers status`: Chỉ đọc `RunManifest` và trạng thái live read-only. Luôn re-inspect lease thực tế từ `workspaceRoot`, `runOwner`, `taskId: "__run__"`; tuyệt đối không tin `observedState` cũ trong manifest. Không tự ý phá stale lease, không kiểm PID còn sống để tự phá lease. Nếu manifest thiếu hoặc hỏng: thoát với exit code `2`. Không đọc `RunSpec` để tự tái tạo binding. Không chỉnh sửa manifest. Tuyệt đối không có trường Task DAG nào trong Slice 8A.
    - Cú pháp lệnh: `coworkers doctor --spec <run-spec.json> [--ao-url <url>] [--gateway-url <url>] [--json]`, `coworkers attach --spec <run-spec.json> --session <id> --workspace <path> --manifest <output-path> [--ao-url <url>] [--gateway-url <url>]`, `coworkers status --manifest <path-to-run-manifest.json> [--ao-url <url>] [--gateway-url <url>] [--json]`.
  * `coworkers doctor`: Khám sức khỏe môi trường, runtime tools (`git` luôn bắt buộc; các công cụ khác kiểm tra theo `requiredTools` qua `exec.LookPath` mà không dựng shell command; Go/Node không phải runtime prerequisite mặc định của target repo; invalid tool name fail-closed exit 2), tính hợp lệ của baseline SHA, đường dẫn target root (bắt buộc sạch) và profile catalog. doctor chỉ chứng minh gateway reachable, CLIPROXY_KEY được chấp nhận, `/v1/models` trả catalog hợp lệ và model có trong catalog; không gọi provider completion, không nâng claim credential eligibility hay usable capacity (`providerCallPerformed: false`, `credentialEligibility: "NOT_OBSERVED"`). doctor không tạo manifest và không inspect hay acquire lease.
  * `coworkers attach` & Thuật toán Worktree Binding bắt buộc:
    Tuyệt đối không tin riêng `RunSpec` hoặc `RunManifest`. AO `SessionView` không trả về đường dẫn vật lý của worktree; không được tuyên bố xác minh worktree chỉ bằng GET session. Lệnh `attach` bắt buộc thực thi thuật toán:
    1. Đọc session qua AO API và lấy project ID + branch;
    2. Canonicalize target root và explicit execution workspace path;
    3. Chạy `git worktree list --porcelain -z` trên đúng repository;
    4. Tìm đúng một worktree có branch bằng session branch;
    5. Path canonical của worktree phải bằng execution workspace;
    6. HEAD phải bằng expected HEAD/baseline theo trạng thái run;
    7. Repository common-dir và identity phải khớp target repo;
    8. Không có worktree, nhiều kết quả, detached ngoài contract hoặc mismatch -> lập tức thoát với exit code `2`;
    9. Không đọc `ao.db` và không suy physical path từ `SessionView`.
  * `coworkers status`: Ở Slice 8A, lệnh status **chỉ báo cáo**: `RunManifest`, endpoint identity của daemon/gateway, phiên/profile được gắn kết, workspace/baseline SHA, và quyền sở hữu run lease. Luôn re-inspect lease từ `workspaceRoot`, `runOwner`, `taskId: "__run__"`, không tin `observedState` cũ trong manifest; không phá stale lease. **Tuyệt đối không có trường Task DAG nào** trong Slice 8A; ghi rõ trạng thái Task/DAG chỉ khả dụng từ Slice 8B.
  * Cơ chế Run Lease: `FileLease` theo source thật đặt tại `<executionWorkspace>/.agents-coworkers/recovery-leases`, tên file suy từ SHA-256(`runOwner + "\n" + taskId`). Slice 8A dùng run-level lease identity cố định: `runOwner = RunSpec.runId`, `taskId = "__run__"`, `ownerId = RunSpec.runId`, `workspaceRoot = canonical executionWorkspace`. Trạng thái quan sát được: `FREE` (lease file không tồn tại), `OWNED` (lease tồn tại và khớp run), `LOCKED` (lease thuộc owner/task khác). Tuyệt đối không dùng trạng thái `ACQUIRED` trong manifest Slice 8A. `FileLease` dùng `O_EXCL` để phối hợp độc quyền giữa các process `coworkers`/`recovery` cùng tuân thủ wrapper contract. Nó không phải global AO lock và không ngăn client AO bên ngoài sử dụng session. Lease này **không có TTL** và **không tự động phá bỏ stale lease**. Lệnh status luôn re-inspect trực tiếp từ `workspaceRoot`, `runOwner`, `taskId: "__run__"`, không tin `observedState` cũ trong manifest. Lệnh attach chỉ gọi API inspect read-only, không gọi Acquire hoặc Release.
  * Lệnh `coworkers run`: Bị từ chối thực thi rõ ràng (luôn trả về mã thoát `1` với thông báo chưa hỗ trợ) cho đến khi Slice 8B được nghiệm thu.
  * Ma trận mã thoát chuẩn hóa:
    - `0`: Thành công.
    - `1`: Lỗi CLI usage hoặc lệnh chưa hỗ trợ (`run` luôn trả về mã thoát `1`).
    - `2`: Lỗi kiểm tra / xác thực (validation, identity, baseline, worktree, hoặc lease conflict).
    - `3`: Endpoint không khả dụng hoặc hết thời gian chờ (endpoint unavailable hoặc timeout).
- **Oracle nghiệm thu (Gate 8A)**:
  * Tier đóng gate: **L4 (Local Process Smoke)**. Unit tests (L2) và integration giả lập (L3) chỉ là bằng chứng bổ trợ; Gate 8A không gọi live provider.
  * Lệnh gọi CLI thực thi nhị phân (`coworkers doctor`, `coworkers attach`, `coworkers status`) là oracle bắt buộc.
  * Hợp đồng JSON output ra stdout khi có cờ `--json`, stderr chuẩn an toàn không rò rỉ secret.

#### Slice 8B: Durable Task Graph & Autonomous Loop (1–3 workers)
- **Mục tiêu**: Xây dựng task graph có cấu trúc DAG bền vững, máy trạng thái điều phối vòng lặp tự vận hành (dispatch -> review -> rework -> integration), và chính sách concurrency an toàn.
- **Chính sách Concurrency**:
  * **Mức mặc định tối đa**: Bằng mức tối đa đã được kiểm chứng thực tế tại Gate 10 / Phase 6: **tối đa 3 workers đồng thời**.
  * Scheduler 8B lựa chọn từ **1 đến 3 workers** dựa trên DAG readiness và dung lượng khả dụng thực tế của pool.
  * Mức **4–7 workers** được phân loại là **`EXPERIMENTAL_CAPACITY`**, chỉ được phép kích hoạt khi: (1) telemetry từ Slice 8C cung cấp đủ căn cứ; (2) có gói authority riêng từ user; (3) có chứng minh cô lập tài nguyên/worktree hoàn chỉnh; và (4) thực hiện capacity proof riêng biệt có ghi nhận evidence. Tuyệt đối không mô tả 1–7 như dải năng lực sản xuất mặc định.
- **Cơ chế vòng lặp & Rework Oracle**:
  * Orchestrator phân rã mục tiêu thành các task độc lập có dependency rõ ràng.
  * Worker thực thi trong worktree cô lập.
  * Orchestrator review diff và kết quả test. Nếu phát hiện sai sót, phát chỉ thị rework có ngữ cảnh; worker thực hiện sửa đổi và nộp lại.
  * **Rework Oracle**: Không yêu cầu tạo lỗi nhân tạo trong Product repository. State-machine rework oracle trong kiểm thử Gate 8B sử dụng **fixture cô lập có contract violation được seed trước**. Khi chạy trên workload thật (Slice 8D), orchestrator chỉ phát chỉ thị rework khi phát hiện lỗi kỹ thuật thật sự trong sản phẩm.
  * Barrier tích hợp tuần tự: Chỉ tích hợp commit đã `ACCEPTED` vào branch tích hợp cục bộ của run (`integration/run-<id>`).

#### Slice 8C: Native Telemetry & Routing Observability
- **Mục tiêu**: Khai thác năng lực quan sát định tuyến sẵn có của CLIProxyAPI và AO để phân loại lỗi, quản lý backoff và tối ưu hóa điều phối.
- **Nguồn gốc và Provenance**:
  * Audit mã nguồn CLIProxyAPI được thực hiện trên commit exact: `2430354330af80b645f9ffb1a51e1e7c72c4cc8e`.
  * **Ranh giới chứng minh**: Việc audit mã nguồn không chứng minh binary runtime đang chạy trên máy được biên dịch từ đúng commit này.
- **Quy tắc Telemetry an toàn**:
  * Header `X-CPA-TRACE-ID`: Có định dạng `<YYYYMMDDHHMMSS>-<authIndex>-<requestID>`. Trong đó `authIndex` là định danh giả danh (pseudonymous) 16-hex được tạo từ SHA-256 seed của credential. Chuỗi này có thể liên kết nhiều request cùng một tài khoản (correlation risk), không chứa raw token nhưng vẫn là **metadata vận hành nhạy cảm**. Bắt buộc phải áp dụng chính sách lọc/redaction và lưu trữ có giới hạn (bounded retention); **không được coi là an toàn tuyệt đối**.
  * Header có thể vắng mặt khi chưa chọn được credential hoặc đường dẫn response không đi qua trace callback; khi vắng mặt bắt buộc phải ghi nhận là `NOT_OBSERVED` hoặc `UNKNOWN`.
  * Header `Retry-After`: Chỉ được sử dụng khi thực sự xuất hiện trong response headers từ gateway/upstream. Không giả định mọi lỗi 429 hoặc 503 đều có `Retry-After`. Khi vắng mặt, áp dụng backoff lũy thừa mặc định có jitter và cận trên.
  * Phân loại lỗi HTTP: Tuyệt đối không tự động ánh xạ mọi mã 402 thành `deactivated_workspace` hay mọi mã 403 thành `VALIDATION_REQUIRED` (đây chỉ là các subcode quan sát được ở một số run lịch sử). Bộ phân loại lỗi phải kết hợp HTTP status code, structured error code/reason trong JSON body và response headers. Thiếu dữ liệu thì phân loại là `UNKNOWN`.
  * Thứ tự ưu tiên giải pháp: (1) Sử dụng API và headers native sẵn có; (2) Xây dựng client adapter an toàn; (3) Đề xuất bản vá header (`X-CPA-Failover-Attempts`) chỉ là phương án ứng viên sau khi có audit riêng và được duyệt. Không đề xuất patch là hướng mặc định.

#### Slice 8D: Workload Execution on Real Product Target
- **Mục tiêu**: Thử nghiệm vận hành workforce trên workload thật của `AI Auto Video Creator`.
- **Ranh giới Sản phẩm và D010**:
  * Thư mục gốc (`Product ROOT checkout`) của `AI Auto Video Creator` **luôn luôn ở chế độ CHỈ ĐỌC (READ-ONLY)** theo `INV-001`.
  * Các git worktree cô lập được cấp quyền cho run có thể thực hiện ghi theo danh mục allowlist cụ thể của từng run. Không diễn giải `INV-001` thành cấm mọi Product worktree.
  * Vòng tích hợp tự động của 8D **dừng lại ở branch tích hợp cục bộ (`integration/run-<id>`)**. Tuyệt đối không tự động merge hoặc push vào branch `main` của Product repository.

#### Slice 8E: Capacity Expansion & Reliability (CONDITIONAL SLICE)
- **Tính chất**: Slice 8E là **nhánh phụ thuộc điều kiện (conditional slice)**, không phải yêu cầu bắt buộc để đóng Phase 8.
- **Tiêu chí đánh giá**:
  * Nếu telemetry từ Slice 8C và 8D chứng minh pool hiện tại (1 Codex + 7 Gemini) đủ năng lực đáp ứng workload với tỷ lệ lỗi thấp: trạng thái Slice 8E chuyển thành **`NOT_TRIGGERED`** hoặc **`NOT_REQUIRED_WITH_EVIDENCE`**, không tiến hành onboarding thêm tài khoản, và Phase 8 vẫn có thể đóng hợp lệ.
  * Nếu phát hiện thiếu hụt dung lượng thực tế qua telemetry (429 quota liên tục, hàng đợi task tắc nghẽn): lập đề xuất xin authority riêng từ user để thực hiện onboarding tài khoản mới và chạy capacity proof.
- **Nguyên tắc**: Số lượng tài khoản trong inventory lịch sử không đồng nghĩa với usable capacity khả dụng. Mục tiêu danh nghĩa ban đầu (6 Plus + 8 Pro) chỉ là mục tiêu lịch sử đã superseded; không đặt ra chỉ tiêu số lượng tài khoản cứng để làm điều kiện đóng phase.

#### Tiêu chí hoàn thành Phase 8 (Exit Criteria)
1. Slice 8A đến 8D hoàn thành và vượt qua các Gate kiểm chứng tương ứng (Gate 8A–8D).
2. Slice 8E được đánh giá: hoàn tất nghiệm thu capacity proof HOẶC được xác nhận `NOT_TRIGGERED_WITH_EVIDENCE` dựa trên dữ liệu telemetry chứng minh pool hiện hữu đủ đáp ứng. Onboarding tài khoản mới không phải điều kiện tiên quyết bắt buộc.
3. Không vi phạm bất biến kiến trúc: Product root checkout nguyên vẹn và sạch; không push trái phép lên Product main; không rò rỉ credential secret trong telemetry hay evidence.

#### Exact Authority Delta cho Implementation 8A (Chưa thực hiện)
Nhằm chuẩn bị cho lượt xin cấp quyền thực thi mã nguồn tiếp theo, phạm vi thay đổi mã nguồn dự kiến của Slice 8A được giới hạn chính xác trong 17 tệp sau (tuyệt đối không dùng ký tự đại diện hay quy định tương đương):
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
- `internal/recovery/lease.go` (giới hạn: chỉ bổ sung API inspect read-only nhận workspaceRoot, runOwner, taskId cần cho status và attach; không đổi Acquire/Release semantics; không TTL, không auto-break stale lease)
- `internal/recovery/lease_test.go`

Ranh giới kỹ thuật:
- Không sửa đổi `go.mod` và `go.sum`. Chỉ sử dụng thư viện chuẩn của Go (Go standard library).
- Tuyệt đối không triển khai task DAG execution, scheduler, worker dispatch, can thiệp Product root, gọi provider workload LLM, onboarding tài khoản, git merge hoặc push lên remote trong Slice 8A.

#### Behavior Matrix 8A
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
   - Kiểm tra workspace dirty policy: Product root dirty luôn exit code 2; `require_clean` + dirty worktree: exit code 2; `allow_dirty_recorded` + dirty worktree: PASS và ghi `porcelainSha256` (SHA-256 của `git status --porcelain=v1 -z`); không tự suy mọi dirty state là "expected";
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
