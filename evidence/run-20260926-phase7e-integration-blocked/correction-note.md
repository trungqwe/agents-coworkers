# Correction Note: Evidence Checkpoint run-20260926-phase7e-integration-blocked

- **Trạng thái run lịch sử**: Giữ nguyên kết luận BLOCKED_AUTHORITY_TEST_PORTABILITY_DISPATCHER_TEST của run p7e-selfhost-corrected-20260926a.
- **Nguyên nhân chênh lệch digest**: Trong bản hashes.sha256 ban đầu, các đường dẫn có tiền tố ảo git-ref-evidence/p7e-integration-blocked-20260926/ không tồn tại trên hệ thống tệp vật lý, và giá trị digest SHA-256 được tính toán từ các tệp mã nguồn đã materialize với định dạng kết thúc dòng CRLF trên Windows thay vì Git blob bytes canonical (LF).
- **Phân loại**: Đây là sự khác biệt về line ending do cơ chế checkout/materialize trên môi trường Windows (CRLF), hoàn toàn KHÔNG phải biến dạng mã nguồn (source mutation).
- **Hiệu chỉnh**:
  1. Toàn bộ thông tin đối tượng Git canonical tại commit cfa5890f1a585f97743475c7d04014b031caa458 (ref evidence/p7e-integration-blocked-20260926) được ghi nhận chuẩn xác vào git-object-manifest.jsonl, với blob_oid và sha256 tính trực tiếp từ byte nguyên bản của blob (git cat-file blob).
  2. Tập hashes.sha256 được cập nhật lại chỉ chứa các tệp chứng cứ vật lý cố định thuộc run: evidence/run-20260926-phase7e-integration-blocked/summary.md, evidence/run-20260926-phase7e-integration-blocked/git-object-manifest.jsonl, evidence/run-20260926-phase7e-integration-blocked/correction-note.md, và evidence/run-20260926-phase7e-integration-blocked/roadmap-diff.patch (thay cho việc hash trực tiếp tệp authoritative đang tiếp tục thay đổi docs/04-ROADMAP.md).
