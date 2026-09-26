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
| **Phase 7** | Vòng workforce tự vận hành trên workload có giới hạn | **COMPLETE** | 7A pilot; 7B/7C code-review-integration; 7D delivery recovery LIVE; 7E repo portability và executable self-host LIVE với Worker B rework OBSERVED; accepted limitations: provider outage LIVE, daemon restart LIVE, Stop LIVE đều NOT OBSERVED, exact-turn Stop trên shared session unsupported/fail-closed |
| **Phase 8** | Capacity & reliability | **PARTIAL_EVIDENCE — onboarding STOPPED** | Giữ pool hiện có; chứng minh reliability theo rủi ro, full pool không chặn sử dụng |

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
- **Objective**: Prove concurrent multi-agent execution at the initial runtime verification target (3 workers). Target worker range is 3–7; maximum proven live concurrency: 3 (verified at runtime in Gate 10).
- **Entry Criteria**: Phase 5 complete.
- **Required Work**: Execute Gate 10 (parallel 3-worker wave).
- **Exit Criteria**: 3 live workers execute in parallel across separate git worktrees without race conditions, token starvation, or session crashes (VERIFIED: maximum proven live concurrency = 3).
- **Evidence Required**: Parallel session execution logs and worktree integrity verification.
- **Explicit Non-Goals**: Do not claim 5 or 7 workers until Phase 8 (Capacity Expansion).

### Phase 7: Vòng workforce tự vận hành (COMPLETE)

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

### Phase 8: Capacity & reliability (PARTIAL_EVIDENCE)

- Pool hiện giữ **1 Codex + 7 Gemini**; onboarding đã dừng theo user. Mục tiêu 6+8 là tùy nhu cầu, không prerequisite workforce.
- [Evidence onboarding](../evidence/run-20260923-phase8-pool-onboarding/summary.md): 14/14 request Gemini fixture trả 200, 5 credential được quan sát chọn và thành công; hai Gemini còn lại chưa đủ evidence loaded/eligible/successful. Codex registered=1, các mức per-model còn lại chưa quan sát trong run đó.
- Source/unit mô phỏng selection/affinity/retry đã có; LIVE_FAILOVER, quota remaining, gateway binary-source correspondence chưa chứng minh. Bốn 429 cũ giữ UNKNOWN, không cần phân loại xong mới tiến hành công việc độc lập.
- Reliability thực thi theo 7D; không cố đốt quota tạo 429, không sửa cooldown hoặc mua credits. Thiếu live event ghi NOT OBSERVED, dùng phép mô phỏng cô lập có nhãn.
- 5–7 workers chỉ mở khi có task độc lập, tài nguyên và scope/phép thử tương ứng. Số account không quyết định số worker. Maximum proven live concurrency vẫn 3.
- Không chạy onboarding/smoke pool trong lượt docs-only. Phase 8 không COMPLETE vì inventory đủ số; exit gắn capacity/recovery thực sự được triển khai và kiểm chứng.
