# Checkpoint profile worker ứng viên trước Phase 7

Ảnh IDE chỉ cho thấy nhãn “Gemini 3.8 Flash High” và mức High được chọn; không phải bằng chứng về payload hay mức thinking upstream. Checkpoint dùng một AO worker thật trên fixture độc lập, không dùng Product.

## Kết quả

- Yêu cầu và AO readback: `kind=worker`, `harness=codex`, `model=gemini-3.8-flash-high`, `reasoningEffort=high`; session `ao-phase5-repo-15`. AO project default vẫn `gemini-3.8-flash-high/low`.
- Gateway đang chạy quảng bá đúng ID `gemini-3.8-flash-high`; ID `gemini-3.8-flash` không có trong active catalog. Registry source khai báo model thuộc Antigravity và hỗ trợ `low/medium/high`. Đây là ID literal của Antigravity, không phải hậu tố thinking dạng ngoặc của CLIProxyAPI; không kết luận tương đương `gemini-3.8-flash + high`.
- Source AO tại `backend/internal/service/chat/controller.go` và `backend/internal/adapters/chatdriver/codexappserver/conversation.go` truyền session settings vào `turn/start` dưới `model` và `effort`. CLIProxyAPI tại `internal/thinking/apply.go` đọc `reasoning.effort` của OpenAI Responses; `internal/thinking/provider/antigravity/apply.go` ghi `request.generationConfig.thinkingConfig.thinkingLevel`; `internal/runtime/executor/antigravity_executor_execute.go` áp dụng thinking trước `payload.override`. Override hiện có nhắm `systemInstruction.parts.0.text`. Đây là **source mapping**, không phải payload quan sát trực tiếp.
- Payload `turn/start` và request Antigravity cuối cùng không được thu thập an toàn; `request.generationConfig.thinkingConfig.thinkingLevel=high` là **NOT OBSERVED**. Không bật full request log, không sửa component, không lưu secret; không suy mức thinking thực tế từ tên model, AO readback hoặc kết quả upstream.
- Một lượt worker hoàn tất, 11 command activities thành công, 0 command thất bại; đọc file, sửa `calc.py`, chạy Python assertions. Kiểm tra độc lập `python -B` exit 0. Worktree chỉ đổi `calc.py` (4 dòng thêm); `git diff --check` exit 2 vì một dòng trống mới tại EOF. Không sửa hậu nghiệm để tránh làm sai lệch bằng chứng worker.

## Lỗi, giới hạn, quyết định

- Smoke gateway ban đầu báo `Invalid API key` vì shell không có `CLIPROXY_KEY`; retry **catalog GET** bằng khóa runtime đã có, chỉ giữ trong bộ nhớ, thành công. Không đổi credential/provider, không OAuth, không thêm alias. Không retry turn hoặc đổi model/effort. Số retry nội bộ của gateway: **NOT OBSERVED**.
- `codex_apps` MCP initialization báo OAuth `token_revoked`; không chạm OAuth. Lượt worker chính vẫn hoàn tất. Không coi lỗi này là chứng cứ về model.
- Không nâng claim concurrency và không sửa evidence Gates 1–10. Ứng viên đạt tool round-trip, nhưng **HOLD_NOT_VERIFIED_HIGH** vì thiếu quan sát an toàn payload `thinkingLevel=high`; chưa thay baseline mặc định hay dùng cho P8.
- Chỉ ghi evidence metadata, không raw auth header, token, capability, request body hoặc log chưa redaction; hash trong `hashes.sha256`. Không commit/push, không đổi trust store, không thao tác Product.

## Phụ thuộc riêng của P8

Phải chốt contract nguồn và evidence cho trạng thái `SUCCEEDED` trước terminal RED. Checkpoint này không mở P8, không mở rộng backend hoặc launcher.
