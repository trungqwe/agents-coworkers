# Manifest Correction Note — `run-20260925-phase7d-stop-live-recovery`

- **Kết quả run giữ nguyên**: Kết quả thực thi và bằng chứng của đợt thử nghiệm `p7d-live-9b72a6c41e30` (Stop/Recovery) giữ nguyên trạng thái quan sát durable; việc tái cấu trúc manifest không làm thay đổi các sự kiện run đã ghi nhận.
- **Tiến triển của source/docs sau run**: Các tệp mã nguồn (`internal/recovery/dispatcher_test.go`) và tài liệu quản trị (`docs/02-ARCHITECTURE.md`, `docs/04-ROADMAP.md`, `docs/05-VERIFICATION.md`, `docs/06-OPERATIONS.md`) tiếp tục phát triển trong các phase tiếp theo (Phase 7D continuation và Phase 7E self-host). Do đó, digest SHA-256 tại thời điểm run 2026-09-25 phản ánh snapshot lịch sử, không đại diện cho trạng thái hiện hành của repository.
- **Bảo toàn digest lịch sử**: Toàn bộ digest tại thời điểm run được lưu trữ nguyên vẹn tại `historical-hash-record.txt` phục vụ mục đích audit truy vết provenance.
- **Phân định active manifest**: Không sử dụng digest lịch sử của source/docs để xác minh current filesystem. Tệp `hashes.sha256` của run này được chuẩn hóa để chỉ chứa các tệp vật lý cố định nằm trong chính thư mục bằng chứng của run.
