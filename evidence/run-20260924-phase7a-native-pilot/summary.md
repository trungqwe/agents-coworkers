# Phase 7A — native AO review pilot (24/09/2026)

## Phạm vi và input

- Pilot review-only; không sửa Product, không triển khai rework, không chạy test Product, không commit/push/merge.
- Product root `D:/AI Auto Video Creator`: baseline `4a7c8c921b7e05066505d51b168a02c3fde61317`, sạch lúc preflight và audit cuối. Worktree `ai-auto-video-creator-2` giữ dirty CP1/CP2A hiện hữu; CP2A chưa accepted, E3 chưa có verification hợp lệ, Temporal cleanup pending.
- Snapshot chọn lọc nằm tại `snapshot/`: 23 entry (gồm source untracked liên quan và tracked patch 18.414 bytes), 23/23 byte count + SHA-256 khớp nguồn lúc tạo và được Astra/ba worker xác minh. `snapshot/manifest.json` SHA-256 `bcc7e623532adcd9d4092dbda2e538f3c9862c55d0108dba58d7daefa594c160`. Snapshot read-only; không chép credential, runtime config, dependency hoặc raw log.
- AO daemon PID `42608`, binary `D:/TU_CODE/agent-orchestrator/backend/ao.exe` SHA-256 `dce49a699c848761a4f23d28a7e6f7218ab6530345062c99b6d356c63725b371`, báo version `dev`; source HEAD `1140dd62dc7bb588b987e2c44aa1ff4796fa732b`. Binary–source correspondence chưa xác lập.

## Capability và provenance

- Source: `service/session/delegation.go` + controller/DTO hỗ trợ native delegate có per-session model/effort; `cli/send.go` + chat controller chuyển message/turn; SQLite conversation/plan và SSE change log hỗ trợ quan sát. `send --recover-only`/`RetryTurn` có phạm vi hạn chế; chưa có bằng chứng scheduler wake-up workflow khi hết pool hay orchestrator idle.
- Binary/runtime: Astra `ao-phase5-repo-1` readback `orchestrator/codex/gpt-6-astra/low/chat`; API native tạo A=`ao-phase5-repo-16`, B=`...-17`, C=`...-18`, mỗi session readback `worker/codex/gemini-3.8-flash-high/high/chat`. Project defaults không đổi. Cả ba worktree fixture riêng giữ HEAD `60aa78498d2e5325c96545d73d006e1f10b812a4`, chỉ có `review-A/B/C.md` untracked tương ứng.
- Coordinator ngoài AO chỉ gửi bootstrap turn `bf9b3cf6-2547-4fd2-904c-c520dd69c28e` với manifest/scope/authority, sau đó quan sát; không relay findings, không dispatch worker hay tổng hợp thay Astra. Astra tự giao ba task qua `ao send`, tự nhận AO reply, đọc/kiểm hash artifact và lọc findings. Reply khi bootstrap turn còn chạy được AO queue thành turn mới; chưa thử wake-up từ trạng thái hoàn toàn idle/restart.

| Task | Scope | Conversation | Dispatch message / worker turn | Reply message trong Astra | Artifact SHA-256 |
|---|---|---|---|---|---|
| A | Contract/state/evidence | `4689e51d-f7e0-40bb-81e0-14a812361fa6` | `8614f28a-3e75-4d27-a800-e721969e641a` / `f3f36ed6-2b69-48a5-ae59-95185b551272` | `537ae72a-860e-4834-b7b4-04c0b395d145` | `469da4b5114248556f3bc330bd038ac53f20641b6c1c14c4e61b6f6c84ae4ff8` |
| B | Transaction/CAS/replay/outbox/checkpoint | `7d07b2fc-d252-45c8-877d-c6c35c9f1a4d` | `99cdd4c8-0981-4679-a6de-57adb62724a3` / `de0172f1-6a93-42d4-a788-16538c120df2` | `308bfee5-c33d-4bb9-a34f-9bce23fc9f4b` | `0cf0cc4e9a975fe97920ad18fb79e90b84f5dffe6ea7be987d08a991e93ed618` |
| C | Query/isolation/coverage | `5a05da0a-4682-4b8c-9d83-0abd457269a7` | `58da5a3c-fd76-4c4c-ae60-109567578d06` / `a7ccc551-5c98-40e9-810f-75c610a54643` | `f22c6d91-c696-4e62-9e89-26bd70803b54` | `712e784af1ff7be4b08395ad7760fef45648d68d88820bbe5a97795cd83c2581` |

- A/B/C bắt đầu 04:46:48–49 (UTC+7), overlap 7 phút 41 giây theo AO turn timestamps; đây không chứng minh CPU/provider execution song song. Không quan sát 429; retry budget 2, đã dùng 0. Không có approval thủ công; AO auto-review hiện hữu xử lý command, không đổi sandbox/approval/default.
- Astra steer A/B một lần về cách ghi Markdown sau lỗi shell quoting (activity `036ecc86-7736-4ef6-aa6e-dd7d9c575c57`, `ad138152-59b4-4d08-a9bb-350e4cdcf01a`); không cung cấp findings hoặc sửa hộ. Một status command treo được dừng và thay bằng timed GET; một URL polling 404 được sửa. Không resend task hay tạo worker thứ tư.

## Verdict của Astra sau mở artifact và đối chiếu source

- Giữ lỗi mapping nội bộ `PREPARED/STARTED` thành API `prepared/started` thay vì contract `accepted/running`: `src/controlplane/infrastructure/db/control_api_queries.py:48`; test snapshot hiện cố định vocabulary sai tại `tests/m2/test_p8_execution_state.py:148,183`. A/C trùng lỗi, Astra gộp thành một. Đây là source-level finding, không phải runtime UI proof.
- Giữ đề xuất rework P4 transition validator và binding/bảo toàn cả claim lẫn activation evidence: `src/controlplane/application/admin_execution.py`, `src/controlplane/infrastructure/db/admin_execution.py`, migration `0009_p8_execution_state.sql` + rollback, test driver/test P8 nếu cần. Nhận xét phải giữ phạm vi: primary execution row ghi đè claim reference, nhưng outbox vẫn giữ historical reference; không tuyên bố mất mọi evidence.
- B không có lỗi correctness mới đủ căn cứ. Các đề xuất về giả định kiểu dữ liệu, stream LATERAL và actor/failure mở rộng không thành rework bắt buộc; thiếu test collection/isolation/limit là coverage gap trong snapshot, không suy leak tenant hoặc toàn CI thiếu coverage. Không chạy lại E3/P8/backend suite.
- Astra verdict tại fixture orchestrator `phase7a-evidence/verdict.md`, SHA-256 `738c7c4f25a0275f6c92786afd0edc18764eefd8c81937e91e6e09ce2e994dc0`; ba artifact tại worktree 16/17/18 với hash ở bảng. Chỉ metadata/hash và kết luận đã lược dữ liệu được chép vào evidence này; không lưu raw AO conversation/request.

## Giới hạn và bước kế tiếp đề xuất

- Pilot chứng minh native dispatch/receive/review trong một lượt review giới hạn; **không** chứng minh parallel implementation, merge/integration, rework đã sửa, retry delivery uncertain, resume sau restart, toàn pool unavailable hoặc Phase 7 COMPLETE. Không patch AO/CLIProxyAPI trong pilot; structured dependency/ownership và recovery workflow vẫn là gap cần oracle riêng.
- CP2A dirty/unaccepted và rework P4/evidence/migration cùng phụ thuộc nhau, không phù hợp ép thành ba coding lane độc lập. Một bước kế tiếp có quyền riêng: dùng fixture AO Git baseline `60aa784…` để chứng minh code + integration thật trên slice calculator nhỏ, với worker I sở hữu `pilot_arithmetic.py` (add/subtract), worker II sở hữu `pilot_division.py` (divide + zero contract), integration worker sở hữu `pilot_calculator.py` và `tests/test_phase7_integration.py`; contract/interface chốt trước dispatch, ba file ownership không giao chéo. Integration owner tạo worktree riêng, nhận hai local commits theo thứ tự xác minh, chạy `unittest` trên combined head và lưu hash/test receipt. Cần duyệt quyền tạo worktree fixture, ghi đúng bốn file, chạy test và local commit/cherry-pick; không push/merge main, không Product/dependency mới. Đây là slice fixture vì không có ba phần Product CP2A độc lập an toàn; chưa thực hiện.
- Recovery checkpoint sau slice cần oracle interrupt/resume/queued reply/delivery ID và cooldown/backoff riêng; không mặc định cần framework mới khi native seam chưa được thử dưới lỗi.
