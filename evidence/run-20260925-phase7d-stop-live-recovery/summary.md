# Phase 7D Stop audit and live delivery recovery

Status: `DELIVERY_RECOVERY_LIVE_PASS__STOP_SIMULATED_LIMITED`. Worktree `phase7d-recovery-prototype`; Product và AO/CLIProxyAPI upstream/shared daemon/session không bị sửa. Metadata durable đã phục hồi và nguồn từng fact nằm tại [durable-correction.md](durable-correction.md).

## Stop audit

- Cancel persist local intent trước I/O. Sau RecoverOnly/Observe, chỉ observed `interrupted/cancelled` trở thành `CANCELLED/HUMAN_STOP`.
- `completed` + receipt đúng task/artifact trở thành `COMPLETED`; completed thiếu/sai receipt giữ `CANCELLED_LOCAL_AO_STOP_UNCONFIRMED/COMPLETED_OUTCOME_UNVERIFIED`.
- `failed` trở thành `BLOCKED/DELIVERY_FAILED`. Các nhánh đều terminal qua restart, ordinary resend=0.
- AO `/conversation/interrupt` là session-wide; precheck active turn có race. Prototype yêu cầu `SessionExclusive=true` và run owner, nhưng không claim exact-turn Stop nếu có client ngoài wrapper.
- API delta tối thiểu nếu cần guarantee: turn-scoped interrupt nhận `{sessionId, turnId, expectedControllerGeneration}`, kiểm active turn atomically dưới controller lock, trả 409 khi fence không khớp.

## LIVE delivery recovery

- Run-owned AO daemon/data dir và gateway process riêng; project `p7d-live-fixture`, final session `p7d-live-fixture-2`, branch `ao/p7d-live-9b72a6c41e30/retry`, model `gemini-3.8-flash-high`, effort authoritative `high`; checkpoint run owner khớp `p7d-live-9b72a6c41e30`.
- Task chỉ đọc `go.mod`; adapter nhận AO HTTP acceptance rồi giả lập mất response. Wrapper restart dùng cùng clientMessageId, POST kế tiếp recover-only, khớp cùng turn và receipt taskId/artifact SHA-256; ordinary resend=0.
- AO/provider interaction là `LIVE`; transport response-loss là `SIMULATED`. Final ownership-correct test exit 0 trong 29.68s.
- Một preflight live trước đó dùng Codex/ChatGPT default và nhận upstream HTTP 400 model unsupported; không được tính PASS. Run đạt dùng isolated CODEX_HOME trỏ loopback CLIProxyAPI và credential runtime hiện có, không ghi secret vào evidence.

## Cleanup và giới hạn

- Session live đã kill; isolated AO daemon và gateway đã dừng; ports 3001/8317 không còn listener; run-owned chat-host descendants đã dừng; Git worktree registration không còn.
- Bản sao auth runtime đã xóa. Hai thư mục temp run-owned còn tồn tại vì policy tool từ chối recursive delete; không còn process/listener sở hữu. Stop LIVE không chạy và actual provider Stop vẫn `NOT OBSERVED`.
