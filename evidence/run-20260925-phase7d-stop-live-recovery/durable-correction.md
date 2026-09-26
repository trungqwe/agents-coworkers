# Durable evidence correction — `p7d-live-9b72a6c41e30`

Không chứa prompt/response thô, credential, token, email, tên tệp xác thực hoặc private runtime config.

| Fact | Giá trị đã lược bí mật | Nguồn durable |
|---|---|---|
| Run owner | `p7d-live-9b72a6c41e30` | test recorder + branch AO |
| Project / session / branch | `p7d-live-fixture` / `p7d-live-fixture-2` / `ao/p7d-live-9b72a6c41e30/retry` | AO SQLite `sessions` |
| Profile | `worker`, `codex`, `gemini-3.8-flash-high`, effort `high` | AO SQLite `sessions` + `conversations` |
| Artifact | `go.mod`; Git HEAD `0190e925cd552b936250c74c29cbc272bb477e49`; SHA-256 `0f96d491be3e33b707ac0e91e14a2355127a1a76b92f44cdf09d7168bbff9de5` | test recorder + receipt |
| Delivery / clientMessageId | `p7d-live-delivery-20260925t094702.217778300`; recovery dùng đúng cùng ID | AO SQLite `conversation_messages.client_message_id` + test recorder |
| Turn | `d95d5b4a-7de0-47fb-8d45-02c746a5b6bb` | AO SQLite `conversation_turns.id` |
| State sequence | `PENDING → DELIVERY_UNCERTAIN → DELIVERED → COMPLETED` | assertions của live test recorder; `completed` đối chiếu AO SQLite |
| Counts | ordinary send `1`; recoverOnly `1`; ordinary resend `0` | live test recorder |
| Receipt | taskId `P7D-LIVE-RECOVERY-01`, artifact SHA khớp, `accepted=true` | assistant message trong AO SQLite; chỉ lưu match booleans và message SHA-256 `cf8f3c39885637b90f7b8a1713d42d27eb2aff60dca40549967569c361831899` |

Thứ tự durable: AO nhận message/turn lúc `2026-09-25T09:47:02.2758556Z`; turn bắt đầu `09:47:02.3484252Z`; adapter giả lập mất response sau acceptance; wrapper restart rồi recover-only theo cùng clientMessageId; AO hoàn tất lúc `09:47:31.7068824Z`. Recovery nằm giữa acceptance và completion theo recorder; không có timestamp AO riêng vì recover-only chỉ đối chiếu delivery hiện hữu, không tạo turn thứ hai.

Claim: AO/provider delivery là `LIVE`; transport response-loss tại adapter boundary là `SIMULATED`; Stop/cancellation là `SIMULATED`; provider outage, daemon restart, unattended recovery và Stop LIVE là `NOT OBSERVED`.

Cleanup audit: hai thư mục `C:\Users\[REDACTED_USER]\AppData\Local\Temp\p7d-live-5da2481c3b78` và `C:\Users\[REDACTED_USER]\AppData\Local\Temp\p7d-live-9b72a6c41e30` resolve đúng dưới temp của user, owner `[REDACTED_USER]`, không có listener 3001/8317 hoặc process run-owned còn sống. Quét tên tệp và pattern secret không phát hiện credential; bản sao xác thực tạm không tồn tại. PowerShell `Remove-Item -LiteralPath ... -Recurse -Force` bị tool policy từ chối trước khi thực thi: `CLEANUP_BLOCKED_POLICY`; hai thư mục vẫn tồn tại.
