# Phase 7D recovery hardening continuation

Status: `SIMULATED_PASS__LIVE_DISPOSABLE_SESSION_NOT_OBSERVED`. Worktree `phase7d-recovery-prototype`, baseline `0190e925cd552b936250c74c29cbc272bb477e49`; Product và AO/CLIProxyAPI upstream không bị sửa.

## Kết quả oracle

- `DELIVERED → Observe error → restart → NOT_FOUND`: giữ `DELIVERED`, backoff reconciliation, sau đó BLOCK `DELIVERY_RECEIPT_LOST`; ordinary Send = 0.
- `DELIVERY_UNCERTAIN → Observe error → restart → NOT_FOUND`: giữ uncertain; đúng deadline chỉ RecoverOnly; ordinary Send = 0.
- AO `outcome=sent` nhận `turnId`; `outcome=steered` fail-closed thành `SESSION_CONTAMINATED`.
- Preflight kiểm session ID/project/kind/harness/model, `isTerminated=false` và conversation controller `ready`. File lease `O_EXCL` run-owned bao quanh preflight + POST; race hai wrapper cho đúng một Send.
- Observe đi qua `beforeSequence`/`oldestSequence` cho tới đúng turn hoặc `hasMoreBefore=false`; cursor không tiến là lỗi đối chiếu, không phải authoritative NOT_FOUND.
- AO `cancelled`/`interrupted` và dispatcher Cancel đều thành terminal `CANCELLED`. Cancel được persist trước `/conversation/interrupt`; mất response interrupt không phục hồi task.
- AO turn `completed` chỉ đóng task khi `TASK_RECEIPT` khớp task ID, artifact SHA-256 và `accepted=true`.
- Transport response loss sau simulated AO acceptance: restart giữ cùng `clientMessageId`, POST kế tiếp là `recoverOnly`, receipt đúng task/artifact, ordinary Send = 1 ban đầu và resend = 0.

## Verification

`go test -count=1 -v ./internal/recovery` exit 0; 31 top-level tests PASS cùng các subtest. `go test -race -count=1 ./internal/recovery`, `go vet ./...`, `git diff --check` đều exit 0. Output biên tập ở `verification.txt`.

## LIVE boundary

`ao status --json` báo daemon dùng chung `stopped`. Không khởi động/restart daemon dùng chung và không nhập credential/private config vào daemon cô lập, nên không thể tạo session AO disposable live mà vẫn giữ session/resource dùng chung nguyên trạng. Proof transport-after-acceptance ở lượt này là `SIMULATED`, không phải `LIVE`; provider failure, daemon restart và unattended wake vẫn `NOT OBSERVED`.

Wrapper lease chỉ loại race giữa các process tuân theo wrapper trên cùng run-owned lease path; nó không khóa một AO client ngoài wrapper. Bước nhỏ nhất còn lại: khi có AO daemon thử nghiệm cô lập đã chạy và credential hợp lệ, tạo đúng một worker fixture disposable, chạy cùng delivery ID qua injected response-loss transport, rồi xóa session/resource run-owned.
