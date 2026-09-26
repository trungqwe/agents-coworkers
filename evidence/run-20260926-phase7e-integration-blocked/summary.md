# Phase 7E — integration bị chặn bởi test portability

Trạng thái: `BLOCKED_AUTHORITY_TEST_PORTABILITY_DISPATCHER_TEST`; không có integration ACCEPT, executable self-host LIVE, cleanup PASS hoặc Phase 7E COMPLETE. Run `p7e-selfhost-corrected-20260926a`, baseline `4dfbf9905ff12c363307e5974fab719a0911b114`. Nguồn: AO conversation API tại `127.0.0.1:3005`, Git worktree AO run-owned và test output đã biên tập; không lưu raw conversation/payload/credential.

## Profile và task graph

- Gateway isolated `127.0.0.1:8321`, config SHA-256 `da5abd67227b050b267baa189cb348b5e08384a15d60365dbd151c300b2af0f9`; AO isolated `127.0.0.1:3005`, project `p7e-corrected`.
- Orchestrator `p7e-corrected-1`: `orchestrator/codex/gemini-3.7-flash-high`, AO settings `reasoningEffort=high`, `approvalMode=accept-edits`.
- Worker A `p7e-corrected-3`: runner/backoff, commit `f8006b64f44b5fea5d93027150d73af62f290572`, ACCEPT. Worker B `p7e-corrected-4`: status/CLI, initial `15ac112663876cd3c02e1ec00d2bbc7cee88b271`, rework `f39a76df9ced79b2411a7df242ccb26046db1346` → `a01c0519aa44137d5af8c5f46888f30e4e113a5a`, ACCEPT sau process-level exit-code matrix. A/B conversation settings không xuất `reasoningEffort`; không suy upstream thinking level từ tên model.
- Worker C `p7e-corrected-5`: `worker/codex/gemini-3.7-flash-high/high`, `accept-edits`, readback trước delivery. Delivery `p7e-integration-task-001`, AO outcome `sent`, turn `a8efe904-5ce7-4049-a95b-5f2ca7e47a6a`. C cherry-pick sạch A rồi ba commit B; combined HEAD thật `cfa5890f1a585f97743475c7d04014b031caa458` (detached), parent chain `b5bc9ef4` → `51521e20` → `56374e9e` → baseline. AO final synopsis ghi một SHA `cfa5890f055...` không khớp Git; giá trị đó không dùng làm evidence.

## Oracle và can thiệp

- Worker B: `go test -v -count=1 ./internal/recovery -run TestStatus` exit 0; `go test -v -count=1 ./cmd/recovery` exit 0, sáu top-level process tests PASS sau khi test timeout ban đầu FAIL vì fixture đi tới `ARTIFACT_READ_FAILED` và B sửa trong cùng worktree.
- C: `git cherry-pick <A> <B1> <B2> <B3>` exit 0, worktree sạch. `go test -v -count=1 ./...` exit 1 tại `internal/recovery/dispatcher_test.go:624`: artifact SHA đúng `0f96d491be3e33b707ac0e91e14a2355127a1a76b92f44cdf09d7168bbff9de5`, Git HEAD đúng combined HEAD, nhưng `Branch` rỗng vì detached HEAD; test hardcode `phase7d-recovery-prototype`. `go test -race`, `go vet`, build và executable self-host: NOT RUN trong C sau failure. Orchestrator focused `cmd/recovery` và `internal/recovery` chọn lọc exit 0, không thay thế full-suite PASS.
- Coordinator đã từ chối lệnh C định sửa `dispatcher_test.go` thành oracle yếu hơn; AO đánh dấu turn C `interrupted`. Coordinator cũng từ chối lệnh orchestrator `git switch -C phase7d-recovery-prototype` vì branch đó đang checkout ở worktree 7D, và từ chối prompt định replay cherry-pick. Orchestrator tự đọc AO/Git/test rồi chốt `BLOCKED_AUTHORITY_TEST_PORTABILITY_DISPATCHER_TEST`. Đây là can thiệp thực, không phải proof điều phối hoàn toàn tự động.
- Approval provenance: qua AO resolve `decisionId=accept` allow-once cho các command đúng scope; `cancel` cho các command nói trên. Không đổi global permission, pool/default hoặc dùng allow-all. AO lưu request/turn IDs; bản evidence này chỉ trích ID cần audit và không chứa raw command output dài.

## Delta cần duyệt và bảo toàn

Test 7D ở `internal/recovery/dispatcher_test.go:616-625` ràng buộc một branch cố định của worktree khác. Delta tối thiểu: cho C sửa riêng test này để tạo Git fixture tạm/branch run-owned hoặc đối chiếu branch hiện tại một cách độc lập mà vẫn xác minh SHA, HEAD và branch không rỗng. Sau đó C gắn combined HEAD vào branch AO của chính C, chạy lại full suite/race/vet/build; orchestrator review độc lập trước khi self-host executable. Chưa cấp authority delta trong run này; không sửa accepted test để ép PASS.

Nguồn code sáu file đã cherry-pick và `dispatcher_test.go` có SHA-256 trong `hashes.sha256`. Combined commit được ghim ở ref cục bộ `evidence/p7e-integration-blocked-20260926` tại đúng `cfa5890f1a585f97743475c7d04014b031caa458`; không merge/push. Main worktree `phase7d-recovery-prototype` tại baseline giữ sạch trước khi cập nhật tài liệu/evidence; Product và AO/CLIProxyAPI upstream không sửa. Gateway credential selection/cooldown cho các turn workload: NOT OBSERVED trong evidence này, không suy quota hoặc failover.

Cleanup: AO `DELETE /projects/p7e-corrected` thành công, năm session terminal và AO worktrees không còn trong Git worktree list. PID AO/gateway được đối chiếu với run metadata/command line rồi dừng; ports `3005/8321` trống. Tool policy từ chối trước thực thi cả lệnh xóa recursive temp run root và lệnh xóa riêng bản sao credential/config cô lập; `CLEANUP_BLOCKED_POLICY`, không tuyên bố đã xóa. Không thử cơ chế khác để vượt policy. Run root còn tồn tại, không có listener/process run-owned đã biết; cần cleanup thủ công theo owner hoặc authority/tool policy phù hợp.
