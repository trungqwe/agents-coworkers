# Phase 8 — credential pool onboarding và routing

- Run: `run-20260923-phase8-pool-onboarding`; ngày 2026-09-23, timezone Asia/Saigon.
- Trạng thái: `IN_PROGRESS_POOL_VERIFICATION`; user dừng onboarding ở mức hiện tại. Phase 8 chưa `COMPLETE`; concurrency proven vẫn 3.
- Bốn AO 429 cũ giữ `UNKNOWN` trong run chẩn đoán trước. Run này không thay evidence Gates 1–10 hoặc accepted evidence Product.

## Preflight

- Gateway listener `127.0.0.1:8317`, binary SHA-256 `e9da39b2491856be2d3711e20be89690f3465e2a5b7a469ba1a29aff7ab342d2`, tự báo `Version: dev, Commit: none, BuiltAt: unknown`. Source checkout `CLIProxyAPI@2430354330af80b645f9ffb1a51e1e7c72c4cc8e`; không chứng minh được binary được build từ chính source này. Không build/patch/thay binary.
- Cấu hình runtime đã lược bí mật: `request-retry=3`, `max-retry-credentials=0`, `max-retry-interval=15s`, `routing.strategy=round-robin`, `session-affinity=true`, `session-affinity-ttl=4h`, `session-affinity-subagents=false`; model/effort, `payload.override` và project defaults không đổi.
- Source: `scheduler.go` loại credential unavailable/quota/cooldown; `conductor_execution.go` thử lại trong budget; `conductor_selection.go` xử lý retry/cooldown; Antigravity `decideAntigravity429` đọc ErrorInfo reason và RetryDelay để phân biệt full quota, short rate-limit và soft retry. Trace header `X-CPA-TRACE-ID` mang auth index ổn định. Đây là SOURCE, không xác nhận binary-source correspondence.

## Onboarding và kiểm chứng

- Inventory theo từng nấc: Codex giữ 1; Gemini `1→2→3→4→5→6→6 (login trùng)→7`. Bảy file Gemini có bảy identity hash khác nhau, không disabled. Lượt cuối hoàn tất ngay khi user yêu cầu dừng; giữ nguyên credential đã tạo. Không có login Codex mới. Không lưu email, filename auth, token hoặc raw response/request.
- LIVE_ROUTING: 14 request fixture tuần tự đến `gemini-3.8-flash-high` với `reasoning.effort=high`, 14 HTTP 200, 0 live 429; 5 auth index khác nhau được quan sát và mỗi ID đó trả 200. Cùng session A giữ G1 ở hai request; các session khác cho thấy G1–G5 được chọn. Chi tiết ẩn danh trong `observations.jsonl`. Usage quan sát: 308 input tokens, 840 output tokens tổng; không suy quota remaining. Latency 3546–14235 ms.
- Đếm theo `gemini-3.8-flash-high`: registered=7, loaded/eligible/observed-selected/successful **ít nhất 5**; hai credential chưa thấy được chọn giữ `UNKNOWN` ở các tầng sau registered. Với Codex: registered=1; loaded/eligible/selected/successful cho model Codex `NOT_OBSERVED` trong run này. Catalog target có mặt và trả HTTP 200, không phải proof per-credential.
- LIVE_FAILOVER: `NOT_OBSERVED`; không có 429/cooldown tự nhiên trong request mới. Không đốt quota, không reset quota/cooldown. Gateway internal attempt count và per-account quota remaining: `NOT_OBSERVED`.
- SOURCE/UNIT `SIMULATED`: selector round-robin, affinity, unavailable failover, all-cooldown error, credential retry budget, request-scoped 429 stop, Antigravity 429 classification và trace header tests đều PASS; không nâng thành live failover.

## AO và CP1

- Gateway pool đáp ứng fixture. AO daemon khởi động với cùng `~/.ao/data`; một lần `resume-agent` trả 500 vì Codex app-server không tìm thấy rollout của thread cũ. Rollout đúng thread tồn tại ở AO-owned fixture `CODEX_HOME`; daemon relaunch với môi trường đó tự reconcile session `ai-auto-video-creator-2` sang `idle`, không tạo session/worktree mới. Worker đã nhận followup CP1 và chuyển `working`.
- `ao start` trước đó thử tải desktop nhưng kết thúc `installed app not found`; daemon thực tế chạy bằng binary sẵn có với lệnh `ao daemon`. Không có patch AO/CLIProxyAPI. AO_TOOL_LOOP được quan sát riêng: session cũ dùng `gemini-3.8-flash-high/high`, turn tiếp nối có command activity, sửa fixture trong worktree cũ và kết thúc tại audit CP1.
- CP1 continuation: IPC isolated 6/6 PASS; PostgreSQL, scaffold build, Chromium và Job Object prerequisite PASS. Windows hiển thị hộp thoại Security Warning khi `certutil -user -addstore Root`; worker dừng fail-closed, không cài certificate. Vì HTTPS trust/bootstrap chưa PASS, ba browser identities được collect nhưng 0 executed, 0 RED observed. Backend RED cũ vẫn là `assert 'revision' in operation`, không rerun do backend source không đổi. Evidence Product: `run-m2-p8-cp1-continuation-20260923164500`; Product root sạch. Container PostgreSQL run-owned `cp1-p8-pg-run-58383` được coordinator dừng sau audit, không dừng Docker Desktop. CP2/P9/M3/Phân hệ A khóa.

## Giới hạn

- Binary không nhúng commit/build provenance, nên source mapping là đối chiếu source chứ không là chứng cứ đường chạy binary.
- Không chứng minh phân phối đều, quota còn lại, pool bảo đảm hết 429, hoặc live failover. `X-CPA-TRACE-ID` chứng minh selection của từng request đã quan sát, không chứng minh mọi credential đã dùng.
