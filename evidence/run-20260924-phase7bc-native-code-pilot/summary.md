# Phase 7B–7C — AO fixture code/integration pilot: audit partial

Trạng thái: `7B_CODE_OVERLAP_OBSERVED__REVIEW_BLOCKED_CAPACITY`; 7C integration **NOT RUN**. Dừng live retry sau ba turn Astra fail cùng `503 auth_unavailable`; không gọi Phase 7 COMPLETE. Run 7A và mọi Product evidence giữ nguyên.

## Authority, graph và preflight

- User cho phép đúng fixture `ao-phase5-repo`, tối đa ba AO worker worktrees, bốn file Python, local commits/cherry-pick và `unittest`; không ghi fixture root/main, Product, AO/CLIProxyAPI hoặc project defaults. Root fixture sạch tại `60aa78498d2e5325c96545d73d006e1f10b812a4`; bốn đường dẫn chưa tồn tại/tracked trước dispatch. Product root sạch tại `4a7c8c921b7e05066505d51b168a02c3fde61317`; CP2A worktree dirty cũ không được ghi.
- Astra `ao-phase5-repo-1` readback `orchestrator/codex/gpt-6-astra/low/chat`; worker A `ao-phase5-repo-19`, B `ao-phase5-repo-20` readback `worker/codex/gemini-3.8-flash-high/high/chat`, idle/clean/base đúng trước task. Default, pool, sandbox không đổi. Integration worker C **không tạo**.
- Astra tự ghi contract trong AO conversation trước dispatch: `A ∥ B → kiểm receipt/diff/test → ACCEPT A+B → C cherry-pick A rồi B → facade/test → audit`. A chỉ `pilot_arithmetic.py` với `add/subtract(float,float)->float`; B chỉ `pilot_division.py` với `divide(float,float)->float`, mẫu số `0` hoặc `-0.0` raise `ValueError('division by zero')`; C chỉ ghi `pilot_calculator.py` và `tests/test_phase7_integration.py`, facade route add/subtract/divide, unknown operation raise `ValueError('unsupported operation: <operation>')`. Oracle chung dự kiến `python -m unittest discover -s tests -p test_phase7_integration.py` trên combined head; **chưa chạy**.

## AO provenance và kết quả đã quan sát

| Task | Session; conversation/turn | Dispatch message | Code commit (parent chung `60aa784…`) | File SHA-256 | Kết luận |
|---|---|---|---|---|---|
| A | `ao-phase5-repo-19`; conversation `7d11c6ad-3b4b-4cb0-a017-08a6a65a626f`, turn `8a3c89cc-0c49-48fb-9ee4-badcaefb0c3b` | `b045d5d4-ec25-473e-bbe0-5f050b77dbaa` | `ffa4562594b5385a9b625bfb0b9cf9719bfaf4b8` | `d3ea9349e9d626078536a0d9ab6b90228137830e9321b5d0432a9a3cf9ad99dc` | Receipt AO `f8f79bbf-286b-456d-a50a-75e5a0d9401f`; Astra chưa ACCEPT |
| B | `ao-phase5-repo-20`; conversation `25aa1482-529b-4550-8f06-e22cb4d87e0f`, turn `9eee3442-a290-41d8-9a0b-88a9f1e7c094` | `44c8f70f-e962-492d-b1dc-03fb6aa331ee` | `afc70b8db27585496052e13f49f32bf7d3e5a1ed` | `7fda8c745f13df4e179ff9b6197e33878013cafcdd51e85ce2911d460a8ce399` | Final receipt ở worker AO conversation; các message gửi Astra trước đó chỉ là quoting probes, Astra chưa ACCEPT |

- Mỗi commit parent đúng baseline, diff-tree chỉ đúng file owner, worktree A/B cuối sạch. Git blob A `5089bfe14f485f540aea044554d62a79e54afcbe`; B `cacf14d5cf14b2da5f941019a36cf731a3cf2de5`. Fixture root/main vẫn tại baseline sạch. Không cherry-pick/integration commit.
- AO turn A chạy `2026-09-24T05:11:12Z–05:23:48Z`; B `05:11:12Z–05:25:29Z`: overlap **12 phút 36 giây** theo AO turn, không khẳng định CPU/provider đồng thời. A/B code và local commit được quan sát trên hai worktree cách ly.
- Test lẻ có AO command receipt: A activity `85`, PowerShell `Invoke-Expression` chạy inline `python -B -c` kiểm signed/fractional/zero/type; raw command SHA-256 `42a45860d9deaa880e62516201c84fad4b91ee395ed981d5bef6cc73cd3bbfb9`, output `TEST_PASSED_ALL_CHECKS_VERIFIED`, inner/outer exit 0. B activity `70`, `python -B -c` kiểm signature, signed/fractional/zero và exact ValueError; command SHA-256 `8534b4e095613985e83118c146ee47725776015266764df987fcd39920abbf1f`, output `ALL TESTS PASSED`, exit 0. AO activity IDs trỏ tới exact command; không lưu raw conversation/request trong evidence. A activity `72` từng có Python SyntaxError dù outer shell exit 0; không tính PASS. B command mở rộng sau commit bị auto-review từ chối do gateway 503; không tính PASS.
- Astra đã quan sát A/B commits và đang đọc parent/hash/test, **chưa đưa ACCEPT/REWORK cuối**, chưa review combined diff, chưa tạo C, chưa chạy test chung. Không tạo lỗi giả để ép rework; rework thực tế `NOT OBSERVED`.

## Gián đoạn và can thiệp

- Bootstrap human-origin turn `ddc263ca-1a3d-4dd4-8a37-6ec3e85272a9` fail `503 auth_unavailable` sau upstream `PROTOCOL_ERROR`, khi delegate đã tạo A/B idle nhưng chưa dispatch. Coordinator kiểm 0 message/turn và clean worktree rồi gửi một bootstrap phục hồi `a6f9ab25-1f6f-4222-ad6b-6350c80a9028`, yêu cầu reuse đúng A/B; Astra tự dispatch A/B. Không dùng `RetryTurn` trên bootstrap cũ vì delegate side effect đã xảy ra.
- Khi Astra xử lý message thử định dạng từ B, automation-origin turn `2c44e114-6243-49f0-8108-5d847e6d8246` fail cùng 503. Source `backend/internal/service/chat/controller.go:3077` giữ queued turns sau failed turn; `RetryTurn` tại `:1415` chỉ nhận human-origin, nên automation-origin failure không retry được qua API đó. Coordinator chờ, kiểm receipts/commits rồi gửi human-origin recovery bootstrap `738eac39-0320-4b4e-86f2-5904416b2302` (message `18926596-8e98-42e1-bcc2-eccfbe465888`), không relay findings hoặc code. Astra bắt đầu tự đọc lại AO state nhưng turn này cũng fail cùng 503. Không có `Retry-After`; không thử lần thứ tư, không tạo session thay thế. Sáu AO turns vẫn queued tại audit, không coi là đã xử lý.
- Worker B gặp cùng 503 ở `codex-auto-review` cho một command kiểm mở rộng, dừng ngay. A/B self-test trước đó đã chạy; không bật bypass/allow-all. Astra có hai steering nội bộ: chặn A đọc environment/config quá rộng và hỗ trợ B metadata ID; coordinator không quyết định code/review thay Astra. Không có approval thủ công qua AO API trong lượt này.
- Chưa có proof native wake-up sau outage; source giữ queue có chủ ý để không cascade. Thiếu capacity của **Codex/Astra và auto-review** là blocker trực tiếp; không suy đó là 429 hoặc quota credential. Pool/credential chưa thay. Recovery wake-up/restart và repo portability vẫn `OPEN`.

## Điểm dừng và integrity

- Không sửa 7A evidence, Product root/worktree, fixture root/main, AO/CLIProxyAPI source. A/B commits được giữ tại đúng worker branches; không push/merge. 7B chỉ đạt code overlap/ownership và local test, chưa đạt acceptance Astra review; 7C chưa bắt đầu.
- Khi capacity phục hồi, cần đọc lại AO failed/queued turn và hai commit/test receipts, rồi cho chính Astra chốt ACCEPT/REWORK trước khi tạo C. Không được phát lại A/B side effects hoặc dùng coordinator tự cherry-pick để biến pilot thành PASS. Lượt hiện tại dừng ở audit partial theo stop condition.
