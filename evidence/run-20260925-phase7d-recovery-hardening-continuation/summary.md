# Phase 7D preflight and cancellation hardening

Status: `SIMULATED_PASS__LIVE_DISPOSABLE_SESSION_NOT_OBSERVED`. Worktree `phase7d-recovery-prototype`, baseline `0190e925cd552b936250c74c29cbc272bb477e49`; Product, AO/CLIProxyAPI upstream và daemon/session dùng chung không bị sửa.

## Contract đã sửa

- AO preflight giải mã đúng `SessionResponse{session: SessionView}`. Fixture JSON bám DTO `SessionResponse`, `SessionView` và `ConversationSnapshotResponse` của AO source.
- Fail-closed khi checkpoint/readback thiếu hoặc lệch session ID, project, kind, harness, model, branch, effort hay controller `ready`. Effort lấy từ `conversation.settings.reasoningEffort`; artifact reader khóa SHA-256, Git HEAD và branch.
- Checkpoint lịch sử trỏ `ao-phase5-repo-21` đã bị loại khỏi testdata; live checkpoint chỉ được tạo từ session disposable mới và branch artifact tương ứng.
- Cancel ghi local terminal intent trước I/O. Nếu acceptance response mất, nó recover-only bằng cùng delivery ID, lấy đúng turn, xác minh turn đó là live turn duy nhất rồi mới interrupt.
- Không xác nhận được AO Stop giữ `CANCELLED_LOCAL_AO_STOP_UNCONFIRMED`; `Step` qua restart thực hiện 0 Send/RecoverOnly. Chỉ Cancel tường minh mới đối chiếu lại. Interrupt response-loss được xác nhận bằng Observe terminal khi có evidence.
- Lease constructor cố định thư mục dưới workspace; tên file là SHA-256(run owner + task ID), O_EXCL. Caller không truyền lease path tùy ý; stale lease không tự bị phá.

## Oracle

- Response envelope/identity/effort/controller missing: fail-closed PASS (8 subcases).
- Lost acceptance → Cancel: RecoverOnly=1, exact owned turn interrupted, ordinary resend=0, restart terminal PASS.
- Interrupt response lost nhưng subsequent Observe=STOPPED: AO Stop xác nhận PASS.
- Recover inconclusive: state unconfirmed, no interrupt of unrelated turn, restart Step=0 I/O PASS; later explicit Cancel recovers exact turn PASS.
- Adapter refuses interrupt when another live turn makes ownership ambiguous PASS.
- Lease race across two wrapper owners sharing run/task: exactly one sender PASS.

Verification: 36 top-level tests PASS; `go test -race`, `go vet`, full `go test ./...` và `git diff --check` exit 0. Edited stdout/stderr with individual test names is in `verification.txt`.

## LIVE boundary và AO Stop

AO shared status is `stopped`; no isolated AO daemon/process with eligible credential was present. Starting the shared daemon or copying private credential into a new daemon would violate this run's boundary. Therefore no disposable live session was created: response-loss/Cancel/Stop results are `SIMULATED`, actual AO provider Stop is `NOT OBSERVED`.

Smallest next step: provide an already-running isolated AO daemon with an eligible disposable project/session. Create a new checkpoint from its authoritative session/conversation readback, inject only post-acceptance response loss, then verify recover-only and exact-turn Stop without touching shared AO state.
