# Historical Locator Record Correction Note — `run-20260924-phase7bc-integration-continuation`

- **Mục đích tài liệu**: Các locator đường dẫn worktree disposable và digest tương ứng từ đợt kiểm thử tích hợp Phase 7B/C lịch sử đã được tách riêng vào `historical-locator-hash-record.txt`, không còn nằm trong active filesystem manifest (do các worktree test đã được dọn dẹp theo vòng đời).
- **Hiệu chỉnh biên tập (Redaction)**: Bốn locator trỏ đến đường dẫn worktree tạm thời cục bộ chứa tên người dùng Windows trước đây đã được biên tập và thay thế thành `C:/Users/[REDACTED_USER]/...` nhằm loại bỏ hoàn toàn tên người dùng hệ điều hành cục bộ khỏi current tree evidence.
- **Tính toàn vẹn**: Các giá trị SHA-256 của nội dung file được giữ nguyên vẹn; bản ghi này được lưu trữ phục vụ truy vết lịch sử (provenance audit).
- **Active Manifest**: Tệp `hashes.sha256` của run chỉ xác thực các tệp vật lý cố định trong chính thư mục run (`summary.md`, `correction-note.md`, `historical-locator-hash-record.txt`), loại bỏ authoritative mutable file (`docs/04-ROADMAP.md`).
