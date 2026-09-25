# 06 — Operational Runbook

Runbook authoritative để sử dụng workforce. Các lệnh setup bên dưới chỉ áp dụng khi thiếu prerequisite và đã có authority; không phải yêu cầu relogin/smoke/relaunch ở mọi lượt.

## 0. Luồng người dùng và ranh giới hiện hành

1. Khởi động/reuse AO và gateway theo môi trường đã được duyệt; discovery đúng daemon. Phiên đang khỏe thì giữ CODEX_HOME/provider/defaults.
2. Chọn repository có AGENTS.md/tài liệu kỹ thuật, baseline, acceptance và authority. Không chọn AI Video Creator như một phụ thuộc bắt buộc của workforce.
3. Giao mục tiêu một lần cho orchestrator bằng prompt §6. Model/effort là lựa chọn theo run tại [01](01-PROJECT-CHARTER.md); trước dispatch phải readback session.
4. Theo dõi task/worker/blocker qua AO; orchestrator tự nhận kết quả, review và giao rework khi có lỗi thật. User không làm kênh relay giữa agents.
5. Nhận diff/code, test evidence, review/integration và giới hạn. Chỉ quyết định phần ngoài scope hoặc hành động không thể tự động hóa theo policy.

Pilot ban đầu [7B–7C](../evidence/run-20260924-phase7bc-native-code-pilot/summary.md) giữ nguyên bằng chứng gián đoạn `503`; continuation dùng orchestrator GPT-5.5/low và worker Gemini high, Astra/GPT-6-Astra không phải profile của continuation. A/B/C được ACCEPT, integration/test chung pass; rework thực tế `NOT OBSERVED` ([evidence](../evidence/run-20260924-phase7bc-integration-continuation/summary.md)). Không suy thành công GPT-5.5 là bằng chứng Astra hết quota. Pool giữ 1 Codex + 7 Gemini; Product worktree/evidence được bảo toàn.

---

## 1. Toolchain & Environment Preflight

Verify prerequisites (Go 1.24+, PowerShell 7+, Git, Codex CLI, Node.js/npm):
```powershell
powershell -ExecutionPolicy Bypass -File scripts\preflight.ps1
```

---

## 2. Private Runtime Configuration

To prevent exposing gateway secrets in version control (`INV-007`):
```powershell
# Copy template to private gitignored file
if (-not (Test-Path "config\cliproxy\config.runtime.yaml")) {
    Copy-Item "config\cliproxy\config.example.yaml" "config\cliproxy\config.runtime.yaml"
}

# Set runtime gateway key for local sessions
$env:CLIPROXY_KEY = "<YOUR_PRIVATE_KEY>"
```

---

## 3. Interactive Account Authentication (Phase 2)

Authenticate the minimum viable credential set (**1 Codex + 1 Antigravity**) before scaling:

```powershell
cd D:\TU_CODE\agent-orchestrator\CLIProxyAPI

# 1. Authenticate one ChatGPT Plus account:
.\cli-proxy-api.exe -config D:\TU_CODE\agents-coworkers\config\cliproxy\config.runtime.yaml -codex-login

# 2. Authenticate one Gemini Pro Google account:
.\cli-proxy-api.exe -config D:\TU_CODE\agents-coworkers\config\cliproxy\config.runtime.yaml -antigravity-login
```

### Verification After Each Login:
Do NOT rely on exit codes alone. CLIProxyAPI error paths may log and return without exiting non-zero. Verify:
1. Visible success message in the interactive terminal.
2. Run the sanitized inventory to verify count increases:
   ```powershell
   powershell -ExecutionPolicy Bypass -File D:\TU_CODE\agents-coworkers\scripts\auth-inventory.ps1
   ```
- **Login Verification Rule**:
  - `Success message AND credential count increases` -> New distinct account authenticated.
  - `Success message BUT count unchanged` -> Existing credential updated (relogin with same Google identity). This does NOT increment pool size.
- **Privacy Assurance**: The inventory script prints only `ProviderType`, `CredentialHash`, and `LastModified`. It never prints raw filenames, emails, full paths, or tokens.

### Checkpoint pool Phase 8 (2026-09-23)
Sau từng login tương tác, chạy inventory và đối chiếu danh tính đã băm trong bộ nhớ để loại lượt đăng nhập trùng; không đưa email, tên file auth hoặc token vào evidence. Ở run tiếp nối, inventory đi từ 1 Codex + 1 Gemini tới 1 + 7; một lượt Gemini trùng chỉ cập nhật credential cũ và không tăng pool. User dừng onboarding tại mức này.

Để kiểm chứng model mục tiêu, gửi request fixture nhỏ, tuần tự, giữ `gemini-3.8-flash-high` và `reasoning.effort=high`; đặt `X-Session-ID` ổn định để thử affinity và dùng session ID khác để thử phân phối. Chỉ trích status, latency, `Retry-After`, usage và chỉ số credential đã ẩn danh từ `X-CPA-TRACE-ID`; không lưu request/response body. Dừng request live khi gặp cooldown/429 và tôn trọng `Retry-After`. Các phép thử giả lập selector/retry phải ghi `SIMULATED`, không nâng thành live failover. Evidence và giới hạn thực tế ở `evidence/run-20260923-phase8-pool-onboarding/`.

---

## 4. Local Gateway Operations

### Launch Gateway:
```powershell
cd D:\TU_CODE\agent-orchestrator\CLIProxyAPI
.\cli-proxy-api.exe -config D:\TU_CODE\agents-coworkers\config\cliproxy\config.runtime.yaml
```

### Run Catalog Smoke:
```powershell
# Reuses running gateway or launches an isolated instance; fails closed if target models missing
powershell -ExecutionPolicy Bypass -File D:\TU_CODE\agents-coworkers\scripts\smoke-cliproxy.ps1
```

### Run Codex CLI Smoke:
```powershell
# Pre-auth check (requires documented unauthenticated error signature):
powershell -ExecutionPolicy Bypass -File D:\TU_CODE\agents-coworkers\scripts\smoke-codex-through-proxy.ps1 -ExpectAuthBlocked

# Post-auth check (strict fail-closed verification):
powershell -ExecutionPolicy Bypass -File D:\TU_CODE\agents-coworkers\scripts\smoke-codex-through-proxy.ps1
```

---

## 5. Agent Orchestrator Daemon Discovery & Configuration

### Dynamic Daemon Discovery Policy (`INV-006`):
The documented daemon discovery runbook logic (PowerShell snippet below) searches candidates in strict priority order, validates each via `/healthz`, confirms PID equality, and **fails closed** if multiple live daemons are running:

```powershell
$candidates = if ($env:AO_RUN_FILE) { @($env:AO_RUN_FILE) } else { @(
    (Join-Path $HOME ".ao\dev\running.json"),
    (Join-Path $HOME ".ao\running.json")
) }
$liveDaemons = @()
foreach ($candidate in ($candidates | Select-Object -Unique)) {
    if (-not (Test-Path $candidate)) { continue }
    try {
        $runInfo = Get-Content $candidate -Raw | ConvertFrom-Json
        $port = [int]$runInfo.port
        if ($port -lt 1 -or $port -gt 65535) { continue }
        $base = "http://127.0.0.1:$port"
        $health = Invoke-RestMethod `
            -Uri "$base/healthz" `
            -Method Get `
            -TimeoutSec 2
        if ($health.status -ne "ok") { continue }
        if ($health.service -ne "agent-orchestrator-daemon") { continue }
        if ($runInfo.pid -and $health.pid -and ([int]$runInfo.pid -ne [int]$health.pid)) { continue }
        $liveDaemons += [PSCustomObject]@{
            RunFile = $candidate
            BaseUrl = $base
            Port    = $port
            PID     = $health.pid
        }
    } catch { continue }
}
if ($liveDaemons.Count -eq 0) {
    throw "No valid live AO daemon found. Start AO or set AO_RUN_FILE explicitly."
}
if ($liveDaemons.Count -gt 1) {
    throw "Multiple live AO daemons found. Set AO_RUN_FILE explicitly to the intended daemon before mutating project configuration."
}
$aoBase = $liveDaemons[0].BaseUrl
Write-Host "Connected to AO Daemon on: $aoBase" -ForegroundColor Green
```

### Path A: per-session override, không ghi đè project defaults

`config/ao/project-config.example.json` là **baseline lịch sử worker low**, giữ nguyên trong lượt docs-only. Không copy vào default để dùng profile mục tiêu; không coi lệnh `ao spawn` không effort tự tạo worker high.

Source native `POST /api/v1/orchestrators/delegate` nhận body sau (minh họa, chỉ gửi khi workload/session được cấp quyền):

```json
{
  "projectId": "<VERIFIED_PROJECT_ID>",
  "brief": "",
  "agent": "codex",
  "model": "gemini-3.8-flash-high",
  "effort": "high",
  "approvalMode": "accept-edits",
  "mode": "chat"
}
```

Endpoint tạo kind worker; `brief` rỗng giữ worker idle theo source để readback **trước khi giao việc**. Brief khác rỗng có thể chạy ngay lúc spawn, không dùng cho bước preflight này. Trước dispatch, đối chiếu `<ORCHESTRATOR_MODEL/EFFORT>` và `<WORKER_MODEL/EFFORT>` user chọn với catalog hiện hành, eligibility của credential pool cho đúng provider/model, rồi đọc lại ID/kind/harness/model/reasoningEffort/worktree/base của từng session qua API chính thức. Chỉ khi khớp mới gửi task gồm ID/objective/exact scope/contract/tests/stop/reply session bằng kênh AO ở §7. Endpoint không đổi project defaults, nhưng native delegation có thể tạo/resume orchestrator cho title refinement khi brief khác rỗng; không coi nó là workflow planner. Với session có sẵn, reuse đúng session và API session settings được binary hỗ trợ; **không gọi delegate để tạo bản thay thế**. Không tạo alias hay đổi model ID do user cung cấp.

`accept-edits` không có nghĩa tự duyệt mọi lệnh. Giữ approvalPolicy=on-request, sandbox=workspace-write, reviewer=user trong scope CP1/CP2A. Nếu readback/effective config không khớp, dừng execution với mismatch, không bật bypass/auto-review để né lỗi. Upstream thinkingLevel chưa quan sát, không cần full request logging để chạy workflow đã chấp nhận giới hạn.

Path B [patch CLI effort](../patches/agent-orchestrator/0001-cli-support-agent-effort.patch) vẫn tùy chọn; chỉ đề xuất khi native API không đáp ứng. Không apply/build upstream trong lượt này.

## 6. Prompt orchestrator tái sử dụng — giao mục tiêu một lần

```text
Bạn là orchestrator của agents-coworkers, dùng AO làm lõi và CLIProxyAPI
làm pool. Dự án đích: <REPO>; baseline: <SHA>; mục tiêu: <GOAL>;
tài liệu authoritative: <DOCS>; authority/allowlist: <SCOPE>; acceptance: <ORACLES>.
Profile user chọn cho run: orchestrator=<ORCHESTRATOR_MODEL/EFFORT>;
worker=<WORKER_MODEL/EFFORT>. Trước dispatch, xác nhận cả hai profile có
trong catalog, credential pool đúng provider/model đủ điều kiện và session
readback khớp. Không đổi project defaults, pool hay model ID để làm PASS.

Đọc AGENTS.md và tài liệu liên quan. Giữ dirty diff/evidence; không dùng
Product root làm nơi thực thi khi chỉ worktree được duyệt.
Lập task graph từ công việc thật. Chọn số workers theo dependency và
resource budget, tối đa mức đã duyệt; không ép đủ 3 khi task phụ thuộc.
Mỗi task có ID, objective, input/output, dependency, file owner/allowlist,
contract, test oracle, worktree/base, resource namespace, stop condition.
File dùng chung chỉ một writer; integration do bạn sở hữu và thực hiện
tuần tự sau review. Không tự đổi model/effort/default hoặc nâng authority.

Dùng AO native để giao việc và nhận kết quả. Workers:
kind=worker, harness=codex, model=<WORKER_MODEL>, effort=<WORKER_EFFORT>.
Readback trước thực thi; gửi context tối thiểu, đường dẫn và contract
liên quan thay vì lặp toàn repo. Ghi task record trong conversation AO
cùng session/turn/delivery IDs và artifact revision/hash.

Worker gửi kết quả về AO orchestrator: task ID, diff/files, test command
và exit/result, artifact/hash, blocker, next action. Không bắt user relay.
Tự đọc diff/test để quyết định ACCEPT hoặc REWORK có lý do. Review bắt buộc
khi đổi interface/schema/security/shared files, test fail, scope drift,
trước integration; checkpoint khác theo rủi ro, không theo từng lệnh đọc.
Giao rework qua AO; kiểm lại kết quả, không code thay worker rồi gọi đó
là proof autonomous. Mọi can thiệp coordinator phải ghi provenance.

Chỉ ghép các thay đổi đã review theo dependency và authority integration.
Chạy test chung; báo cáo code/test/evidence và phần chưa quan sát.
Không tự commit/push/merge nếu scope chưa cho phép.

Quota do gateway xử lý. Tôn trọng Retry-After/budget; không đổi account/model
thủ công để né policy, không đốt quota. Nếu hết pool hoặc session gián đoạn:
persist tiến độ/next action/delivery IDs; đọc process/turn/artifact trước
retry; recover delivery uncertain, không phát lại side effect đã hoàn tất.
Nếu chưa có wake-up/resume mechanism đã kiểm chứng, ghi gap cụ thể, không
hứa tự phục hồi từ một prompt. Đề xuất native/config trước patch nhỏ.

Tự xử lý debug/test và approval nằm trong quyền user đã ủy quyền,
không đổi sandbox hoặc global permission. Chỉ hỏi user khi thiếu quyết
định sản phẩm, vượt scope hoặc thao tác tương tác không thể tự thực hiện.
Dừng tại acceptance/audit đã giao; cleanup đúng owner, báo phần còn pending.
```

## 7. Messages, approval và recovery

- AO source `backend/internal/cli/send.go` hỗ trợ `ao send --session <ID> --message "<BRIEF>"`; không cần user copy/paste. Với chat delivery có rủi ro gửi trùng, dùng `--steer --client-message-id <STABLE_ID>`. Khi `CHAT_STEER_UNCERTAIN`, dùng `--steer --recover-only --client-message-id <SAME_ID>` để lấy receipt, không gửi lại. Đây là source-supported path; binary phải hỗ trợ và workflow phải có evidence riêng.
- Nhận message không đồng nghĩa agent đã thực hiện. Đọc status/conversation/artifact. RetryTurn không dành cho mọi message (failed human-origin/active lineage/known provider turn); không biến retry API thành bảo đảm exactly-once tools.
- Quyền `delegated_user_authorization_cp1` chỉ áp dụng CP1 và phần CP2A user mở rộng trong đúng worktree. Khi AO phát approval: đối chiếu exact command, cwd, file/resource ownership và scope; nếu hợp lệ gửi allow-once qua API chính thức, ghi decision/provenance. Không allow-all/global bypass. Scope không khớp thì từ chối và xin delta.
- Không tự mang delegation đó sang workload/repo khác. Gói authority mới phải ghi quyền đọc/sửa/test/integration/cleanup và giới hạn trước dispatch, không hỏi lại mỗi thao tác đã nằm trong gói.
- Hộp thoại Windows nếu công cụ/policy không cho tự bấm thì báo đúng certificate/action và chờ user; không bypass.
- Reuse pinned environment/container khi ownership/readiness đúng. Port/DB/temp/certificate có run owner; không reset repo chung, dừng Docker Desktop hoặc xóa resource không thuộc run.
- Toàn pool unavailable: lưu task state, deadline/next retry, giới hạn attempts và side-effect receipt; không vòng retry nóng. Wake-up tự động còn là gap cần proof ở roadmap 7D, không nói đã vận hành chỉ vì runbook có mô tả.
- Không dump toàn bộ environment từ worker (`Get-ChildItem Env:` / `env`). Chỉ kiểm tra key cụ thể đã allowlist; ưu tiên ghi presence/status, không value. Giới hạn stdout/tool output mặc định 4 KiB và filter/redact trước khi lưu; không ghi token, cookie, capability hoặc raw environment vào history/evidence.
- 7D fixture proof: `exit-agent`/`resume-agent` native đã tiếp tục cùng session C; message mới gửi tường minh vào session idle chạy ngay. T1/T2 bị `conversation/interrupt` là expected cancellation, không phải bằng chứng auto-wake thiếu. Same-session resume/explicit wake `LIVE`; delivery recovery dùng AO/provider thật cũng `LIVE`, còn transport response-loss là injection `SIMULATED`. Provider outage, daemon restart, unattended recovery và Stop LIVE `NOT OBSERVED`. Prototype dispatcher dùng checkpoint bền vững, AO/Git preflight, write-ahead uncertain trước Send, recover-only cùng delivery ID, single-owner serialization và giữ human Stop terminal. AO turn completed chỉ đóng task khi receipt đúng task/artifact oracle. Chi tiết tại [live evidence](../evidence/run-20260924-phase7d-wakeup-proof/summary.md), [prototype evidence](../evidence/run-20260925-phase7d-recovery-prototype/summary.md) và [durable correction](../evidence/run-20260925-phase7d-stop-live-recovery/durable-correction.md).
- Trước ordinary Send, wrapper recovery phải giữ lease run-owned độc quyền xuyên suốt preflight + POST, xác minh đúng session owner và controller `ready`. `outcome=steered` là contamination và phải BLOCK; chỉ `outcome=sent` tạo task turn độc lập. Observe phải phân trang tới đúng turn hoặc hết history; lỗi Observe giữ nguyên `DELIVERED`/`DELIVERY_UNCERTAIN`. Human Stop ghi `CANCELLED` trước interrupt; không wake/recover task đó dù interrupt response bị mất.
- Response session phải được đọc từ `{session:{...}}`; checkpoint live khóa project/kind/harness/model/effort/branch và artifact Git branch/head/hash. Khi Cancel gặp delivery uncertain, ghi `CANCELLED_LOCAL_AO_STOP_UNCONFIRMED`, recover-only cùng delivery ID, rồi chỉ gọi interrupt nếu recovered turn là live turn duy nhất. Nếu interrupt outcome không xác nhận được, giữ state unconfirmed và cách ly session; `Step` không được reconcile hoặc send. Lease path luôn do wrapper suy từ workspace root + run owner + task ID; stale lease chỉ dọn sau kiểm ownership/process ngoài contract này.
- Không mô tả `/conversation/interrupt` là exact-turn primitive: route hiện session-wide, không có expected turn/controller fence và precheck có race. Chỉ dùng trong session disposable, run-owned, độc quyền; nếu không chứng minh được exclusivity thì fail-closed. `SessionExclusive` chỉ là assertion/lease của wrapper, không khóa được AO client khác. `completed` phải kiểm receipt và giữ `COMPLETED`; thiếu receipt giữ outcome unverified; `failed` là `DELIVERY_FAILED`; chỉ observed `interrupted/cancelled` mới xác nhận `CANCELLED`.
