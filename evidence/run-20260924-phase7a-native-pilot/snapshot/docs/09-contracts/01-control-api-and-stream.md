# 01. Control API và luồng cập nhật

> Trạng thái: **Bản thiết kế trước triển khai**  
> Chủ sở hữu biên: phân hệ H phối hợp với G và J.  
> Đối tượng sử dụng: giao diện quản trị tiếng Việt và local agent đã xác thực.

## 1. Phạm vi

Control API cung cấp:

- query cho tin, media, hook, batch, job, stage, log, cấu hình và capability;
- command bắt đầu lô, chạy từng module để debug, thử lại và tạo biến thể mới;
- sửa metadata được user cho phép;
- quản lý nguồn và hook;
- cấu hình provider/API theo biên secret;
- luồng SSE để theo dõi thay đổi gần thời gian thực.

Control API không cung cấp trong v1:

- sửa nội dung bài gốc;
- sửa trực tiếp quan hệ sự kiện;
- sửa script hoặc bước duyệt script;
- đăng video lên nền tảng;
- đưa secret trở lại UI sau khi đã lưu;
- dùng SSE để gửi command.

## 2. Quy tắc HTTP

### CT-API-001 — Quy tắc request

- Base path có version: `/v1`.
- Mutation yêu cầu `Idempotency-Key`.
- Metadata update yêu cầu `If-Match` hoặc `expected_revision` tương đương.
- Mọi request có authentication và được ràng buộc vào `workspace_id`.
- Response có `correlation_id`.
- `202 Accepted` chỉ xác nhận đã ghi nhận công việc bất đồng bộ.
- `409 Conflict` dùng cho revision/state/idempotency conflict; `422` dùng cho input hợp lệ về cú pháp nhưng sai quy tắc.
- Error dùng `ProblemDetail` từ hợp đồng chung.

## 3. Query surface

### CT-API-002 — Danh mục query

| Resource logic | Query tối thiểu | Bộ lọc chính |
|---|---|---|
| Sources | danh sách, chi tiết, lịch sử thay đổi | chủ đề, Tier, trạng thái, lần quét cuối |
| Articles | danh sách, chi tiết revision/provenance | chủ đề, thời gian, nguồn, trạng thái trùng |
| Events | danh sách, timeline, article links | chủ đề, trạng thái liên kết, có diễn biến mới |
| Media | danh sách, chi tiết, rendition, usage | loại, tag, nguồn, availability, safety |
| Hooks | visual/audio riêng biệt | loại, tag, trạng thái bật, usage |
| Batches | danh sách, tiến độ, lý do dừng | session, trạng thái, ngày |
| Jobs | danh sách, chi tiết snapshot/stage/output | batch, stage, trạng thái, event/article |
| Stage runs | input/output refs, attempt, lỗi | job, module, trạng thái |
| Sessions | phiên tắt/bật app và output folder | ngày, desktop, trạng thái |
| Logs | bản tóm tắt và technical detail theo quyền | session, job, severity, time |
| Configuration | revision đang hiệu lực và lịch sử | scope, module, trạng thái |
| Capabilities | worker/provider/storage health | loại capability, availability |
| Operations | trạng thái command bất đồng bộ | actor, resource, trạng thái |

Query article không tự động tải thêm nguồn; query artifact không tự động materialize lên desktop.

## 4. Command surface

### CT-API-003 — Quản lý nguồn

| Command | Kết quả | Ghi chú |
|---|---|---|
| `RegisterSource` | source ref + receipt | URL phải qua kiểm tra an toàn |
| `UpdateSourceMetadata` | revision mới | Chỉ Tier, chủ đề/tag, ghi chú, bật/tắt và metadata quản trị |
| `RemoveSource` | tombstone revision | Ngừng thu thập; không tự thêm lại |
| `RestoreSource` | source revision phục hồi | Khôi phục trạng thái vận hành trước khi xóa nếu còn hợp lệ |
| `TestSource` | operation ref | Không thay cho lượt quét định kỳ |

### CT-API-004 — Quản lý hook và media

| Command | Kết quả | Ghi chú |
|---|---|---|
| `ImportVisualHook` | hook ref + artifact operation | User cung cấp video hook đã làm mờ |
| `ImportAudioHook` | hook ref + artifact operation | Kho audio hook độc lập |
| `UpdateHookMetadata` | revision mới | Tag, mô tả, trạng thái bật/tắt, ghi chú |
| `UpdateMediaMetadata` | revision mới | Chỉ metadata quản trị cho phép |

Không có command sửa byte của bản gốc tại chỗ. Bản xử lý là artifact version mới.

### CT-API-005 — Sản xuất và debug

| Command | Kết quả | Ngữ nghĩa |
|---|---|---|
| `StartProductionBatch` | batch ref + operation ref | User đặt số video hoàn thành mục tiêu và cấu hình lô |
| `RunDebugStage` | stage run ref | Chạy một module; không tự đi tiếp stage sau |
| `RetryStage` | cùng job/snapshot, attempt mới | Giữ nguyên input logic; không tính là biến thể mới |
| `CreateNewVariant` | job mới + intent mới | Phải đạt quy tắc khác biệt đã chốt |
| `RegenerateFromStage` | operation ref | Chỉ hợp lệ nếu dependency graph cho phép và phải tạo revision đầu ra mới |

`RegenerateFromStage` không cho phép sửa lịch sử. Các output cũ vẫn được giữ để audit theo retention policy.

> **Hiệu chỉnh theo Amendment P8-START-001 (User approved cho Phase 7 / M2-P8):**
> `operation_id` của receipt trả về từ `StartProductionBatch` chỉ theo dõi chu trình thực thi của chính command `StartProductionBatch`:
> - `PREPARED` (hiển thị `accepted`): Lệnh đã được ghi nhận trong Unit of Work, batch ở trạng thái `CREATED` (revision 1), operation execution state khởi tạo ở revision 0, receipt trả 202 kèm operation ref.
> - `STARTED` (hiển thị `running`): Executor tin cậy đã nhận và bắt đầu xử lý command, chuyển trạng thái sang `STARTED` (revision 1) cùng bằng chứng khởi động (start evidence kind/ref).
> - `SUCCEEDED` (hiển thị `succeeded`): Trong cùng Unit of Work, command đã chuyển bền vững batch từ `CREATED` sang `RUNNING` (revision 2) và operation chuyển sang `SUCCEEDED` (revision 2) với bằng chứng kích hoạt (activation evidence kind/ref) được ràng buộc.
> - Trạng thái `SUCCEEDED` của operation này **KHÔNG** có nghĩa là batch hoàn thành (`COMPLETED_TARGET`), job hoàn thành hay toàn bộ quy trình video production kết thúc; và **KHÔNG** cấp quyền tạo `CompletionLedger`. Batch completion và ledger tiếp tục tuân thủ tuyệt đối CT-ORC-008/010.
> - Historical receipts cũ (trước P8): do không có bản ghi trong bảng execution state và không được backfill suy diễn, trả `status="unknown_legacy"`, `revision=null`.


### CT-API-006 — Cấu hình và tài khoản ngoài

| Command | Kết quả | Quy tắc |
|---|---|---|
| `PublishConfigurationRevision` | config revision ref | Áp dụng cho job chưa bắt đầu tạo script |
| `UpdateSecret` | secret metadata, không trả secret | Giá trị chỉ đi vào secret boundary |
| `TestExternalAccount` | operation ref + kết quả đã redacted | Không làm lộ credential |
| `EnableExternalAccount` | account revision | Có kiểm tra policy và quota |
| `DisableExternalAccount` | account revision | Việc không cần account vẫn tiếp tục |

## 5. Operation resource

### CT-API-007 — `OperationView`

| Trường | Ý nghĩa |
|---|---|
| `operation_id` | ID theo dõi từ receipt |
| `command_ref` | Command khởi tạo |
| `status` | `accepted`, `running`, `waiting`, `outcome_unknown`, `succeeded`, `failed`, `unknown_legacy` (cho historical receipts cũ) |
| `progress` | Stage, số lượng hoặc phần trăm nếu đo được |
| `resource_refs` | Batch/job/stage/artifact liên quan |
| `started_at`, `updated_at`, `finished_at` | Mốc thời gian |
| `wait_reason` | Lý do chờ có cấu trúc |
| `problem` | Lỗi chuẩn khi có |

Progress chỉ để quan sát; không phải bằng chứng commit hoàn thành.

## 6. Luồng SSE

### CT-API-008 — `OperationStream`

Endpoint logic: `GET /v1/operations/stream?cursor=...`

Mỗi item có:

- `stream_event_id` duy nhất;
- `cursor` tăng theo stream projection;
- `resource_type`, `resource_id`, `resource_revision`;
- `event_kind`;
- `occurred_at`, `recorded_at`;
- `summary` an toàn để hiển thị;
- `correlation_id`;
- `resync_required` khi cursor đã hết retention.

Quy tắc:

1. Client có thể reconnect bằng cursor cuối đã nhận.
2. Event SSE có thể lặp; UI deduplicate theo `stream_event_id`.
3. Nếu mất khoảng dữ liệu hoặc cursor hết hạn, UI query lại resource snapshot.
4. SSE không đảm bảo thay thế audit log bền vững.
5. Tooltip lỗi lấy `technical_detail_ref` qua query có quyền, không nhúng stack trace trong stream.
6. Luồng SSE đóng vai trò tín hiệu invalidation để UI làm mới snapshot tài nguyên từ authoritative source; SSE không truyền thay thế snapshot trạng thái đầy đủ.
7. Trình duyệt `EventSource.onerror` chuẩn không cung cấp HTTP status code hoặc response body; việc kiểm chứng các mã lỗi `CURSOR_*` được thực hiện qua HTTP / APIRequestContext.

## 7. Cạnh tranh và áp dụng cấu hình

### CT-API-009 — Sửa khi lô đang chạy

- Metadata/config revision mới áp dụng cho video chưa bắt đầu stage tạo script.
- Job đã commit `ProductionSnapshot` tiếp tục dùng revision cũ.
- UI phải hiển thị revision đang hiệu lực của từng job.
- Sửa đồng thời với revision cũ trả `REVISION_CONFLICT`.
- User phải tải lại dữ liệu trước khi gửi lại; server không tự merge.

## 8. Bảo mật biên UI

### CT-API-010 — Yêu cầu bắt buộc

- Local UI dùng HTTPS, session an toàn, kiểm tra `Host`/`Origin` và chống CSRF theo kiến trúc đã duyệt.
- API không tin workspace, role hoặc file path do client tự khai báo mà không xác minh.
- Nội dung nguồn bên ngoài được escape khi hiển thị.
- Secret đã lưu chỉ hiển thị metadata như provider, account label, trạng thái và thời điểm cập nhật.
- Log download và thao tác nhạy cảm có audit actor.

## 9. Acceptance contract

1. Gửi lặp cùng command không tạo hai batch, job, source hoặc hook.
2. Hai tab sửa cùng metadata không âm thầm ghi đè nhau.
3. UI phân biệt rõ command đã nhận với công việc đã hoàn thành.
4. Mất SSE rồi kết nối lại không làm mất trạng thái cuối.
5. Không tồn tại endpoint sửa article body, event relation hoặc script trong v1.
6. Không response nào trả secret hoặc stack trace thô.

