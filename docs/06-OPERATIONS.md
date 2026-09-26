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
- Vận hành CLI recovery (`recovery.exe`):
  - Tra cứu trạng thái checkpoint: `recovery status -checkpoint <path-to-checkpoint.json> [-json]`
  - Thực thi vòng lặp phục hồi: `recovery run -checkpoint <path-to-checkpoint.json> -workspace <path-to-worktree> -ao-url <daemon-url> -poll <poll-duration> -timeout <timeout-duration>`
  - Mã thoát chuẩn hóa: `0` (thành công / trạng thái terminal `COMPLETED`), `1` (lỗi sử dụng CLI, ví dụ lệnh cancel không được hỗ trợ qua CLI), `2` (lỗi terminal, ví dụ `BLOCKED: ARTIFACT_CHANGED`), `3` (hết hạn timeout context trước khi đạt trạng thái terminal).
  - Khả năng di chuyển (portability) và self-host LIVE đã được xác nhận tại Phase 7E ([evidence](../evidence/run-20260926-phase7e-selfhost-pass/summary.md)). Giữ nguyên các giới hạn bất biến: provider outage tự phục hồi LIVE, daemon restart recovery LIVE, và Stop LIVE đều `NOT OBSERVED`; exact-turn Stop trên shared session là unsupported và fail-closed.
- Không mô tả `/conversation/interrupt` là exact-turn primitive: route hiện session-wide, không có expected turn/controller fence và precheck có race. Chỉ dùng trong session disposable, run-owned, độc quyền; nếu không chứng minh được exclusivity thì fail-closed. `SessionExclusive` chỉ là assertion/lease của wrapper, không khóa được AO client khác. `completed` phải kiểm receipt và giữ `COMPLETED`; thiếu receipt giữ outcome unverified; `failed` là `DELIVERY_FAILED`; chỉ observed `interrupted/cancelled` mới xác nhận `CANCELLED`.

---

## 8. Phase 8 Proposed Control Surface (Draft Design / Implementation Not Authorized)

> [!IMPORTANT]
> Toàn bộ nội dung trong mục này là tài liệu thiết kế và quy chuẩn vận hành dự kiến cho Phase 8 (`cmd/coworkers`). Hiện tại **CHƯA ĐƯỢC CẤP QUYỀN THỰC THI (IMPLEMENTATION NOT AUTHORIZED)**. Khi vận hành thực tế ở thời điểm hiện tại, kỹ sư tiếp tục sử dụng runbook tại các Mục 0–7 và công cụ `recovery.exe` (`cmd/recovery`).

### 8.1. Entrypoint điều khiển dòng lệnh: `coworkers` (PROPOSED)

CLI `coworkers` được thiết kế như một thin client giao tiếp qua loopback HTTP REST với Agent Orchestrator daemon và CLIProxyAPI gateway.

#### Thứ tự phân giải Endpoint & Xác thực (Endpoint Discovery & Gateway Auth):
Hệ thống xác định endpoint của Agent Orchestrator theo thứ tự ưu tiên nghiêm ngặt (nguyên tắc fail-closed):
1. Cờ lệnh tường minh: `--ao-url <URL>` (bắt buộc host loopback `localhost`, `127.0.0.1`, hoặc `::1`; nếu truyền URL không hợp lệ hoặc không phải loopback thì fail-closed ngay lập tức, không fallback; xác minh `GET /api/v1/identity`; không dùng OS port inspection để suy PID; ghi nhận `aoDiscoverySource: "explicit_url"`, `aoPid: 0`, `aoPidStatus: "NOT_OBSERVED"`);
2. Biến môi trường: `AO_BASE_URL` (bắt buộc host loopback; nếu tồn tại nhưng không hợp lệ hoặc không phải loopback thì fail-closed, không fallback; xác minh `GET /api/v1/identity`; ghi nhận `aoDiscoverySource: "environment_url"`, `aoPid: 0`, `aoPidStatus: "NOT_OBSERVED"`);
3. Biến môi trường: `AO_RUN_FILE` (đường dẫn tệp run metadata, ví dụ `running.json`, không phải URL; parse `running.json`, kiểm tra PID > 0 và tiến trình còn sống, tạo loopback URL từ port, xác minh `GET /api/v1/identity`; ghi nhận `aoDiscoverySource: "run_file"`, `aoPid`, `aoPidStatus: "VERIFIED"`);
4. Các tệp candidate mặc định: `~/.ao/dev/running.json`, `~/.ao/running.json` (chỉ quét khi không có cả ba nguồn ưu tiên trên; quy tắc xác minh tương tự `AO_RUN_FILE`);
5. Quy tắc PID: `/api/v1/identity` chỉ trả `hostId` và `apiVersion` (hoặc `contractVersion`), không trả PID. Chỉ có `running.json` mới chứa PID. Do đó, chỉ ghi nhận `aoPidStatus: "VERIFIED"` khi endpoint đến từ `running.json`. Tuyệt đối không suy PID bằng OS port inspection khi URL được cấp trực tiếp;
6. Nếu quét các candidate mặc định mà phát hiện nhiều hơn một daemon hợp lệ đang chạy thì **dừng lại ngay lập tức (fail-closed, exit 2)** và yêu cầu người dùng chỉ định rõ endpoint qua `--ao-url`. Nếu không tìm thấy daemon hợp lệ nào: exit 3. Cổng 3005 không phải là cổng kiến trúc cố định. Nếu endpoint hợp lệ nhưng không kết nối được hoặc timeout: exit 3. Nếu sai lệch parse/schema/identity/PID: exit 2.

Cổng của gateway CLIProxyAPI được phân giải theo thứ tự:
1. Cờ lệnh tường minh: `--gateway-url <URL>` (nếu không hợp lệ thì fail-closed, không fallback);
2. Biến môi trường: `COWORKERS_GATEWAY_URL` (nếu không hợp lệ thì fail-closed, không fallback);
3. Giá trị mặc định lịch sử: `http://127.0.0.1:8317`.

Xác thực Gateway: Thăm dò catalog gateway sử dụng `GET /v1/models`. Khi endpoint yêu cầu xác thực, khóa gateway được lấy từ biến môi trường `CLIPROXY_KEY` trong process environment. Tuyệt đối không đưa khóa vào RunSpec, RunManifest, argv, stdout, stderr hoặc evidence; không ghi raw Authorization header; thiếu credential bắt buộc hoặc 401/403: exit 2; không kết nối được hoặc timeout: exit 3; gateway trả catalog/JSON không hợp lệ: exit 2.

#### Các lệnh con và quy chuẩn vận hành trong Slice 8A:

1. **Khám sức khỏe môi trường và hạ tầng (`coworkers doctor`)**:
   ```bash
   coworkers doctor --spec <run-spec.json> [--ao-url <url>] [--gateway-url <url>] [--json]
   ```
   - **Cờ `--spec` là bắt buộc**: Nếu thiếu, tệp hỏng hoặc sai schema `RunSpec`, lệnh lập tức thoát với exit code `2`.
   - **Kiểm tra kết nối và danh mục (Catalog Probe)**: Kiểm tra kết nối và identity của AO daemon (`GET /api/v1/identity`) và catalog gateway (`GET /v1/models`). Lệnh `doctor` chỉ chứng minh: gateway reachable, `CLIPROXY_KEY` được chấp nhận, `/v1/models` trả catalog hợp lệ và model yêu cầu xuất hiện trong catalog. **Tuyệt đối không đồng nhất sự hiện diện của model trong catalog với credential eligibility hay usable capacity** (`providerCallPerformed: false`, `credentialEligibility: "NOT_OBSERVED"`).
   - **Kiểm tra công cụ thực thi (`requiredTools`)**: `git` luôn là công cụ bắt buộc cho Slice 8A. Các công cụ khác (Python, Node, Go...) chỉ được kiểm tra khi `RunSpec` khai báo tường minh trong `requiredTools`. `doctor` dùng `exec.LookPath` hoặc kiểm tra file thực thi trực tiếp, tuyệt đối không dựng shell command từ giá trị `RunSpec`; tên công cụ phải qua xác thực chặt chẽ (không chứa đường dẫn hay shell metacharacter). Go được dùng để build/test chính `agents-coworkers` nhưng không phải prerequisite mặc định của target project; Node không được hardcode làm prerequisite.
   - **Chính sách không gian làm việc (`workspacePolicy`)**:
     * `productRootMustBeClean: true`: Checkout gốc Product (`AI Auto Video Creator`) bắt buộc phải ở trạng thái sạch hoàn toàn qua raw `git status --porcelain=v1 -z`, không có bất kỳ ngoại lệ nào (kể cả tệp dưới `.agents-coworkers/**`).
     * `executionWorkspaceDirtyPolicy`: Nhận giá trị `require_clean` (mặc định) hoặc `allow_dirty_recorded`. Execution workspace đánh giá source-dirty từ raw porcelain (`git status --porcelain=v1 -uall -z`) sau khi loại duy nhất exact untracked porcelain record đã tính từ `RunSpec.runId` (`?? .agents-coworkers/recovery-leases/<sha256(runId + "\n__run__")>.lease`). Mọi trạng thái tracked, staged, modified, deleted, rename hoặc copy tại cùng path vẫn được xem là source-dirty; tuyệt đối không gọi mọi nội dung `.agents-coworkers/**` là expected; ngoại lệ lease chỉ tồn tại để giữ tính lũy đẳng khi cùng run chuyển FREE→OWNED. Nếu `allow_dirty_recorded`, worktree thực thi được phép dirty và `porcelainSha256` là SHA-256 của canonical filtered source-porcelain này; không tự suy đoán mọi dirty state là "expected".
   - **Không đụng chạm Lease**: `doctor` không kiểm tra hoặc acquire lease.
   - **Xuất kết quả**: Xuất `DoctorReport` qua `stdout`/JSON; **tuyệt đối không tạo hoặc sửa đổi `RunManifest`** và **không nhận cờ `--manifest`**.

2. **Gắn kết phiên điều khiển (`coworkers attach`)**:
   ```bash
   coworkers attach --session <session-id> --workspace <path> --spec <path-to-run-spec.json> --manifest <path-to-run-manifest.json> [--json]
   ```
   - Gắn kết control-plane vào session/project hiện có trên AO daemon. Cờ `--manifest` là đường dẫn xuất tệp (output path).
   - **Kiểm tra Workspace Dirty Policy**:
     * Product root checkout bị dirty (đánh giá nguyên trạng output `git status --porcelain=v1 -z`, không ngoại lệ) -> luôn lập tức dừng và thoát với exit code `2`.
     * Nếu `executionWorkspaceDirtyPolicy == "require_clean"` mà execution workspace bị dirty (sau khi loại duy nhất exact untracked lease record của run) -> exit code `2`.
     * Nếu `executionWorkspaceDirtyPolicy == "allow_dirty_recorded"` mà execution workspace bị dirty -> cho phép tiếp tục (PASS) và tính toán băm `porcelainSha256` từ canonical filtered source-porcelain (sau khi loại duy nhất exact untracked lease record của run) để ghi vào `worktreeBinding`.
   - **Kiểm tra Run Lease (Chỉ gọi API inspect read-only)**:
     * Lease root: `<executionWorkspace>/.agents-coworkers/recovery-leases`. Tên tệp lease suy từ SHA-256 của `runOwner + "\n" + taskId` với `runOwner = RunSpec.runId`, `taskId = "__run__"`.
     * `attach` **tuyệt đối không gọi `Acquire` hoặc `Release`** vì không có tiến trình run dài hạn giữ lease trong Slice 8A.
     * `FREE`: Cho phép `attach` tiếp tục tạo manifest.
     * `OWNED` (khớp `runId` và binding hiện hành): Cho phép đọc lại thành công lũy đẳng (idempotent readback).
     * `LOCKED` (thuộc owner/task khác): Xung đột quyền sở hữu -> lập tức thoát với exit code `2`.
   - **Tự động tạo mới manifest nếu chưa có sẵn**:
     * Nếu manifest chưa tồn tại: lệnh `attach` sẽ tạo mới sau khi kiểm tra hợp lệ.
     * Nếu manifest đã tồn tại và khớp hoàn toàn (`runId`, `runSpecSha256`, session, repository và worktree binding): trả về thành công lũy đẳng (idempotent success), không ghi đè lại.
     * Nếu manifest đã tồn tại nhưng bị hỏng cấu trúc hoặc sai lệch binding: fail-closed và thoát với exit code `2`.
   - **Thuật toán Worktree Binding bắt buộc**: Endpoint `GET /api/v1/sessions/{id}` của AO (SessionView) không trả về đường dẫn thư mục vật lý của worktree; do đó hệ thống không được tuyên bố xác minh worktree chỉ bằng lệnh GET session. Lệnh `attach` bắt buộc thực thi 9 bước tuần tự:
     1. Đọc session qua AO API và lấy project ID + branch;
     2. Canonicalize target root và explicit execution workspace path;
     3. Chạy `git worktree list --porcelain -z` trên đúng repository;
     4. Tìm đúng một worktree có branch bằng session branch;
     5. Path canonical của worktree phải bằng execution workspace;
     6. HEAD phải bằng expected HEAD/baseline theo trạng thái run;
     7. Repository common-dir và identity phải khớp target repo;
     8. Nếu không có worktree, có nhiều kết quả, detached ngoài contract hoặc mismatch -> lập tức thoát với exit code `2`;
     9. Tuyệt đối không đọc `ao.db` và không suy physical path từ `SessionView`.
   - **Ghi nguyên tử (Atomic Write)**: Tạo `RunManifest` qua cơ chế atomic write: ghi tệp tạm -> gọi fsync/close -> rename sang `--manifest`.

3. **Báo cáo trạng thái phiên và run lease (`coworkers status`)**:
   ```bash
   coworkers status --manifest <path-to-run-manifest.json> [--json]
   ```
   - Ở Slice 8A, lệnh `status` chỉ đọc `run-manifest.json` và trạng thái live read-only: manifest hiện hành, identity endpoint của daemon/gateway, phiên/profile được gắn kết, workspace/baseline SHA, và trạng thái quyền sở hữu run lease.
   - Nếu manifest bị thiếu hoặc hỏng định dạng: lập tức thoát với exit code `2`. Lệnh `status` **không đọc `RunSpec` để tự tái tạo binding** và **không sửa đổi manifest**.
   - **Không hiển thị Task DAG**: Tuyệt đối không có trường Task DAG nào trong Slice 8A. Trạng thái task DAG chỉ xuất hiện sau khi Slice 8B được triển khai.
   - **Quy chuẩn Lease**: `FileLease` dùng `O_EXCL` để phối hợp độc quyền giữa các process `coworkers`/`recovery` cùng tuân thủ wrapper contract. Nó không phải global AO lock và không ngăn client AO bên ngoài sử dụng session. Không có TTL và không tự động phá vỡ stale lease. Lệnh `status` luôn re-inspect trạng thái lease thực tế từ `workspaceRoot`, `runOwner`, và `taskId: "__run__"`; **không tin tưởng `observedState` cũ đã ghi trong manifest**. Trạng thái hiển thị gồm `FREE`, `OWNED`, hoặc `LOCKED`. Tuyệt đối không dùng trạng thái `ACQUIRED` trong Slice 8A. PID chỉ là metadata giám sát, không kiểm tra PID còn sống để tự phá lease. Slice 8A chỉ bổ sung API inspect read-only cho status, không thay đổi ngữ nghĩa Acquire/Release.

4. **Từ chối thực thi lệnh chạy tự động (`coworkers run`)**:
   ```bash
   coworkers run --spec <path-to-run-spec.json>
   ```
   - Trong Slice 8A, lệnh này bắt buộc luôn trả về mã thoát `1` với thông báo lỗi: `coworkers run is not supported until Slice 8B is accepted`.

#### Ma trận mã thoát chuẩn hóa:
- `0`: Thành công (success).
- `1`: Lỗi CLI usage hoặc lệnh chưa hỗ trợ (`run` luôn trả về mã thoát `1`).
- `2`: Lỗi kiểm tra / xác thực (validation, identity, baseline, worktree, hoặc lease conflict).
- `3`: Endpoint không khả dụng hoặc hết thời gian chờ (endpoint unavailable hoặc timeout).

### 8.2. Hợp đồng Run Spec và Run Manifest (Bản thảo đề xuất / PROPOSED)

Hệ thống sử dụng định dạng JSON thuần túy để triển khai Slice 8A bằng thư viện chuẩn của Go (`encoding/json`), không phụ thuộc thư viện bên ngoài.

#### 1. Tệp đặc tả lượt chạy (`run-spec.json` — Immutable User Input):
```json
{
  "schemaVersion": "run-spec/v1-draft",
  "runId": "run-20261001-sample-001",
  "description": "Sample autonomous run specification",
  "target": {
    "targetRoot": "<TARGET_REPO_ROOT_PATH>",
    "executionWorkspace": "<RUN_WORKTREE_PATH>",
    "baselineSha": "<BASELINE_SHA>",
    "expectedBranch": "<AO_SESSION_BRANCH>"
  },
  "workspacePolicy": {
    "productRootMustBeClean": true,
    "executionWorkspaceDirtyPolicy": "require_clean"
  },
  "requiredTools": [
    {
      "name": "git",
      "required": true
    }
  ],
  "endpointPolicy": {
    "aoDiscovery": "flag_env_runfile_candidates_fail_closed",
    "gatewayDiscovery": "flag_env_historical_default_fail_closed"
  },
  "concurrency": {
    "maxConcurrency": 3
  },
  "profiles": {
    "orchestrator": {
      "model": "<ORCHESTRATOR_MODEL>",
      "reasoningEffort": "low"
    },
    "worker": {
      "model": "<WORKER_MODEL>",
      "reasoningEffort": "high"
    }
  },
  "documentation": {
    "targetRepoDocs": [
      "<TARGET_REPO>/docs/architecture.md",
      "<TARGET_REPO>/docs/contracts.md"
    ],
    "controlPlaneDocs": [
      "docs/02-ARCHITECTURE.md",
      "docs/06-OPERATIONS.md"
    ]
  },
  "policies": {
    "approvalPolicy": "per_task_delegated",
    "evidenceDir": "evidence/run-sample-001",
    "integrationBranchPolicy": "local_only",
    "cleanupOwnership": "run_lifecycle_bound"
  }
}
```

#### 2. Tệp manifest lượt chạy (`run-manifest.json` — Generated Output):
```json
{
  "schemaVersion": "run-manifest/v1-draft",
  "runId": "run-20261001-sample-001",
  "runSpecSha256": "<RUN_SPEC_SHA256>",
  "generatedAt": "2026-10-01T00:00:00Z",
  "generatedBy": "coworkers attach",
  "target": {
    "targetRoot": "<TARGET_REPO_ROOT_PATH>",
    "executionWorkspace": "<RUN_WORKTREE_PATH>",
    "baselineSha": "<BASELINE_SHA>",
    "currentHead": "<CURRENT_HEAD_SHA>",
    "branch": "<AO_SESSION_BRANCH>",
    "repositoryIdentity": "<REPO_COMMON_DIR_ID>"
  },
  "endpoints": {
    "aoUrl": "<AO_RESOLVED_URL>",
    "aoIdentity": "<AO_SERVICE_IDENTITY>",
    "aoDiscoverySource": "run_file",
    "aoPid": 12345,
    "aoPidStatus": "VERIFIED",
    "gatewayProbe": {
      "url": "http://127.0.0.1:8317",
      "catalogPath": "/v1/models",
      "catalogStatus": "VERIFIED",
      "observedModels": [
        "<ORCHESTRATOR_MODEL>",
        "<WORKER_MODEL>"
      ],
      "providerCallPerformed": false,
      "credentialEligibility": "NOT_OBSERVED"
    }
  },
  "sessionBinding": {
    "sessionId": "<SESSION_ID>",
    "projectId": "<PROJECT_ID>",
    "sessionBranch": "<AO_SESSION_BRANCH>"
  },
  "worktreeBinding": {
    "canonicalPath": "<RUN_WORKTREE_PATH>",
    "worktreeBranch": "<AO_SESSION_BRANCH>",
    "head": "<EXPECTED_HEAD>",
    "isClean": true,
    "dirtyPolicy": "require_clean",
    "porcelainSha256": "<PORCELAIN_SHA256>",
    "verifiedPorcelain": true
  },
  "attachedSessionProfile": {
    "kind": "orchestrator",
    "harness": "codex",
    "model": "<ORCHESTRATOR_MODEL>",
    "reasoningEffort": "low",
    "status": "VERIFIED"
  },
  "lease": {
    "workspaceRoot": "<CANONICAL_EXECUTION_WORKSPACE>",
    "runOwner": "run-20261001-sample-001",
    "taskId": "__run__",
    "ownerId": "run-20261001-sample-001",
    "observedState": "FREE",
    "observedPid": 0
  },
  "sourceProvenance": {
    "cliProxyApiSha": "2430354330af80b645f9ffb1a51e1e7c72c4cc8e",
    "sourceAuditOnly": true
  }
}
```

### 8.3. Quy chuẩn Điều phối Concurrency (Slice 8B)
- **Mức mặc định**: Tối đa **3 workers đồng thời**. Đây là giới hạn cao nhất đã được chứng minh an toàn trong môi trường live tại Gate 10.
- **Mức mở rộng thử nghiệm (4–7 workers)**: Được gán nhãn **`EXPERIMENTAL_CAPACITY`**. Chỉ được kích hoạt khi:
  1. Đã thu thập đủ dữ liệu telemetry từ Slice 8C chứng minh pool có đủ credential khỏe và không bị nghẽn rate limit;
  2. Được cấp quyền tường minh (explicit authority) từ người dùng;
  3. Có bằng chứng cô lập tài nguyên phần cứng (CPU, memory, disk I/O, separate git worktrees) đầy đủ;
  4. Thực hiện phép thử dung lượng (capacity proof) riêng có ghi nhận bằng chứng.

### 8.4. Quy chuẩn Quan sát Định tuyến & Telemetry (Slice 8C)
- **Nguồn gốc mã nguồn (Provenance)**: Mã nguồn CLIProxyAPI được audit tại commit `2430354330af80b645f9ffb1a51e1e7c72c4cc8e`. Audit mã nguồn không chứng minh binary runtime đang chạy được biên dịch từ commit này.
- **Header `X-CPA-TRACE-ID`**: Có định dạng `<YYYYMMDDHHMMSS>-<authIndex>-<requestID>`. `authIndex` là chuỗi 16-hex pseudonymous từ SHA-256 của credential seed. Nó có rủi ro liên kết tài khoản (correlation risk), không chứa raw token nhưng là metadata vận hành nhạy cảm bắt buộc phải redact và giới hạn thời gian lưu trữ (bounded retention); không tuyên bố là an toàn tuyệt đối.
- Khi header `X-CPA-TRACE-ID` vắng mặt (do chưa chọn credential hoặc response không đi qua trace callback), bắt buộc ghi nhận `NOT_OBSERVED` hoặc `UNKNOWN`.
- **Header `Retry-After`**: Chỉ được đọc khi xuất hiện thực tế trong response header từ gateway. Tuyệt đối không giả định mọi mã 429 hoặc 503 đều có `Retry-After`.
- **Phân loại lỗi**: Kết hợp HTTP status code, structured error code/reason trong JSON body và response headers. Không tự động ánh xạ mọi mã 402 thành `deactivated_workspace` hay mọi mã 403 thành `VALIDATION_REQUIRED`. Trường hợp thiếu dữ liệu phân loại là `UNKNOWN`.
- **Thứ tự ưu tiên**: (1) Native API / existing headers; (2) Safe client-side adapter; (3) Patch candidate chỉ sau khi có audit riêng và được duyệt. Không đề xuất patch là hướng mặc định.

### 8.5. Behavior Matrix 8A
Ma trận hành vi bắt buộc kiểm chứng cho Slice 8A:
1. **Doctor**:
   - Cờ `--spec` là bắt buộc; thiếu hoặc hỏng `RunSpec`: fail-closed, exit code 2;
   - Không tạo hoặc sửa tệp `RunManifest`; không nhận cờ `--manifest`;
   - Model catalog presence không nâng thành credential eligibility (`providerCallPerformed: false`, `credentialEligibility: "NOT_OBSERVED"`);
   - Git luôn bắt buộc; các công cụ khác kiểm tra theo `requiredTools` (kiểm tra executable trực tiếp, invalid tool name fail-closed exit 2);
   - Không tạo hoặc inspect lease;
   - Kiểm tra redaction bí mật `CLIPROXY_KEY` trong stdout/stderr/logs;
   - Kiểm tra cờ endpoint tường minh hoặc biến môi trường không hợp lệ sẽ fail-closed ngay lập tức, không fallback;
   - Kiểm tra mã thoát: exit code 2 (validation/catalog/auth lỗi) và exit code 3 (unreachable/timeout) đúng contract;
   - Xác nhận Product root checkout sạch (`git status` clean); chấp nhận dirty trên run worktree chỉ khi `executionWorkspaceDirtyPolicy == "allow_dirty_recorded"`.
2. **Attach**:
   - Kiểm tra `workspacePolicy`: Product root dirty (đánh giá nguyên trạng output `git status --porcelain=v1 -z`, không ngoại lệ) luôn exit code 2; `require_clean` + dirty worktree (sau khi loại duy nhất exact untracked lease record của run từ `git status --porcelain=v1 -uall -z`) -> exit code 2; `allow_dirty_recorded` + dirty worktree -> PASS và ghi nhận `porcelainSha256` (SHA-256 của canonical filtered source-porcelain);
   - Kiểm tra run lease từ API inspect read-only (`workspaceRoot`, `runOwner`, `taskId: "__run__"`); tuyệt đối không gọi `Acquire` hoặc `Release`;
   - `FREE`: cho phép tiếp tục; `OWNED` cùng run: idempotent readback; `LOCKED`: fail-closed, exit code 2;
   - Thiếu hoặc hỏng `RunSpec`: exit code 2;
   - Manifest chưa tồn tại (manifest absent): tạo mới bằng cơ chế atomic write thành công;
   - Manifest đã tồn tại và khớp hoàn toàn (`runId`, `runSpecSha256`, session, repo, worktree): trả về thành công lũy đẳng (idempotent success);
   - Manifest đã tồn tại nhưng hỏng cấu trúc hoặc sai lệch binding: fail-closed, exit code 2;
   - Chỉ xác nhận đối tượng duy nhất `attachedSessionProfile`;
   - Kiểm tra băm toàn vẹn `runSpecSha256`;
   - Tuyệt đối không truy cập trực tiếp `ao.db`.
3. **Status**:
   - Re-inspect trạng thái lease thực tế từ `workspaceRoot`, `runOwner`, `taskId: "__run__"`; không tin tưởng `observedState` cũ trong manifest;
   - Trạng thái lease: `FREE`, `OWNED`, `LOCKED` (tuyệt đối không có trạng thái `ACQUIRED`);
   - Không tự động phá stale lease; PID chỉ là metadata giám sát;
   - Xử lý các tình huống manifest: hợp lệ, thiếu, hoặc hỏng cấu trúc (thiếu/hỏng: exit code 2);
   - Không tự động phục hồi hay tái tạo manifest từ `RunSpec`;
   - Tuyệt đối không xuất hiện bất kỳ trường Task DAG nào trong Slice 8A.
4. **CLI**:
   - Cú pháp `coworkers doctor --spec <run-spec.json>`; không nhận cờ `--manifest`;
   - Lệnh `attach` bắt buộc nhận cả `--spec` và `--manifest` (output path);
   - Lệnh `status` bắt buộc nhận `--manifest`;
   - Lệnh `run` bị từ chối rõ ràng với mã thoát `1` trong toàn bộ Slice 8A;
   - Ma trận mã thoát chuẩn hóa: `0` (thành công), `1` (lỗi usage/lệnh chưa hỗ trợ), `2` (lỗi validation/conflict), `3` (endpoint unavailable/timeout);
   - Xuất dữ liệu JSON có cấu trúc qua `stdout` và thông điệp chẩn đoán qua `stderr`.
