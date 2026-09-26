# Historical Locator Record Correction Note —
un-20260924-phase7bc-integration-continuation

- **Mục đích tài liệu**: Tệp hashes.sha256 trong thư mục này là một **historical locator record** ghi lại các đường dẫn worktree tạm thời từ đợt kiểm thử tích hợp Phase 7B/C lịch sử, không còn là manifest có thể đọc trực tiếp từ filesystem hiện tại (do các worktree disposable phục vụ test đã được dọn dẹp theo vòng đời).
- **Hiệu chỉnh biên tập (Redaction)**: Bốn locator trỏ đến đường dẫn worktree tạm thời cục bộ chứa tên người dùng Windows trước đây đã được biên tập và thay thế thành C:/Users/[REDACTED_USER]/... nhằm loại bỏ hoàn toàn tên người dùng hệ điều hành cục bộ khỏi current tree evidence.
- **Tính toàn vẹn**: Các giá trị SHA-256 của nội dung file được giữ nguyên vẹn; bản ghi này được lưu trữ phục vụ truy vết lịch sử (provenance audit).
