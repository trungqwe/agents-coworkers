# Phase 7E — Sửa test portability, hoàn tất integration và executable self-host LIVE

Trạng thái: `PHASE_7E_PORTABILITY_SELFHOST_PASS`.
Run `p7e-selfhost-20260926b`, baseline `4dfbf9905ff12c363307e5974fab719a0911b114`, combined input `cfa5890f1a585f97743475c7d04014b031caa458`, final integration commit `e80958331900b1fdd5e1190c0a75500bb1057157`.
Nguồn: AO daemon cô lập tại `127.0.0.1:3005`, CLIProxyAPI gateway tại `127.0.0.1:8321` (config SHA-256 `da5abd67227b050b267baa189cb348b5e08384a15d60365dbd151c300b2af0f9`). Nguồn auth: `shared_read_only` (không lưu đường dẫn/khóa nhạy cảm).

## 1. Task Graph và Commit Chain

- Baseline commit: `4dfbf9905ff12c363307e5974fab719a0911b114`
- Worker A commit: `56374e9e033d693db16f064fcfb46959556a3e2a` (runner loop & backoff)
- Worker B commits (Rework thực tế OBSERVED):
  - `51521e204369a471447fbafca0988ad17799518d` (status formatting & recovery CLI)
  - `b5bc9ef4e2d312da9b398be80a6b6378db7ad143` (exit code matrix & CLI tests)
  - `cfa5890f1a585f97743475c7d04014b031caa458` (timeout step fix)
- Worker C integration rework commit: `e80958331900b1fdd5e1190c0a75500bb1057157`
  - Message: `fix(recovery): make GitArtifactReader test deterministic with isolated fixture repo`
  - Scope: Duy nhất `internal/recovery/dispatcher_test.go` (1 file changed, 61 insertions(+), 7 deletions(-)). Zero production code edits.
  - Parent chain: `e809583` -> `cfa5890` -> `b5bc9ef` -> `51521e2` -> `56374e9` -> `4dfbf99`.

## 2. Test Fix và Lý do

- **Nguyên nhân gốc của blocker**: Test `TestGitArtifactReaderBindsHashAndHead` trong Phase 7D prototype gán cứng kỳ vọng `got.Branch != "phase7d-recovery-prototype"` và trỏ thẳng vào repo gốc đang chạy test. Khi chạy trong worktree AO cô lập hoặc commit detached, giá trị branch trả về khác hoặc rỗng, làm fail assertion dù logic đọc artifact và Git HEAD hoàn toàn chính xác.
- **Giải pháp deterministic**:
  - Tạo Git repo riêng trong `t.TempDir()`.
  - Tạo branch fixture có tên cố định thuộc riêng test: `fixture-test-branch`.
  - Cấu hình local Git identity fixture: `user.name = "Test User"`, `user.email = "test@example.com"`.
  - Tạo và commit artifact fixture trong repo tạm.
  - Lấy độc lập: expected artifact SHA-256 (`crypto/sha256`), expected Git HEAD (`git rev-parse HEAD`), expected branch (`fixture-test-branch`).
  - Gọi `GitArtifactReader.Observe()` trên repo fixture và assert nghiêm ngặt: SHA-256 chính xác, Git HEAD chính xác, Branch chính xác và không rỗng.
  - Giữ nguyên kiểm tra path traversal bị từ chối.
  - Không sửa production code, không special-case detached HEAD trong dispatcher, không làm yếu oracle.

## 3. Kết quả Verification Bắt buộc

1. **Focused Test**: `go test -count=1 ./internal/recovery -run TestGitArtifactReaderBindsHashAndHead -v`
   - Exit code: 0, PASS (0.27s).
2. **Full Suite**: `go test -count=1 ./...`
   - Exit code: 0, PASS (cmd/recovery: 3.552s, internal/recovery: 1.764s, tests/integration: 0.210s).
3. **Race Detection**: `go test -race -count=1 ./internal/recovery`
   - Exit code: 0, PASS (2.794s).
4. **Static Analysis**: `go vet ./...`
   - Exit code: 0, clean.
5. **Build**: `go build -o <RUN_TEMP>/recovery.exe ./cmd/recovery`
   - Exit code: 0, binary 9,726,976 bytes, SHA-256: `8c069d69d06dc046f8d6bc81ac348700474a527b8d86aad53e981e133bac2ad7`.
6. **Git Diff Check**: `git diff --check`
   - Exit code: 0, clean.
7. **Diff Audit**: Đúng 1 file `internal/recovery/dispatcher_test.go` được cấp quyền ngoài các commit A/B đã ghép.

## 4. Orchestrator Review và Phê duyệt

- Orchestrator session `p7e-continuation-1` đọc diff, test outputs và commit `e809583`.
- Phát review verdict `ACCEPT` qua AO conversation API (turn `f89d48b1-e951-490d-958f-f3c3588116aa`).
- Chỉ sau khi ACCEPT mới tiến hành chạy executable self-host.

## 5. Self-Host LIVE Facts trên AO Session Disposable

- Session disposable: `p7e-continuation-3` (project: `p7e-continuation`, kind: `worker`, harness: `codex`, model: `gemini-3.7-flash-high/high`, branch: `ao/p7e-selfhost-worker`).
- Checkpoint read-only ban đầu:
  - Task ID: `P7E-SELFHOST-TASK-001`, Delivery ID: `p7e-selfhost-delivery-001`, State: `WAITING_RETRY`.
  - Artifact: `go.mod` (SHA-256: `0f96d491be3e33b707ac0e91e14a2355127a1a76b92f44cdf09d7168bbff9de5`).
  - Git HEAD: `e80958331900b1fdd5e1190c0a75500bb1057157`.
- **Preflight LIVE**: Executable `recovery.exe` xác thực chính xác project ID, session ID, kind, harness, model (`gemini-3.7-flash-high`), effort (`high`), branch (`ao/p7e-selfhost-worker`), và lease độc quyền.
- **Delivery LIVE**: Gửi delivery qua AO `/conversation/steer-or-send`, turn ID `5f8b27a5-d88e-4c1e-a373-5635c898d310`. Checkpoint tự động chuyển `DELIVERED`.
- **Execution & Receipt LIVE**: Model đọc `go.mod` và phát receipt chính thức:
  `TASK_RECEIPT {"taskId":"P7E-SELFHOST-TASK-001","artifactSha256":"0f96d491be3e33b707ac0e91e14a2355127a1a76b92f44cdf09d7168bbff9de5","accepted":true}`
- **Completion LIVE**: Executable `recovery.exe` quan sát turn hoàn tất, bóc tách receipt, đối chiếu taskId và artifact SHA-256, chuyển trạng thái checkpoint thành `COMPLETED` và `SideEffect: COMPLETED`. Exit code: 0.
- **Exit Code Matrix**:
  - Exit 0 [LIVE]: `recovery status` thành công; `recovery run` quan sát turn hoàn tất và chuyển checkpoint thành `COMPLETED`.
  - Exit 1 [LOCAL_EXECUTABLE/SIMULATED]: Lệnh `recovery cancel` bị từ chối hợp lệ (unsupported CLI command).
  - Exit 2 [LOCAL_EXECUTABLE/SIMULATED]: `recovery run` trên task terminal `BLOCKED` báo lỗi terminal chính xác (`ARTIFACT_CHANGED`).
  - Exit 3 [LOCAL_EXECUTABLE/SIMULATED]: `recovery run` với timeout 90s trên task chưa terminal báo timeout chính xác (`context deadline exceeded`).
- Không có ordinary resend ngoài hợp đồng; không dùng source invocation thay cho executable.

## 6. Phân loại Chứng cứ

- **LIVE**:
  - Deterministic test fix và full suite PASS.
  - Orchestrator review ACCEPT qua AO API.
  - Preflight kiểm tra project/session/model/effort/branch/HEAD/SHA-256.
  - Executable recovery `status` và `run` chuyển dịch state `WAITING_RETRY` -> `DELIVERED` -> `COMPLETED`.
  - Parse `TASK_RECEIPT` và đối chiếu SHA-256 thật từ provider task hoàn tất (Exit 0).
  - Repo portability và executable self-host LIVE trên repo thứ hai `agents-coworkers`.
- **LOCAL_EXECUTABLE / SIMULATED**:
  - Exit 1: `recovery cancel` CLI usage error (exit 1).
  - Exit 2: `recovery run` trên fixture checkpoint `BLOCKED` (exit 2).
  - Exit 3: `recovery run` timeout 90s hết hạn trước terminal state (exit 3; command `recovery run -checkpoint <RUN_TEMP>/checkpoint.json -workspace <RUN_TEMP>/ao-data/worktrees/p7e-continuation/p7e-continuation-3 -ao-url http://127.0.0.1:3005 -poll 1s -timeout 90s`, thực tế timeout 90s, nguồn trạng thái `context deadline exceeded`; không báo 1ms).
- **NOT OBSERVED**:
  - Provider outage recovery LIVE: NOT OBSERVED.
  - Daemon restart recovery LIVE: NOT OBSERVED.
  - Stop LIVE: NOT OBSERVED.
  - Exact-turn Stop trên shared session: unsupported/fail-closed.

## 7. Coordinator Intervention và Approval Provenance

- Coordinator bootstrap runtime, cấu hình `approvalMode=accept-edits` và `reasoningEffort=high` qua AO API.
- Tự động duyệt allow-once cho các lệnh đọc `go.mod`, kiểm tra git/file trong scope.
- Từ chối lệnh out-of-scope hoặc sửa ngoài file được phân công.
- Không dùng allow-all, không dump environment chứa secrets.

## 8. Trạng thái Phase 7E và Giới hạn Còn lại

- **Trạng thái**: Phase 7E đạt `COMPLETE`. Toàn bộ Exit Criteria Phase 7 đã hoàn thành với accepted limitations.
- **Giới hạn chấp nhận (Accepted Limitations)**:
  - Provider outage / network partition tự phục hồi unattended: `NOT OBSERVED`.
  - Daemon restart crash giữa chừng và khôi phục lease: `NOT OBSERVED`.
  - User Stop LIVE: `NOT OBSERVED`.
  - Exact-turn Stop trên shared session: unsupported và fail-closed.
  - Dừng tại audit trước push/merge. Không push lên origin hoặc merge vào main.
