# 02 — Kiến trúc và capability gap

## 1. Ownership và đường thực thi

```text
Mục tiêu + tài liệu target repo + authority
  → orchestrator (AO / Codex / model + effort được user chọn theo run)
  → task/dependency + ownership → Gemini workers (AO / Codex / gemini-3.8-flash-high / high)
  → kết quả qua AO → orchestrator review → rework nếu có lỗi thật, hoặc tích hợp tuần tự → test → bàn giao
                         ↓ mọi request model
                  CLIProxyAPI → credential đủ điều kiện → provider
```

| Thành phần | Trách nhiệm | Không được đồng nhất |
|---|---|---|
| AO | Project, role/session, delegation/message, conversation/status, worktree, review surface và lifecycle | Spawn worker không tự chứng minh planning/scheduling toàn workflow |
| Codex | Tool-loop đọc/sửa/chạy theo sandbox/approval của session | Không tự cấp quyền hoặc quản lý pool |
| CLIProxyAPI | Catalog/translation, chọn credential, affinity, cooldown, retry/failover | Không quản lý task graph, Git integration hoặc side effect của tools |
| agents-coworkers | Nối thành vòng điều phối, contract ownership, acceptance, recovery và tính năng còn thiếu có căn cứ | Không chỉ là config; không viết lại các primitive upstream |
| Target repo | Source, contract, test và acceptance sản phẩm; AI Video Creator là một ví dụ | Không phụ thuộc runtime vào workforce; P8 không phải workforce gate |

## 2. Source đối chiếu và mức chứng minh

Các link AO dưới đây đối chiếu checkout `1140dd62dc7bb588b987e2c44aa1ff4796fa732b`; CLIProxyAPI `2430354330af80b645f9ffb1a51e1e7c72c4cc8e`. Đây là source audit, không chứng minh binary gateway được build từ SHA đó. Bằng chứng runtime và giới hạn tập trung tại [05](05-VERIFICATION.md).

| Capability | Exact source | Phân loại và giới hạn |
|---|---|---|
| Role/session và delegation | [sessions.go](../../agent-orchestrator/backend/internal/httpd/controllers/sessions.go), [dto.go](../../agent-orchestrator/backend/internal/httpd/controllers/dto.go), [delegation.go](../../agent-orchestrator/backend/internal/service/session/delegation.go): `DelegateTask` | Có native/source; Gates 7–8 có runtime role/tool-loop. `POST /api/v1/orchestrators/delegate` spawn worker với model/effort override; title refinement gửi orchestrator sau spawn, **không phải** tự phân rã mục tiêu. |
| Giao việc/kết quả/review | [cli/send.go](../../agent-orchestrator/backend/internal/cli/send.go): `sendMessage/steerMessage` | Native; Gate 9 có review/rework component. Continuation fixture 7B–7C dùng GPT-5.5/low làm orchestrator, nhận reply A/B/C qua AO và ACCEPT ba diff/test; không có user relay. Rework thực tế `NOT OBSERVED`; đây chưa phải proof tự vận hành trên workload sản phẩm. |
| Status và durable conversation | [httpd/events.go](../../agent-orchestrator/backend/internal/httpd/events.go), [conversations.sql](../../agent-orchestrator/backend/internal/storage/sqlite/queries/conversations.sql), [domain/conversation.go](../../agent-orchestrator/backend/internal/domain/conversation.go) | Có source CDC replay từ `change_log`, conversation/turn/plan persisted. `ConversationPlanStep` là step/status, không phải contract DAG/ownership/resource lease có enforcement. |
| Worktree và review | [gitworktree/workspace.go](../../agent-orchestrator/backend/internal/adapters/workspace/gitworktree/workspace.go), [workspace_review.go](../../agent-orchestrator/backend/internal/service/session/workspace_review.go), [cli/review.go](../../agent-orchestrator/backend/internal/cli/review.go) | Gate 10 chứng minh 3 worktree; pilot 7B–7C chứng minh trên fixture với orchestrator GPT-5.5/low: worker C cherry-pick A/B đã ACCEPT và test chung 8/8. Rework thực tế, merge/push và tích hợp workload sản phẩm chưa được chứng minh. |
| Quota routing | [selector.go](../../agent-orchestrator/CLIProxyAPI/sdk/cliproxy/auth/selector.go): `RoundRobinSelector.Pick/SessionAffinitySelector.Pick`; [conductor_selection.go](../../agent-orchestrator/CLIProxyAPI/sdk/cliproxy/auth/conductor_selection.go) | Có source + UNIT/SIMULATED; LIVE_ROUTING chỉ subset. Affinity giữ credential còn eligible; unavailable chọn lại qua fallback, không round-robin bắt buộc từng request. |
| Retry/cooldown | [conductor_execution.go](../../agent-orchestrator/CLIProxyAPI/sdk/cliproxy/auth/conductor_execution.go): `Execute`; [antigravity_executor_credits.go](../../agent-orchestrator/CLIProxyAPI/internal/runtime/executor/antigravity_executor_credits.go): `decideAntigravity429` | Source đọc reason/RetryDelay, phân biệt full quota/short cooldown/soft retry; request-scoped stop có thể chặn retry. Không suy nguyên nhân từ 429 đơn lẻ; LIVE_FAILOVER chưa quan sát. |
| Recovery delivery/turn | [cli/send.go](../../agent-orchestrator/backend/internal/cli/send.go), [chat/controller.go](../../agent-orchestrator/backend/internal/service/chat/controller.go): `RetryTurn`, `afterProject`, `drainLocked` | LIVE trên fixture: `exit-agent` → `resume-agent` native giữ session/worktree; delivery mới đánh thức session idle. T1/T2 bị `/conversation/interrupt` là expected cancellation theo lệnh user/coordinator, không phải phép thử provider recovery. Provider failure, daemon restart và unattended recovery vẫn `NOT OBSERVED`. |

## 3. Phần nối cần hoàn thiện, không dựng framework mới

Phân loại audit bắt buộc: **(1) có source và runtime evidence**, **(2) có source/config nhưng chưa proof runtime**, **(3) cần cấu hình/kết nối**, **(4) thiếu capability thật, cần code**. Bảng §2 ghi (1)/(2); task contract và đường nối dưới đây thuộc (3). Chưa có đủ bằng chứng để chốt một patch thuộc (4): schema plan thiếu DAG không tự chứng minh toàn AO thiếu mọi cách biểu diễn dependency. Capability audit 7A phải loại trừ native seam trước khi đưa exact code delta; không đổi NOT OBSERVED thành kết luận “phải viết framework”.

**Pilot native 7A (24/09/2026):** binary AO đang chạy (`backend/ao.exe`, SHA-256 `dce49a699c848761a4f23d28a7e6f7218ab6530345062c99b6d356c63725b371`) phục vụ `POST /api/v1/orchestrators/delegate`, conversation/send và per-session profile. Run 7A dùng lịch sử **GPT-6-Astra/low** cho orchestrator; Astra tự dispatch ba review task, nhận ba native reply, đọc artifact và đưa verdict ([evidence](../evidence/run-20260924-phase7a-native-pilot/summary.md)). Đây là review-only, không là run 7B–7C.

**Continuation 7B–7C:** đúng orchestrator fixture `ao-phase5-repo-1` được readback **GPT-5.5/low**; không gọi profile này là Astra. Hai worker code A/B chạy song song, được orchestrator ACCEPT; C tích hợp tuần tự và combined unittest 8/8 PASS ([evidence](../evidence/run-20260924-phase7bc-integration-continuation/summary.md)). Rework thực tế `NOT OBSERVED`. Profile mới không chứng minh tình trạng quota của GPT-6-Astra.

**Existing:** AO lưu session/worktree binding và conversation messages/turns/plans trong SQLite dưới AO data dir; CLIProxyAPI sở hữu credential/cooldown. Không ghi trực tiếp DB của AO hoặc tạo nguồn session status cạnh tranh.

**Cần cấu hình/kết nối:** prompt orchestrator ở [06](06-OPERATIONS.md) phải yêu cầu task record trong conversation: task ID, dependency, owner, file allowlist, resource ownership, session/worktree, revision/hash, test receipt, review verdict, next action. Worker trả artifact qua AO; orchestrator đọc diff/test, không chỉ tin summary. Bản tóm tắt/handoff phải trỏ về durable record và artifact, không biến chat volatile thành nguồn duy nhất.

**Gap contract:** source plan step chưa có structured DAG, file/resource ownership và integration receipt cho cả workflow. Đề xuất trước mắt lưu record có cấu trúc trong conversation qua API native; semantic enforcement do orchestrator thực hiện và phải kiểm chứng. Nếu không đủ durability/query/enforcement, đề xuất delta integration tối thiểu với schema/owner/idempotency/oracle, không tự thêm DB/framework.

**Gap recovery (7D):** live proof xác nhận `exit-agent`/`resume-agent` native tiếp tục cùng session C và delivery mới đánh thức session đang idle; không restart daemon. T1/T2 bị `/conversation/interrupt` nên kết quả `interrupted` là expected cancellation, không chứng minh thiếu auto-wake. T3 gửi rõ ràng sau interruption chạy `running → completed`; đây chỉ là explicit wake. Provider failure, daemon restart, pool exhaustion và unattended recovery vẫn `NOT OBSERVED`.

**Prototype 7D recovery:** native `RetryTurn` tái phát durable prompt thành turn mới và phù hợp cho retry do người dùng yêu cầu, không tự phân biệt outage với Stop. `steer-or-send` có durable `clientMessageId` và `recoverOnly`, giải quyết delivery uncertain nhưng không lập lịch retry theo `nextRetry`. Transition-message dispatcher của AO có durable outbox/poll riêng cho interface transition, không phải API tổng quát cho workload. Delta nhỏ nhất được chọn là dispatcher giới hạn trong `agents-coworkers`: checkpoint task/delivery/session/turn/artifact/side-effect/retry; write-ahead `DELIVERY_UNCERTAIN` trước provider I/O; đọc AO và Git trước send; recover-only với cùng ID; không gửi khi blocked/completed/human Stop. AO HTTP adapter chỉ dùng loopback API chính thức. AO turn completed chưa đủ đóng task: reply phải có receipt đúng task ID, accepted và artifact SHA. Không patch AO/CLIProxyAPI.

Hardening tiếp theo giữ nguyên trạng thái `DELIVERED`/`DELIVERY_UNCERTAIN` khi Observe tạm lỗi, phân trang conversation tới khi tìm đúng turn hoặc hết lịch sử authoritative, và fail-closed nếu `steer-or-send` trả `steered`. Send mới chỉ được phép sau preflight session idle/owner và trong lease file độc quyền của wrapper run-owned; lease này không được diễn giải thành global lock đối với mọi AO client. Human Stop được ghi `CANCELLED` trước lệnh interrupt và luôn terminal. Các contract này đã được kiểm bằng test cô lập; live disposable-session proof còn `NOT OBSERVED` khi daemon AO dùng chung đang dừng.

Preflight dùng đúng envelope AO `{session:{...}}`, khóa session/project/kind/harness/model/branch và lấy effort từ `conversation.settings.reasoningEffort`; branch AO phải bằng branch Git của artifact. Stop sau lost acceptance dùng recover-only cùng `clientMessageId`, chỉ interrupt khi đúng recovered turn là live turn duy nhất. Nếu không xác nhận được AO đã dừng, checkpoint giữ terminal `CANCELLED_LOCAL_AO_STOP_UNCONFIRMED`; gọi `Step` không thể wake/send, còn một lệnh `Cancel` tường minh sau restart mới được phép đối chiếu lại. Lease path được suy từ workspace root cố định + run owner + task ID, không nhận path tùy ý và không tự phá stale lease.

Giới hạn Stop: AO `POST /conversation/interrupt` là session-wide và không nhận expected turn ID; Observe/precheck rồi POST có race. Prototype chỉ cho phép gọi route này khi checkpoint tuyên bố session disposable thuộc run và độc quyền. `SessionExclusive` là assertion/lease của wrapper run-owned, không phải khóa toàn cục ngăn AO client khác; vì vậy không claim exact-turn Stop trên session chia sẻ. Delta AO tối thiểu nếu cần bảo đảm triệt để là `POST /sessions/{sessionId}/conversation/turns/{turnId}/interrupt` với expected controller generation; service phải kiểm target vẫn là active turn dưới cùng controller lock và trả `409 CHAT_TURN_NOT_ACTIVE` khi fence lệch.

**Portability và Executable Self-Host (7E):**
Phase 7E hoàn tất quy trình workforce và tự vận hành binary `recovery.exe` trên repository thứ hai `agents-coworkers`:
- Worker B ghi nhận rework thực tế (OBSERVED): bổ sung matrix mã thoát quá trình (exit codes 0, 1, 2, 3), kiểm thử subprocess CLI và xử lý timeout context.
- Worker C giải quyết dứt điểm rào cản test portability: sửa `TestGitArtifactReaderBindsHashAndHead` trong `internal/recovery/dispatcher_test.go` dùng fixture repo cô lập trong thư mục tạm thay cho kỳ vọng cứng branch tĩnh, giữ nguyên oracle xác minh hash, HEAD và branch không rỗng.
- Executable `recovery.exe` được biên dịch trực tiếp từ mã nguồn đã tích hợp, thực thi preflight kiểm tra session identity/model/effort/branch, gửi task tới worker disposable qua AO API, bóc tách `TASK_RECEIPT` và chuyển checkpoint thành `COMPLETED` (Exit 0 LIVE).
- Giới hạn chấp nhận: provider outage recovery LIVE, daemon restart recovery LIVE, và user Stop LIVE đều `NOT OBSERVED`; exact-turn Stop trên shared session unsupported/fail-closed.

**Lịch sử gián đoạn 7B:** pilot ban đầu có Codex auto-review/AO turn `503`; continuation dùng GPT-5.5/low theo lựa chọn run và hoàn tất audit. Không suy ra GPT-6-Astra hết quota. Lượt auto-review bổ sung của worker B không được tính test evidence; [evidence pilot ban đầu](../evidence/run-20260924-phase7bc-native-code-pilot/summary.md) và [continuation](../evidence/run-20260924-phase7bc-integration-continuation/summary.md) giữ riêng lịch sử với kết quả cuối.

Luồng phục hồi đề xuất: ghi checkpoint task + delivery ID + hash artifact + side effects đã xác nhận → chờ theo Retry-After và budget → đọc lại AO turn/process/artifact → recover delivery cũ nếu uncertain → chỉ phát next action chưa hoàn tất. Hết budget thì giữ trạng thái chờ có reason/next retry, không tạo session/worktree mới để né lỗi và không replay mù thao tác không idempotent.

## 4. Cách ly và integration

Worktree cách ly file nhưng Git metadata dùng chung. Port/DB/schema/container/temp/certificate phải có run/task owner, namespace riêng và cleanup kiểm ownership/liveness. Shared files chỉ có một writer hoặc được chỉnh tuần tự theo dependency. Orchestrator theo profile của từng run giữ integration ownership; chỉ tích hợp diff đã review, theo base/hash đã ghi, rồi test chung. Quyền commit/merge phải được cấp riêng; read-only review artifact không được gọi là integration proof.

## 5. Profile và đường cấu hình

Path A vẫn ưu tiên: UI/API daemon chính thức, per-session model/effort và readback; không suy `high` từ default `low`. Config mẫu hiện tại là baseline lịch sử, không áp dụng đè defaults. Profile/giới hạn ở [01](01-PROJECT-CHARTER.md), thao tác ở [06](06-OPERATIONS.md).

Path B: [patch effort CLI tùy chọn](../patches/agent-orchestrator/0001-cli-support-agent-effort.patch) khắc phục CLI mirror thiếu effort; không được áp dụng chỉ vì sửa docs. Nếu native/config không đủ một capability cần thiết, ghi gap, exact source seam, scope/verification và authority cho patch nhỏ.

Candidate B/C (AO native `backend/internal/adapters/agent/opencode/`, `agy/`) vẫn dormant: chỉ xét khi Candidate A có lỗi kiến trúc tái hiện được, không vì quota hoặc lỗi setup.
