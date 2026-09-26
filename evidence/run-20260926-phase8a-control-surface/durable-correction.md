# Ghi Chú Đính Chính Tính Bền Vững Byte-Stable (Gate 8A Evidence)

- **Nguyên nhân đính chính**: Tập tin `hashes.sha256` ban đầu được tính toán từ các tệp trong Windows working tree mang ký tự xuống dòng CRLF (`\r\n`). Khi commit vào Git repository (`77d6179a4c28447e215dfd0775e5caa158cd9d86`), Git chuẩn hóa dòng thành LF (`\n`), dẫn đến mã băm trong manifest không khớp với byte thực tế của các committed blobs.
- **Biện pháp xử lý**:
  1. Thiết lập chính sách byte-stable trong `.gitattributes` phạm vi hẹp cho đúng thư mục run này: `/evidence/run-20260926-phase8a-control-surface/** -text`.
  2. Khôi phục toàn bộ working-tree bytes từ các Git blob chuẩn (LF) đã commit của `77d6179a4c28447e215dfd0775e5caa158cd9d86`.
  3. Bổ sung ghi chú đính chính này (`durable-correction.md`) với chuẩn UTF-8 (không BOM) và LF.
  4. Tái tạo `hashes.sha256` bằng LF, bao phủ đủ 6 tệp evidence chuẩn hóa: `commands.jsonl`, `durable-correction.md`, `environment.json`, `summary.md`, `test-matrix.json`, `verification.txt`.
- **Phạm vi & Ranh giới kỹ thuật**:
  - Không thay đổi bất kỳ dòng mã nguồn nào trong `cmd/coworkers` hay `internal/**`.
  - Không thay đổi kết quả kiểm thử (25/25 subtests, 26 subprocesses PASS) hay kết luận nghiệm thu đóng Gate 8A ở cấp độ L4.
  - Slice 8B giữ nguyên trạng thái `NEXT / IMPLEMENTATION_NOT_AUTHORIZED`.
