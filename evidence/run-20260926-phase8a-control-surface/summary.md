# Báo Cáo Kiểm Chứng Đóng Gate 8A: Phase 8A Control Surface

## 1. Thông Tin Chung & Quyết Định Đóng Gate

- **Run ID**: `run-20260926-phase8a-control-surface`
- **Mục Tiêu**: Đóng Gate 8A ở cấp độ L4 (Local Process Smoke), xác nhận hoàn thành Slice 8A.
- **Trạng Thái Gate 8A**: `VERIFIED — L4`
- **Trạng Thái Slice 8A**: `COMPLETE`
- **Trạng Thái Phase 8**: `ACTIVE_IMPLEMENTATION` (Phase 8 chưa COMPLETE; không tuyên bố production-ready).
- **Trạng Thái Slice 8B**: `NEXT / IMPLEMENTATION_NOT_AUTHORIZED` (không mở triển khai Slice 8B).
- **Baseline Commit**: `3d99486d9bfc53513517259f607cb978ed7f0925`
- **Implementation Commit**: `8c8948a41a11065cbfe9f7b2d2549c7f43b6e65b`
- **Branch**: `phase8a-control-surface`
- **Worktree**: `D:\TU_CODE\agents-coworkers-phase8a-control`
- **Môi Trường**: `go version go1.26.0 windows/amd64`, `git version 2.55.0.windows.5` trên Windows.

## 2. Danh Sách Tệp Triển Khai Chính Xác (17 Source/Test Files + 5 Contract Docs)

Triển khai của Slice 8A gồm đúng 17 tệp source/test và 5 tệp hợp đồng tài liệu được đồng bộ hoá:

### 17 Tệp Source & Test Slice 8A
1. `cmd/coworkers/main.go`
2. `cmd/coworkers/main_test.go`
3. `internal/recovery/lease.go`
4. `internal/recovery/lease_test.go`
5. `internal/workforce/control/attach.go`
6. `internal/workforce/control/attach_test.go`
7. `internal/workforce/control/discovery.go`
8. `internal/workforce/control/discovery_test.go`
9. `internal/workforce/control/doctor.go`
10. `internal/workforce/control/doctor_test.go`
11. `internal/workforce/control/manifest.go`
12. `internal/workforce/control/manifest_test.go`
13. `internal/workforce/control/run_spec.go`
14. `internal/workforce/control/run_spec_test.go`
15. `internal/workforce/control/status.go`
16. `internal/workforce/control/status_test.go`
17. `internal/workforce/control/types.go`

### 5 Tệp Hợp Đồng Tài Liệu Đồng Bộ
1. `docs/02-ARCHITECTURE.md`
2. `docs/03-DECISIONS.md`
3. `docs/04-ROADMAP.md`
4. `docs/05-VERIFICATION.md`
5. `docs/06-OPERATIONS.md`

`go.mod` và `go.sum` hoàn toàn nguyên vẹn, không bổ sung bất kỳ third-party dependency nào; toàn bộ sử dụng Go standard library (`encoding/json`, `flag`, `os`, `path/filepath`, `net/http`, `crypto/sha256`).

## 3. Hợp Đồng Kiểm Tra Workspace Dirty Đã Được Chuẩn Hoá

- **Product Root**: Đánh giá bằng lệnh raw `git status --porcelain=v1 -z` không có ngoại lệ. Bất kỳ tệp untracked/modified nào kể cả dưới `.agents-coworkers/**` đều làm Product root dirty và khiến `doctor`/`attach` fail-closed với exit code `2`.
- **Execution Workspace**: Đánh giá bằng `git status --porcelain=v1 -uall -z`, lọc duy nhất bản ghi untracked chính xác:
  `?? .agents-coworkers/recovery-leases/<sha256(runId + "\n__run__")>.lease`
  Mọi trạng thái tracked, staged (`A `), modified (` M`, `M `), deleted (` D`, `D `), type changed, unmerged, rename hoặc copy tại cùng đường dẫn vẫn được giữ nguyên và làm workspace dirty. Ngoại lệ lease chỉ tồn tại để giữ tính lũy đẳng (idempotency) khi cùng run chuyển lease từ `FREE` sang `OWNED`.
- **`porcelainSha256`**: Là băm SHA-256 của canonical filtered source-porcelain này.

## 4. Kết Quả Kiểm Chứng Thực Tế

| Nhóm Kiểm Tra | Lệnh Thực Thi | Kết Quả | Ghi Chú |
|---|---|---|---|
| Mã nguồn Go | `gofmt -s -l ...` | Exit 0 | Không có sai lệch định dạng |
| Git Diff | `git diff --cached --check` | Exit 0 | Không có lỗi whitespace/formatting |
| Tĩnh học Go | `go vet ./...` | Exit 0 | Không có cảnh báo vet |
| Full Test Suite | `go test -count=1 ./...` | Exit 0 | Toàn bộ package pass |
| Race Detection | `go test -race -count=1 ./...` | Exit 0 | 0 race condition |
| L4 Durable Process Oracle | `go test -v -count=1 -run TestL4_DurableProcessOracle ./cmd/coworkers` | Exit 0 | 25/25 subtests, 26 subprocesses pass |

### Bảng Mã Thoát Thực Tế (Exit Code Mapping)
- `0` (Success): Hoàn thành kiểm tra môi trường (`doctor`), gắn kết session (`attach`), và truy vấn trạng thái (`status`).
- `1` (Unsupported / Usage): Lệnh chưa được hỗ trợ (`coworkers run` trong Slice 8A luôn trả về mã thoát `1`).
- `2` (Validation / Conflict / Auth): Thiếu tham số, sai cấu trúc `RunSpec`, model vắng mặt trong `/v1/models`, gateway 401/403, Product root dirty, lease bị lock, hoặc sai lệch binding branch.
- `3` (Endpoint Unavailable): AO daemon loopback không phản hồi hoặc timeout kết nối.

## 5. Giới Hạn & Phạm Vi Kiểm Thừa Nhận

1. **Không gọi Live Provider**: Toàn bộ kiểm thử dùng fake loopback server và run-owned Git fixture. Tuyệt đối không gọi completion API ra bên ngoài (`providerCallPerformed: false`).
2. **Không Chứng Minh Credential Eligibility Hay Usable Capacity**: Sự hiện diện của model trong danh mục `/v1/models` chỉ chứng minh khả năng hiển thị catalog, không nâng claim thành credential eligibility hay usable capacity (`credentialEligibility: "NOT_OBSERVED"`).
3. **Slice 8B Chưa Triển Khai**: Lệnh `coworkers run` chưa được kích hoạt, chưa hỗ trợ workflow dispatching, exit code 1.
4. **Quét Bí Mật (Secret Scan)**: 100% sạch; không chứa API key, token, cookie, email thật hoặc credential file path nhạy cảm trong RunSpec, RunManifest, log hay artifacts.
