# 02 — Technical Architecture

Tài liệu này xác định ranh giới kiến trúc, hợp đồng dữ liệu, cơ chế điều phối và chính sách cô lập của **agents-coworkers**.

---

## 1. Tổng quan hệ thống và Entrypoint sản phẩm

### 1.1. Mục tiêu kiến trúc
Hệ thống **agents-coworkers** là lớp điều phối cộng tác đa tác nhân (multi-agent workforce) hoạt động trên máy trạm cục bộ của kỹ sư, tận dụng:
1. **Agent Orchestrator (AO)**: Daemon quản lý vòng đời session, workspace git worktree, tiến trình agent, và giao tiếp agent native qua HTTP REST loopback. Mọi URL daemon đều bắt buộc phải dùng host loopback (`localhost`, `127.0.0.1`, hoặc `::1`). Endpoint được phân giải động theo thứ tự ưu tiên nghiêm ngặt (nguyên tắc fail-closed):
   - (1) Cờ lệnh tường minh: `--ao-url <URL>` (nếu truyền URL không hợp lệ hoặc không phải loopback thì fail-closed ngay lập tức, không fallback; xác minh `GET /api/v1/identity`; không dùng OS port inspection để suy PID; ghi nhận `aoDiscoverySource: "explicit_url"`, `aoPid: 0`, `aoPidStatus: "NOT_OBSERVED"`);
   - (2) Biến môi trường: `AO_BASE_URL` (nếu tồn tại nhưng không hợp lệ hoặc không phải loopback thì fail-closed, không fallback; xác minh `GET /api/v1/identity`; ghi nhận `aoDiscoverySource: "environment_url"`, `aoPid: 0`, `aoPidStatus: "NOT_OBSERVED"`);
   - (3) Biến môi trường: `AO_RUN_FILE` (đường dẫn tệp run metadata, ví dụ `running.json`, không phải URL; nếu tệp hỏng hoặc endpoint sai thì fail-closed; parse `running.json`, kiểm tra PID > 0 và tiến trình còn sống; tạo loopback URL từ port; xác minh `GET /api/v1/identity`; ghi nhận `aoDiscoverySource: "run_file"`, `aoPid`, `aoPidStatus: "VERIFIED"`);
   - (4) Các tệp candidate mặc định: `~/.ao/dev/running.json`, `~/.ao/running.json` (chỉ quét khi không có cả ba nguồn ưu tiên trên; quy tắc xác minh tương tự `AO_RUN_FILE`);
   - (5) Lưu ý hợp đồng PID: `/api/v1/identity` chỉ trả `hostId` và `apiVersion` (hoặc `contractVersion`), không trả PID. Chỉ có `running.json` mới chứa PID, port và startedAt. Do đó, chỉ tuyên bố `aoPidStatus: "VERIFIED"` khi endpoint đến từ `running.json`. Tuyệt đối không dùng OS-specific port inspection để suy PID khi URL được cấp trực tiếp;
   - (6) Nếu quét các candidate mặc định mà phát hiện nhiều hơn một daemon hợp lệ đang chạy thì **dừng lại ngay lập tức (fail-closed, exit 2)** và yêu cầu chỉ định rõ `--ao-url`. Nếu không tìm thấy daemon hợp lệ nào: exit 3. Cổng 3005 không phải là cổng kiến trúc cố định. Nếu endpoint hợp lệ nhưng không kết nối được hoặc timeout: exit 3. Nếu sai lệch parse/schema/identity/PID: exit 2.
2. **CLIProxyAPI**: Cổng định tuyến LLM cục bộ (mặc định lịch sử: `http://127.0.0.1:8317`, cấu hình được qua cờ `--gateway-url` hoặc biến môi trường `COWORKERS_GATEWAY_URL`, áp dụng cùng nguyên tắc fail-closed: bắt buộc phải dùng host loopback `localhost`, `127.0.0.1`, hoặc `::1`; nếu URL tường minh hoặc biến môi trường không hợp lệ thì dừng lại ngay, không fallback). Xác thực gateway sử dụng biến môi trường `CLIPROXY_KEY` khi endpoint yêu cầu authentication; không đưa khóa vào RunSpec, RunManifest, argv, stdout, stderr hoặc evidence; không ghi raw Authorization header; thiếu credential bắt buộc hoặc 401/403: exit 2; không kết nối được hoặc timeout: exit 3; gateway trả catalog/JSON không hợp lệ: exit 2.

```
+-----------------------------------------------------------------------------------+
|                              USER OPERATIONAL SURFACE                             |
|                                                                                   |
|   Target Entrypoint: `coworkers` (cmd/coworkers - PROPOSED / NOT IMPLEMENTED)     |
|   Legacy / Transition Tool: `recovery` (cmd/recovery - IMPLEMENTED)               |
+-----------------------------------------+-----------------------------------------+
                                          |
                                          v
+-----------------------------------------------------------------------------------+
|                        CONTROL PLANE & RECOVERY CORE                              |
|                                                                                   |
|   - Discovery & Health Preflight          - Task DAG & State Machine              |
|   - Workspace / Baseline Validation       - Autonomous Review-Rework Loop         |
|   - Run Spec & Run Manifest               - Sequential Integration Barrier        |
|   - Native Telemetry & Redaction          - Delivery Recovery (internal/recovery) |
+---------------------+---------------------------------------+---------------------+
                      |                                       |
       HTTP Loopback  |                        HTTP Loopback  |
    (Dynamic Discover)|                        (Default: 8317)|
                      v                                       v
+-----------------------------+         +-----------------------------+
|    AGENT ORCHESTRATOR       |         |        CLIPROXYAPI          |
|    (Daemon Runtime)         |         |      (Local Gateway)        |
|                             |         |                             |
| - Sessions & Worktrees      |         | - Account Pooling           |
| - Turns & Conversation      |         | - Model Routing             |
| - Git Worktree Diffs        |         | - Cooldown & Failover       |
| - Official SQLite (ao.db)   |         | - Trace ID Injection        |
+-----------------------------+         +-----------------------------+
```

### 1.2. Quyết định Entrypoint: Thin Control-Plane CLI (`coworkers`)
Theo quyết định kiến trúc [D014](03-DECISIONS.md#d014--target-product-entrypoint-cli-coworkers):
- **Tên entrypoint**: Giao diện người dùng mục tiêu của sản phẩm là lệnh CLI `coworkers`.
- **Ranh giới mỏng (Thin Client)**: `coworkers` là thin control-plane CLI, gọi trực tiếp AO backend và CLIProxyAPI qua các API loopback chính thức theo endpoint discovery policy đã được xác thực.
- **Không cạnh tranh trạng thái**: Hệ thống **không** xây dựng daemon riêng, **không** tạo cơ sở dữ liệu riêng, và tuyệt đối **không đọc/ghi trực tiếp** tệp SQLite `ao.db`. AO daemon là nguồn sự thật duy nhất (single source of truth) về phiên và tiến trình.
- **Tái sử dụng lõi phục hồi**: Toàn bộ logic checkpoint bền vững, write-ahead delivery uncertain, session preflight, và run lease được tái sử dụng trực tiếp từ package `internal/recovery`.
- **Phân tách quá độ**: `cmd/recovery` hiện là công cụ phục hồi hẹp và được duy trì tính tương thích ngược; mã nguồn mục tiêu sẽ đặt tại `cmd/coworkers` (PROPOSED / NOT IMPLEMENTED) mà không sao chép logic recovery.
- **Phân tách Hợp đồng Dữ liệu (RunSpec vs RunManifest)**:
  * `RunSpec` (định dạng JSON, input do người dùng cung cấp, `schemaVersion = "run-spec/v1-draft"`): bất biến sau khi run bắt đầu; phân biệt rõ `targetRoot` là checkout gốc của Product và `executionWorkspace` là worktree thực thi AO; `expectedBranch` sử dụng placeholder `<AO_SESSION_BRANCH>` (đại diện cho nhánh AO session/worktree dự kiến); chứa baseline SHA, profiles, policies, và delegated authority. Tuyệt đối không chứa `aoUrl` hay `gatewayUrl`; việc phân giải endpoint được định nghĩa qua `endpointPolicy` (không chứa URL). Chứa chính sách công cụ `requiredTools` (`git` luôn bắt buộc cho Slice 8A; các công cụ khác chỉ kiểm khi khai báo, kiểm tra trực tiếp qua `exec.LookPath`, tên công cụ không chứa path hoặc shell metacharacters; Go/Node không phải runtime prerequisite mặc định của target repo) và chính sách workspace `workspacePolicy` (`productRootMustBeClean: true`, `executionWorkspaceDirtyPolicy`: `require_clean` [mặc định] hoặc `allow_dirty_recorded`).
  * `RunManifest` (định dạng JSON, output sinh ra duy nhất từ `coworkers attach`, `schemaVersion = "run-manifest/v1-draft"`): chứa `runSpecSha256`, `generatedBy: "coworkers attach"`, resolved endpoints (aoUrl, aoIdentity, aoDiscoverySource, aoPid, aoPidStatus), `gatewayProbe` (url, catalogPath: "/v1/models", catalogStatus: "VERIFIED", observedModels, `providerCallPerformed: false`, `credentialEligibility: "NOT_OBSERVED"`), target repository identity, baseline SHA, current HEAD, `sessionBranch` và `worktreeBranch` (hai giá trị phải khớp nhau), `worktreeBinding` (canonicalPath, worktreeBranch, head, isClean, dirtyPolicy, porcelainSha256, verifiedPorcelain), đối tượng cấu hình duy nhất `attachedSessionProfile` (kind, harness, model, reasoningEffort, status), `lease` (workspaceRoot, runOwner, taskId: "__run__", ownerId, observedState: "FREE", observedPid: 0; không có trạng thái ACQUIRED), source provenance, và timestamps. Tuyệt đối không chứa secret (token, cookie, raw email, hoặc raw credential filename) và không chứa endpoint input dư thừa.
  * Vòng đời thống nhất của RunSpec và RunManifest:
    - `coworkers doctor`: Chuẩn hóa cú pháp: `coworkers doctor --spec <run-spec.json> [--ao-url <url>] [--gateway-url <url>] [--json]`. Cờ `--spec` là bắt buộc (thiếu, hỏng hoặc sai schema RunSpec: exit 2). Lệnh `doctor` không nhận cờ `--manifest`, không tạo hoặc sửa `RunManifest`, và không inspect hay acquire lease. Kiểm tra gateway reachable, `CLIPROXY_KEY` được chấp nhận, `/v1/models` trả catalog hợp lệ và model yêu cầu xuất hiện trong catalog (tuyệt đối không đồng nhất catalog với credential eligibility hay usable capacity; `providerCallPerformed: false`, `credentialEligibility: "NOT_OBSERVED"`). Kiểm tra runtime tools theo `requiredTools` (`git` bắt buộc; Go/Node không phải prerequisite mặc định của target repo).
    - `coworkers attach`: Đọc `RunSpec`. Nhận `--session`, `--workspace` và `--manifest` (đây là output path). Kiểm tra chính sách workspace dirty: Product root dùng raw `git status --porcelain=v1 -z` không có ngoại lệ (bất kỳ tệp nào kể cả dưới `.agents-coworkers/**` đều làm Product root dirty và exit code 2). Execution workspace đánh giá source-dirty từ raw porcelain (`git status --porcelain=v1 -uall -z`) sau khi loại duy nhất exact untracked porcelain record đã tính từ `RunSpec.runId` (`?? .agents-coworkers/recovery-leases/<sha256(runId + "\n__run__")>.lease`). Mọi trạng thái tracked, staged (`A `), modified (` M`, `M `), deleted (` D`, `D `), type changed, unmerged, rename hoặc copy tại cùng path vẫn được xem là source-dirty; không gọi mọi nội dung `.agents-coworkers/**` là expected; ngoại lệ lease duy nhất này chỉ tồn tại để giữ tính lũy đẳng (idempotency) khi cùng run chuyển trạng thái lease từ FREE→OWNED. `require_clean` + dirty worktree: exit code 2; `allow_dirty_recorded` + dirty worktree: PASS và ghi `porcelainSha256` là SHA-256 của canonical filtered source-porcelain này (không tự suy mọi dirty state là "expected"). Xác minh session/worktree theo thuật toán 9 bước bắt buộc. Gọi API inspect read-only kiểm tra lease (`workspaceRoot`, `runOwner`, `taskId: "__run__"`); tuyệt đối không gọi `Acquire` hoặc `Release` vì không có process run dài hạn giữ lease (`FREE`: tiếp tục; `OWNED` cùng run: idempotent readback; `LOCKED`: exit 2). Tạo `RunManifest` bằng cơ chế ghi nguyên tử (atomic write: temp file -> fsync/close -> rename). Nếu manifest chưa tồn tại: tạo mới (lệnh attach tự động tạo mới, không đòi hỏi tệp có sẵn từ trước). Nếu manifest đã tồn tại và khớp hoàn toàn (`runId`, `runSpecSha256`, session, repository và worktree binding): trả về thành công lũy đẳng (idempotent success), không viết lại. Nếu manifest bị hỏng hoặc sai lệch binding: dừng lại ngay (fail-closed) và thoát với exit code `2`.
    - `coworkers status`: Chỉ đọc `RunManifest` và trạng thái live read-only. Luôn re-inspect lease thực tế từ `workspaceRoot`, `runOwner`, `taskId: "__run__"`; tuyệt đối không tin `observedState` cũ trong manifest. Không tự ý phá stale lease, không kiểm PID còn sống để tự phá lease. Nếu manifest thiếu hoặc hỏng: thoát với exit code `2`. Không đọc `RunSpec` để tự tái tạo binding. Không chỉnh sửa manifest. Tuyệt đối không có trường Task DAG nào trong Slice 8A.
  * Cú pháp lệnh: `coworkers doctor --spec <run-spec.json> [--ao-url <url>] [--gateway-url <url>] [--json]`, `coworkers attach --spec <run-spec.json> --session <id> --workspace <path> --manifest <output-path> [--ao-url <url>] [--gateway-url <url>]`, `coworkers status --manifest <path-to-run-manifest.json> [--ao-url <url>] [--gateway-url <url>] [--json]` (triển khai thuần túy bằng thư viện chuẩn Go `encoding/json`, không thêm YAML dependency).
- **Thuật toán Worktree Binding bắt buộc cho `coworkers attach`**:
  Tuyệt đối không tin riêng `RunSpec` hoặc `RunManifest`. Lệnh `attach` phải thực hiện xác thực tuần tự:
  1. Đọc session qua AO API và lấy project ID + branch;
  2. Canonicalize target root và explicit execution workspace path;
  3. Chạy `git worktree list --porcelain -z` trên đúng repository;
  4. Tìm đúng một worktree có branch bằng session branch;
  5. Path canonical của worktree phải bằng execution workspace;
  6. HEAD của worktree phải bằng expected HEAD/baseline theo trạng thái run;
  7. Repository common-dir và identity phải khớp target repo;
  8. Nếu không có worktree, có nhiều kết quả, worktree ở trạng thái detached ngoài contract, hoặc sai lệch đường dẫn -> lập tức thoát với exit code `2`;
  9. Tuyệt đối không đọc `ao.db` và không suy physical path từ AO `SessionView`.
- **Ma trận Mã thoát chuẩn hóa**:
  * `0`: Thành công (success).
  * `1`: Lỗi CLI usage hoặc lệnh chưa hỗ trợ (`coworkers run` trong Slice 8A luôn trả về mã thoát `1`).
  * `2`: Lỗi kiểm tra / xác thực (validation, identity, baseline, worktree, hoặc lease conflict).
  * `3`: Endpoint không khả dụng hoặc hết thời gian chờ (endpoint unavailable hoặc timeout).
- **Phạm vi Slice 8A**: Chỉ thiết kế các khả năng kiểm tra và kiểm soát:
  * `coworkers doctor`: Khám sức khỏe môi trường, runtime tools (`git` luôn bắt buộc; các công cụ khác kiểm tra theo `requiredTools` qua `exec.LookPath` mà không dựng shell command; Go/Node không phải runtime prerequisite mặc định của target repo; invalid tool name fail-closed exit 2), target root (bắt buộc sạch), baseline SHA và profile catalog. doctor chỉ chứng minh gateway reachable, CLIPROXY_KEY được chấp nhận, `/v1/models` trả catalog hợp lệ và model có trong catalog; không gọi provider completion, không nâng claim credential eligibility hay usable capacity (`providerCallPerformed: false`, `credentialEligibility: "NOT_OBSERVED"`). doctor không tạo manifest và không inspect hay acquire lease.
  * `coworkers attach`: Kiểm tra chính sách workspace dirty: Product root dùng raw `git status --porcelain=v1 -z` không có ngoại lệ (bất kỳ tệp nào kể cả dưới `.agents-coworkers/**` đều làm Product root dirty và exit code 2). Execution workspace đánh giá source-dirty từ raw porcelain (`git status --porcelain=v1 -uall -z`) sau khi loại duy nhất exact untracked porcelain record đã tính từ `RunSpec.runId` (`?? .agents-coworkers/recovery-leases/<sha256(runId + "\n__run__")>.lease`). Mọi trạng thái tracked, staged (`A `), modified (` M`, `M `), deleted (` D`, `D `), type changed, unmerged, rename hoặc copy tại cùng path vẫn được xem là source-dirty; không gọi mọi nội dung `.agents-coworkers/**` là expected; ngoại lệ lease duy nhất này chỉ tồn tại để giữ tính lũy đẳng (idempotency) khi cùng run chuyển trạng thái lease từ FREE→OWNED. `require_clean` + dirty worktree: exit code 2; `allow_dirty_recorded` + dirty worktree: PASS và ghi `porcelainSha256` là SHA-256 của canonical filtered source-porcelain này (không tự suy mọi dirty state là "expected"). Gắn kết read-only/control-plane vào session/project hiện có theo đúng 9 bước worktree binding. Gọi API inspect read-only kiểm tra lease (`FREE`, `OWNED`, `LOCKED`); tuyệt đối không gọi `Acquire` hoặc `Release`.
  * `coworkers status`: Chỉ báo cáo `RunManifest`, endpoint identity, single attached session/profile, workspace/baseline, và run lease. Luôn re-inspect lease từ `workspaceRoot`, `runOwner`, `taskId: "__run__"`, không tin `observedState` cũ trong manifest; không phá stale lease. Tuyệt đối không có trường Task DAG nào trong Slice 8A.
  * Run Lease: `FileLease` theo source thật đặt tại `<executionWorkspace>/.agents-coworkers/recovery-leases`, tên file suy từ SHA-256(`runOwner + "\n" + taskId`). Slice 8A dùng run-level lease identity cố định: `runOwner = RunSpec.runId`, `taskId = "__run__"`, `ownerId = RunSpec.runId`, `workspaceRoot = canonical executionWorkspace`. Trạng thái quan sát được: `FREE` (lease file không tồn tại), `OWNED` (lease tồn tại và khớp run), `LOCKED` (lease thuộc owner/task khác). Tuyệt đối không dùng trạng thái `ACQUIRED` trong manifest Slice 8A. `FileLease` dùng `O_EXCL` để phối hợp độc quyền giữa các process `coworkers`/`recovery` cùng tuân thủ wrapper contract; nó không phải global AO lock và không ngăn client AO bên ngoài sử dụng session. Không có TTL và không tự phá stale lease. PID chỉ là metadata giám sát, không kiểm tra PID còn sống để tự phá lease. Slice 8A chỉ bổ sung API inspect read-only nhận đủ `workspaceRoot`, `runOwner`, và `taskId` cần thiết cho status và attach, không thay đổi ngữ nghĩa Acquire/Release.
  * Gate 8A Oracle: Executable CLI invocation là oracle bắt buộc ở cấp độ L4 (Local Process Smoke). Unit tests (L2) và integration giả lập (L3) chỉ là bằng chứng bổ trợ.
  * Danh sách tệp cho phép chính xác của Slice 8A (Exact File Allowlist — 17 files, không dùng wildcard):
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
    Không sửa `go.mod`/`go.sum`; chỉ sử dụng thư viện chuẩn của Go.

---

## 2. Mô hình Task DAG và Vòng lặp tự vận hành (Slice 8B)

### 2.1. Cấu trúc Task Graph (DAG)
Mỗi run được đặc tả thành một Đồ thị có hướng không chu trình (Directed Acyclic Graph - DAG) gồm các Task Nodes bền vững:
- `TaskID`: Định danh duy nhất trong run (ví dụ `task-001-worker-a`).
- `Dependencies`: Danh sách các `TaskID` tiên quyết phải đạt trạng thái `ACCEPTED` trước khi task này đủ điều kiện kích hoạt.
- `Owner`: Vai trò được giao (Orchestrator, Worker A..N, Integration Worker).
- `AssignedSession` & `AssignedWorktree`: Phiên AO và worktree Git độc quyền cho lane làm việc.
- `FileAllowlist`: Danh sách đường dẫn tệp được phép sửa đổi; mọi vi phạm ghi ngoài allowlist đều dẫn đến fail-closed.
- `ResourceNamespace`: Không gian tài nguyên cô lập (cổng mạng, schema database tạm, tệp chứng chỉ).
- `DeliveryID` & `ClientMessageID`: Khóa idempotent bảo vệ việc gửi thông điệp tới AO.
- `ArtifactSHA` & `GitHead`: Bằng chứng commit và digest tệp bàn giao.
- `TestReceipt`: Biên nhận kết quả kiểm thử độc lập (lệnh chạy, mã thoát, log hash).
- `ReviewVerdict`: Phán quyết của Orchestrator (`PENDING`, `ACCEPTED`, `REWORK`).
- `RetryBudget`: Số lần cho phép retry/rework trước khi chuyển sang `BLOCKED`.
- `IntegrationBarrier`: Rào chắn đồng bộ hóa ngăn việc ghép mã nguồn khi chưa có đủ review hợp lệ.

### 2.2. Vòng lặp tự vận hành (Autonomous Loop State Machine)

```
        +-------------------------------------------------------------+
        |                          DISPATCH                           |
        |  Scheduler chọn 1-3 workers theo DAG & capacity khả dụng    |
        +------------------------------+------------------------------+
                                       |
                                       v
        +-------------------------------------------------------------+
        |                          EXECUTION                          |
        |  Worker chạy trên worktree riêng, commit Git & sinh receipt |
        +------------------------------+------------------------------+
                                       |
                                       v
        +-------------------------------------------------------------+
        |                     ORCHESTRATOR REVIEW                     |
        |  Completion kích hoạt Orchestrator kiểm tra diff & test log |
        +------------------------------+------------------------------+
                                       |
                      +----------------+----------------+
                      |                                 |
              Verdict = REWORK                  Verdict = ACCEPT
                      |                                 |
                      v                                 v
        +---------------------------+     +---------------------------+
        |          REWORK           |     |    INTEGRATION BARRIER    |
        | Quay lại đúng worker /    |     | Mở khóa dependencies &    |
        | session / worktree cũ     |     | cho phép tuần tự ghép mã  |
        +-------------+-------------+     +-------------+-------------+
                      |                                 |
                      +---------------+                 v
                                      |   +---------------------------+
                                      |   |   SEQUENTIAL INTEGRATION  |
                                      +-->| Ghép commit vào branch run|
                                          | Chạy test tích hợp chung  |
                                          +---------------------------+
```

1. **Dispatch & Chính sách Concurrency**:
   - **Mức mặc định tối đa**: Tối đa **3 workers đồng thời** (mức tối đa đã được kiểm chứng thực tế tại Gate 10 / Phase 6).
   - Scheduler 8B lựa chọn từ **1 đến 3 workers** dựa trên các task sẵn sàng trong DAG và dung lượng khả dụng của pool.
   - Mức **4–7 workers** được phân loại là **`EXPERIMENTAL_CAPACITY`**, chỉ được phép kích hoạt khi: (1) telemetry 8C cung cấp đủ căn cứ; (2) có authority riêng từ user; (3) có chứng minh cô lập tài nguyên/worktree hoàn chỉnh; và (4) thực hiện capacity proof riêng biệt. Tuyệt đối không mô tả 1–7 như dải năng lực sản xuất mặc định.
2. **Execution**: Worker thực thi nhiệm vụ trong phạm vi `FileAllowlist` trên git worktree riêng, thực hiện commit cục bộ và sinh `TASK_RECEIPT`.
3. **Review**: Khi worker hoàn tất turn, Orchestrator được kích hoạt tự động để đối chiếu diff git, kiểm tra biên nhận test và mã thoát.
4. **Phán quyết REWORK & Ranh giới Không tạo lỗi giả trong Product**:
   - Trong quá trình phát triển và kiểm chứng state machine (Gate 8B), oracle rework sử dụng **fixture cô lập có contract violation được seed trước**, không yêu cầu tạo lỗi nhân tạo trong Product repository.
   - Khi chạy trên workload thật của Product (Slice 8D), Orchestrator chỉ phát chỉ thị REWORK khi phát hiện lỗi kỹ thuật thật sự trong mã nguồn hoặc kết quả test của worker.
   - Task **bắt buộc** phải quay lại đúng worker/session/worktree ban đầu nếu phiên còn hợp lệ.
5. **Phán quyết ACCEPT**: Khi đáp ứng đầy đủ tiêu chí, Orchestrator phê duyệt task, giải phóng các task phụ thuộc và mở rào cản tích hợp.
6. **Sequential Integration**: Tác nhân tích hợp (Integration Worker) thực hiện tích hợp tuần tự các commit đã được duyệt vào branch tích hợp của run (`integration/run-<id>`) trên một worktree tích hợp riêng biệt, sau đó chạy test tổng thể.
7. **Bảo toàn khi sự cố & Human Stop**:
   - Nếu tiến trình gặp sự cố (crash), cơ chế phục hồi sử dụng checkpoint ghi trước (`DELIVERY_UNCERTAIN`) và gửi thăm dò `recover-only` cùng `ClientMessageID` để khôi phục trạng thái mà không phát lại side effect đã thực hiện.
   - Khi người dùng phát tín hiệu Stop, task lập tức chuyển sang trạng thái `CANCELLED` (terminal) và fail-closed toàn bộ vòng lặp.

---

## 3. Quan sát định tuyến và Năng lực Native (Slice 8C)

### 3.1. Ma trận Năng lực Native (Native Seams Matrix)
Căn cứ vào kết quả audit trực tiếp mã nguồn của `CLIProxyAPI` (checkout tại commit exact `2430354330af80b645f9ffb1a51e1e7c72c4cc8e`; lưu ý: việc audit mã nguồn không chứng minh binary runtime đang chạy được biên dịch từ đúng commit này) và `agent-orchestrator`:

| Năng lực / Tín hiệu Quan sát | Nguồn Seam (CLIProxyAPI / AO) | Phân loại | Khả năng quan sát & Ranh giới an toàn |
| :--- | :--- | :--- | :--- |
| **Downstream Trace & Credential Correlation** | Header `X-CPA-TRACE-ID` (`cpa_trace.go:FormatCPATraceID`) | `AVAILABLE` | Header downstream có định dạng `<YYYYMMDDHHMMSS>-<authIndex>-<requestID>`. Trong đó `authIndex` là chuỗi băm sha256 16-hex (`stableAuthIndex`) từ seed của credential. Có rủi ro liên kết tài khoản (correlation risk), không chứa raw token nhưng là metadata vận hành nhạy cảm; bắt buộc phải lọc (redaction) và lưu trữ có giới hạn (bounded retention); không được coi là an toàn tuyệt đối. Có thể vắng mặt khi chưa chọn được credential hoặc response không đi qua trace callback; khi vắng mặt phải ghi nhận `NOT_OBSERVED` hoặc `UNKNOWN`. |
| **Rate Limiting & Cooldown Window** | Header `Retry-After` (`handlers_errors.go`, `SafeResponseHeaders`) | `AVAILABLE` | Chỉ đọc khi thực sự xuất hiện trong response header từ gateway/upstream khi gặp lỗi `429` hoặc `503`. Không giả định mọi lỗi 429/503 đều có `Retry-After`. |
| **HTTP Status Code Classification** | Mã trạng thái HTTP downstream (`402`, `403`, `429`, `503`, `200`) | `AVAILABLE` | Quan sát trực tiếp trên client. Tuyệt đối không tự động ánh xạ mọi 402 thành `deactivated_workspace` hay mọi 403 thành `VALIDATION_REQUIRED` (đây chỉ là observed subcodes ở một số run lịch sử). Bộ phân loại lỗi phải kết hợp status code + structured error code/reason trong body + headers; thiếu dữ liệu thì phân loại `UNKNOWN`. |
| **AO Session & Turn Introspection** | Endpoint `/api/v1/sessions/{id}`, `/conversation`, `/settings` | `AVAILABLE` | Truy vấn trực tiếp trạng thái turn, model readback, reasoning effort qua REST API loopback của AO daemon. Lưu ý: SessionView không trả về đường dẫn worktree vật lý. |
| **Gateway Static Cooldown Config** | Tệp cấu hình `config.yaml` / `config.runtime.yaml` | `CONFIG_ONLY` | Cấu hình tĩnh về thời gian cooldown và giới hạn retry. Không thể quan sát động từng lần chuyển đổi trạng thái qua header. |
| **Dynamic Credential Remaining Quota** | Bộ nhớ nội bộ (in-memory) của CLIProxyAPI | `NOT EXPOSED` | Gateway không cung cấp trường header nào phản ánh số lượng quota còn lại của từng credential cho client downstream. |
| **In-flight Failover Attempts Trail** | Vòng lặp retry nội bộ của conductor trong CLIProxyAPI | `NOT EXPOSED` | Gateway tự động thử xoay vòng các credential khi gặp lỗi, nhưng downstream response chỉ mang trace của credential cuối cùng, không kèm lịch sử các credential đã thử trước đó. |
| **Safe Gateway Log Adapter** | Bộ chuyển đổi đọc tệp log của CLIProxyAPI | `ADAPTER_CANDIDATE` | Adapter đọc log cục bộ đã được lọc và ẩn danh (redacted) để thu thập đường dẫn failover mà không sửa mã nguồn gateway. |
| **Failover Trail Downstream Header** | Bản vá bổ sung header downstream tại CLIProxyAPI | `PATCH_CANDIDATE` | Đề xuất bổ sung downstream header tùy chọn `X-CPA-Failover-Attempts` nếu cần quan sát chính xác số lần failover. Chỉ là phương án ứng viên sau khi có audit riêng và được duyệt; không phải hướng mặc định. |

### 3.2. Nguyên tắc Telemetry an toàn
- **Thứ tự ưu tiên giải pháp**: (1) Sử dụng API và headers native sẵn có; (2) Xây dựng client adapter an toàn; (3) Bản vá header chỉ là phương án ứng viên sau audit riêng và approval.
- **Không giả định**: Tuyệt đối không giả định `agents-coworkers` có thể nhìn thấy credential, cooldown, hoặc failover trail nếu gateway chưa cung cấp qua response header chính thức.
- **Dữ liệu lưu trữ tối thiểu**: Bản ghi telemetry chỉ được phép lưu trữ: credential ID đã băm (16-hex), số lượt thử (attempt), model ID, provider, nhóm trạng thái (status category), thời gian cooldown (nếu có `Retry-After`), và `X-CPA-TRACE-ID`. Tuyệt đối không ghi nhận API keys, tokens, cookies, hoặc thông tin cá nhân.
- **Phân loại lỗi chặt chẽ**:
  * Kết hợp HTTP status code, structured error code/reason trong JSON body và response headers.
  * Không suy diễn: thiếu dữ liệu phân loại là `UNKNOWN`.
- **Thay thế chỉ tiêu coverage**: Không áp dụng chỉ tiêu coverage hình thức một cách máy móc; thay thế bằng ma trận hành vi và xử lý lỗi (behavior/error matrix) bao quát mọi tình huống trả về của gateway.

---

## 4. Cách ly Workload và Ranh giới Tích hợp (Slice 8D)

### 4.1. Quy tắc bất biến đối với Product Repository
- **Product Root Read-Only (`INV-001`)**: Thư mục gốc (`Product ROOT checkout`) của `AI Auto Video Creator` luôn luôn ở chế độ **CHỈ ĐỌC (READ-ONLY)**. Mọi thao tác ghi trực tiếp vào Product root đều bị chặn đứng và coi là vi phạm an toàn nghiêm trọng.
- **Quyền ghi trên Worktree cô lập**: Các git worktree cô lập được cấp quyền cho run có thể thực hiện ghi theo danh mục allowlist cụ thể của từng run. `INV-001` bảo vệ product root và không cấm các Product worktree được cấp quyền.
- **Kiểm kê trước khi chạy (Pre-run Inventory)**: Trước khi kích hoạt bất kỳ run nào trên workload thật, hệ thống phải kiểm kê đầy đủ Product root, các worktree AO cũ còn tồn tại, commit baseline, dirty diff hiện có, và quyền hạn (authority) được cấp.
- **Workload có nguồn gốc xác thực**: Nhiệm vụ workload bắt buộc phải được trích xuất từ tài liệu kiến trúc, roadmap, hoặc contract có thẩm quyền của AI Auto Video Creator sau khi thực hiện audit chỉ đọc.

### 4.2. Cơ chế Worktree và Ranh giới Tự Merge
- **Worktree riêng cho Worker**: Mỗi worker hoạt động trên một git worktree hoàn toàn độc lập, tách biệt khỏi Product root và các worker khác.
- **Integration Worktree riêng**: Quá trình tích hợp diễn ra trên một worktree tích hợp riêng biệt dành cho run.
- **Định nghĩa "Tự merge"**: "Tự merge" trong phạm vi workforce **CHỈ CÓ NGHĨA** là tự động tích hợp các commit đã được Orchestrator chấp thuận (`ACCEPTED`) vào branch tích hợp cục bộ của run (`integration/run-<id>`).
- **Nghiêm cấm Push vào Product Main**: Tác nhân **TUYỆT ĐỐI KHÔNG** tự động merge hoặc push mã nguồn vào branch `main` của Product repository. Mọi quá trình tích hợp sản phẩm thực tế dừng lại ở bước lập báo cáo audit đầy đủ và bàn giao cho người dùng quyết định.

---

## 5. Mở rộng Dung lượng dựa trên Telemetry (Slice 8E)

### 5.1. Định vị lại Pool hiện có
- Pool tài khoản hiện thời (**1 Codex + 7 Gemini**) là dữ liệu **inventory lịch sử** từ các đợt thử nghiệm trước ([run-20260923-phase8-pool-onboarding](../evidence/run-20260923-phase8-pool-onboarding/summary.md)), không đồng nghĩa với việc toàn bộ các tài khoản này đang sẵn sàng hoặc có thể sử dụng (usable capacity).
- Mục tiêu danh nghĩa ban đầu (6 Plus + 8 Pro) chỉ là mục tiêu lịch sử đã superseded; không coi đây là điều kiện tiên quyết hay tiêu chí đóng Phase 8.

### 5.2. Chính sách Onboarding dựa trên Dữ liệu thực tế (Slice 8E Conditional)
- **Slice 8E là conditional slice**: Nếu telemetry từ Slice 8C và 8D chứng minh pool hiện tại đủ dùng với tỷ lệ lỗi thấp, trạng thái Slice 8E là `NOT_TRIGGERED` hoặc `NOT_REQUIRED_WITH_EVIDENCE`; không cần onboarding thêm tài khoản và Phase 8 vẫn có thể đóng hợp lệ.
- Chỉ tiến hành quy trình onboarding tài khoản mới khi dữ liệu đo lường (telemetry) từ Slice 8C và 8D chứng minh sự thiếu hụt dung lượng thực tế (tỷ lệ 429 quota cao, hàng đợi task tắc nghẽn) đối với một model hoặc profile cụ thể, và được người dùng cấp authority riêng.
- Sử dụng đúng các công cụ và đường dẫn tệp thực tế trong kho mã nguồn:
  * Script kiểm kê: `scripts/auth-inventory.ps1`
  * Thư mục cấu hình gateway: `config/cliproxy/`
  * Tuyệt đối không sử dụng các đường dẫn không tồn tại.
