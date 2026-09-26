# 03 — Architecture Decision Records

> [!IMPORTANT]
> **Authoritative Decision Ledger**: This document is the single source of truth for architectural decisions in the `agents-coworkers` repository.
> Active Roadmap Phase: **Phase 8 — Productization Roadmap & Design Governance (DESIGN_PHASE / IMPLEMENTATION_NOT_AUTHORIZED)**; Phase 7 is **COMPLETE_WITH_ACCEPTED_LIMITATIONS** (authoritative status tracked in [04-ROADMAP.md](04-ROADMAP.md)).

### Decision Status
- `PROVISIONAL`: Accepted for exploration, subject to verification gates.
- `ACCEPTED`: Verified by runtime evidence; binding constraint.
- `SUPERSEDED`: Replaced by a later decision.
- `REJECTED`: Explored and eliminated.

### Evidence Maturity
- `STATIC`: Verified by source code analysis, contract inspection, or schema validation.
- `UNIT`: Verified by unit tests with mocked boundaries.
- `INTEGRATION`: Verified with mocked external services (e.g., mock LLM server).
- `LIVE_RUNTIME`: Verified end-to-end against live external authenticated provider or supervised session runtime (L5/L6/L7).
- `NOT_APPLICABLE`: Formal policy, governance rule, or architectural scope boundary not requiring empirical evidence.

---

### D001 — Candidate A Accepted
- **Decision Status**: `ACCEPTED`
- **Evidence Maturity**: `LIVE_RUNTIME` (Runtime verification across all 10 canonical gates complete; initial concurrency target proven in Gate 10)
- **Decision**: Candidate A (Codex harness via CLIProxyAPI gateway) is accepted as the runtime architecture for Agent Orchestrator integration.
- **Reason**: Maximizes harness uniformity, minimizes client diversity, and leverages existing provider pools.
- **Evidence**: Source inspection of AO Codex adapter, CLIProxyAPI translators, schema tests.
- **Revisit Trigger**: Revisit Candidate A only after a runtime gate has an unresolved, reproducible failure that has been root-cause isolated to a Candidate A architectural boundary. Configuration mistakes, authentication mistakes, transient provider errors, or unrelated local failures are not architecture-revisit triggers. Candidate B/C must remain dormant until such a failure is proven.

### D002 — Unified Codex Client Harness
- **Decision Status**: `PROVISIONAL`
- **Evidence Maturity**: `LIVE_RUNTIME`
- **Decision**: Use `agent = "codex"` for both orchestrator and worker roles (`INV-003`).
- **Reason**: Eliminates multi-harness drift, leverages existing `cmd/ao` codex adapter.
- **Evidence**: `backend/internal/adapters/agent/codex.go` source inspection; schema tests.
- **Revisit Trigger**: Codex CLI changes or AO adapter incompatibilities.

### D003 — CLIProxyAPI Local Routing Boundary
- **Decision Status**: `ACCEPTED`
- **Evidence Maturity**: `LIVE_RUNTIME`
- **Decision**: The initial nominal target pool (6 Plus + 8 Pro) is superseded historical context; the active pool (1 Codex + 7 Gemini) is tracked as historical inventory. Pool capacity expansion is governed strictly by empirical telemetry data (Slice 8E) rather than preemptive quota expansion.
- **Reason**: Decouples credential lifecycle and rate limiting from AO core logic.
- **Evidence**: Proxy schema inspection, upstream issue #32 compatibility analysis.
- **Revisit Trigger**: Proxy latency, protocol incompatibilities, or security constraints.

### D004 — AO Upstream Clean by Default
- **Decision Status**: `ACCEPTED`
- **Evidence Maturity**: `STATIC`
- **Decision**: Zero upstream patches to AO core (`INV-002`). All integration via config and existing CLI/API interfaces.
- **Reason**: Upstream maintainability, clean upgrade path.
- **Evidence**: AO configuration interface accommodates proxy endpoints natively.
- **Revisit Trigger**: Architectural limitation that cannot be resolved via configuration.

### D005 — Zero-Patch Path A as Preferred Configuration
- **Decision Status**: `ACCEPTED`
- **Evidence Maturity**: `STATIC`
- **Decision**: Path A (AO config: provider=openai, wire_format=chatgpt, base_url=proxy, effort=low) is the primary architecture.
- **Reason**: Zero code changes to AO; clean operational deployment.
- **Evidence**: Configuration schema analysis; verified compatible with AO v0.10.x.
- **Revisit Trigger**: Path A fails at runtime (e.g., tool call parsing or model name issues).

### D006 — Headless Effort Patch Maintained as Optional Fallback
- **Decision Status**: `PROVISIONAL`
- **Evidence Maturity**: `STATIC`
- **Decision**: Keep the one-line headless effort patch in `ao/` as an optional fallback only, not deployed in Path A.
- **Reason**: Path A avoids all patches; patch retained for documentation and potential future use.
- **Evidence**: Code inspection of `backend/internal/adapters/agent/codex.go`.
- **Revisit Trigger**: Path A failure requiring thinking-level control that cannot be achieved via config.

### D007 — Worker Concurrency Target Starts at 3, Not 7
- **Decision Status**: `ACCEPTED`
- **Evidence Maturity**: `LIVE_RUNTIME` (empirically proven in Gate 10 with 3 concurrent workers, 38.848216s overlap)
- **Decision**: Prove 3 concurrent workers first (`maximum proven live concurrency = 3`), which serves as the default scheduler limit (1–3 workers). Scaling to 4–7 workers is classified as `EXPERIMENTAL_CAPACITY` and requires separate authority and capacity proof.
- **Reason**: Risk management; resource limits on development machines; incremental verification.
- **Evidence**: Concurrency testing analysis; rate limit considerations.
- **Revisit Trigger**: Runtime gate failure under concurrency, or successful proof of 3 workers with request for higher throughput.

### D008 — OpenCode and Agy as Dormant Fallbacks
- **Decision Status**: `PROVISIONAL`
- **Evidence Maturity**: `STATIC`
- **Decision**: OpenCode (Candidate B) and Agy (Candidate C) adapters remain dormant in `archive/` or design docs.
- **Reason**: Focus on Candidate A; avoid multi-harness maintenance overhead.
- **Evidence**: Source inspection of both candidate designs.
- **Revisit Trigger**: Candidate A fails an architecture revisit trigger (see D001).

### D009 — Runtime Proof Outranks Static Inference
- **Decision Status**: `ACCEPTED`
- **Evidence Maturity**: `STATIC`
- **Decision**: No gate is marked PASS without empirical evidence from the actual verification tier (`INV-004`).
- **Reason**: Avoid false confidence from static analysis; ensure production readiness.
- **Evidence**: Core governance policy. Pre-auth audit correctly halted at `BLOCKED_RUNTIME_AUTH`.
- **Revisit Trigger**: Fundamental change in verification policy (not permitted).

### D010 — Product Repository Remains Strictly Read-Only
- **Decision Status**: `ACCEPTED`
- **Evidence Maturity**: `STATIC`
- **Decision**: Keep `AI-Auto-Video-Creator` repository ROOT checkout strictly read-only (`INV-001`). Authorized isolated git worktrees created for a specific run may perform writes strictly governed by the run-specific allowlist. `INV-001` protects the product root and does not prohibit authorized isolated run worktrees. Autonomous integration in Slice 8D stops strictly at the local integration branch (`integration/run-<id>`); merging or pushing to the Product main branch is strictly prohibited.
- **Reason**: Protect Product baseline, maintain accepted status across all Product milestones, prevent unverified writes to main.
- **Evidence**: Product root clean at commit `4a7c8c921b7e05066505d51b168a02c3fde61317`.
- **Revisit Trigger**: Explicit user authorization changing product milestone boundaries.

### D011 — Pool capacity requires live per-credential evidence
- **Decision Status**: `ACCEPTED`
- **Evidence Maturity**: `LIVE_RUNTIME` cho subset request đã quan sát; phần còn lại `NOT_OBSERVED`.
- **Decision**: Phân biệt số credential đăng ký, đã nạp, đủ điều kiện, được chọn và request thành công theo provider/model. Inventory hoặc catalog không chứng minh phân phối quota; `X-CPA-TRACE-ID` chỉ chứng minh credential được chọn cho request đã quan sát. Khi session affinity bật, không đòi round-robin ở từng request. Không suy quota còn lại từ token usage.
- **Evidence**: `evidence/run-20260923-phase8-pool-onboarding/`: 1 Codex + 7 Gemini danh tính riêng đã đăng ký; 14 request fixture tuần tự tới `gemini-3.8-flash-high/high` trả 200, 5 credential ẩn danh được chọn. Các test fake executor/selector là `SOURCE/UNIT`, không phải `LIVE_FAILOVER`; bốn lỗi 429 cũ vẫn `UNKNOWN`.
- **Giới hạn**: Binary gateway tự báo `dev/none/unknown`; SHA-256 binary và source SHA được lưu nhưng chưa chứng minh binary-build correspondence. Gemini chưa được chọn, Codex pool routing, quota còn lại và live failover chưa được chứng minh. Concurrency proven vẫn là 3; Phase 8 đang ở `DESIGN_PHASE / IMPLEMENTATION_NOT_AUTHORIZED`.
- **Revisit Trigger**: Dữ liệu telemetry thực tế từ Slice 8C/8D chứng minh quota cạn kiệt hoặc hành vi routing mới.

### D012 — Workforce là sản phẩm; AI Video Creator là workload mẫu
- **Decision Status**: `ACCEPTED` theo yêu cầu hiệu chỉnh governance.
- **Evidence Maturity**: `NOT_APPLICABLE` cho quyết định; Phase 7 đã đạt `COMPLETE_WITH_ACCEPTED_LIMITATIONS`.
- **Decision**: `agents-coworkers` là sản phẩm workforce độc lập; `AI-Auto-Video-Creator` phục vụ như một workload mẫu để kiểm thử và hoàn thiện workforce, không thay thế mục tiêu sản phẩm. Người dùng giao mục tiêu một lần; orchestrator tự phân việc, nhận kết quả, review/rework và tích hợp/test qua AO. Model/effort của orchestrator và worker do user chọn theo từng run từ catalog AO/gateway đang hỗ trợ sau preflight, không khóa cố định vào Astra.
- **Supersedes**: Ràng buộc trong roadmap/kế hoạch cũ coi hoàn tất P8/Admin UI/production launcher/P9 của Product là điều kiện hoàn thành hoặc sử dụng workforce. Phase 7 đã đạt `COMPLETE_WITH_ACCEPTED_LIMITATIONS`.
- **Làm rõ D004/D005**: Zero-patch vẫn ưu tiên, không cấm đề xuất integration feature/patch nhỏ cho gap có source/oracle. Không cấp quyền sửa upstream trong lượt docs-only.
- **Cập nhật D010**: Root Product vẫn read-only; ngoại lệ đã cấp gồm CP1 và CP2A backend trong `ai-auto-video-creator-2`, không chỉ CP1. CP1 accepted; CP2A có implementation nhưng chưa accepted, hiện bảo toàn/chưa tiếp tục. P9/M3/Phân hệ A khóa.
- **Evidence correction**: CP2A có coordinator sửa trực tiếp sau worker patch invalid/no-op; không là proof Astra-worker tự hoàn tất vòng điều phối. E3 đã chạy thử và thất bại prerequisite, chưa có verification hợp lệ. P7B hardening 37/38, M1 loại E3 92 passed/1 deselected; Temporal cleanup pending. Inventory chi tiết và nguồn tại [kế hoạch workload](phase7-first-workload-plan.md).
- **Effect**: Thiếu credential E3 chỉ chặn audit workload AI Video Creator, không chặn hoàn thiện workforce. Phase 7 đã COMPLETE_WITH_ACCEPTED_LIMITATIONS; Phase 8 chuyển trọng tâm sang productization của `coworkers`.
- **Revisit Trigger**: Thay đổi mục tiêu sản phẩm hoặc capability gap cần contract/scope mới.

### D013 — Run-Scoped Model Profiles
- **Decision Status**: `ACCEPTED`
- **Evidence Maturity**: `LIVE_RUNTIME`
- **Decision**: User selects orchestrator and worker model/effort for each run. Before dispatch, verify gateway/AO catalog presence, credential eligibility and exact session readback. Do not silently substitute models. A profile-specific failure blocks only that profile unless evidence shows a wider provider/pool outage.
- **Evidence**: Historical GPT-6 Astra/low and GPT-5.5/low orchestrator runs plus Gemini high worker/orchestrator runs; the failed GPT-5.5 Phase 7E bootstrap remains recorded as `BLOCKED_PROFILE_ELIGIBILITY_GPT55`, not global capacity exhaustion.
- **Revisit Trigger**: AO introduces an authoritative policy layer that explicitly binds roles to models and the user adopts that policy.

### D014 — Target Product Entrypoint CLI (coworkers)
- **Decision Status**: `PROVISIONAL` (Phase 8 Design Proposal — Implementation Not Authorized)
- **Evidence Maturity**: `STATIC`
- **Decision**:
  1. The target product operational user entrypoint is named `coworkers`.
  2. `coworkers` is architected as a thin control-plane CLI that talks to AO backend and CLIProxyAPI exclusively via loopback HTTP REST APIs.
  3. AO endpoint resolution follows a strict priority order; all AO and Gateway URLs must strictly use loopback hosts (`localhost`, `127.0.0.1`, or `::1`):
     (1) explicit `--ao-url` flag (invalid URL or non-loopback fails closed, does not fallback; verifies `GET /api/v1/identity`; does not use OS port inspection to infer PID; records `aoDiscoverySource: "explicit_url"`, `aoPid: 0`, `aoPidStatus: "NOT_OBSERVED"`);
     (2) `AO_BASE_URL` environment variable (invalid URL or non-loopback fails closed, does not fallback; verifies `GET /api/v1/identity`; records `aoDiscoverySource: "environment_url"`, `aoPid: 0`, `aoPidStatus: "NOT_OBSERVED"`);
     (3) `AO_RUN_FILE` environment variable (file path to run metadata, e.g. `running.json`, not a URL; corrupt file or invalid endpoint fails closed; parses `running.json`, verifies `pid > 0` and process is alive, constructs loopback URL from port, verifies `GET /api/v1/identity`; records `aoDiscoverySource: "run_file"`, `aoPid`, `aoPidStatus: "VERIFIED"`);
     (4) default candidate files: `~/.ao/dev/running.json`, `~/.ao/running.json` (only scanned when none of the three priority sources above exist; same verification as `AO_RUN_FILE`);
     (5) PID Contract Rule: `/api/v1/identity` only returns `hostId` and `apiVersion`/`contractVersion`, not process PID. Only `running.json` contains PID, port, and startedAt. Therefore, `aoPidStatus: "VERIFIED"` is strictly declared only when endpoint is discovered from `running.json`. Never infer PID via OS port inspection when URL is explicitly supplied or from environment;
     (6) multiple valid default candidates detected -> fail-closed with exit code 2 and halt without auto-selecting; 0 valid default candidates -> exit 3.
     Port 3005 is not a fixed architectural port. Gateway URL resolution follows the same fail-closed principle (strictly loopback host):
     (1) explicit `--gateway-url` flag (invalid URL fails closed, does not fallback);
     (2) `COWORKERS_GATEWAY_URL` environment variable (invalid URL fails closed, does not fallback);
     (3) default historical loopback URL: `http://127.0.0.1:8317`.
  4. Gateway Authentication & Probe:
     Gateway catalog probe uses `GET /v1/models`. When the endpoint requires authentication, the gateway key is retrieved strictly from the `CLIPROXY_KEY` environment variable in the process environment. Never include the key in `RunSpec`, `RunManifest`, argv, stdout, stderr, or evidence artifacts. Never log raw `Authorization` headers. Missing required credentials or HTTP 401/403 returns exit code 2. Unreachable endpoint or connection timeout returns exit code 3. Invalid JSON/catalog returned by gateway returns exit code 2.
     Catalog presence strictly proves model visibility in the catalog and does NOT constitute credential eligibility or usable capacity (`providerCallPerformed: false`, `credentialEligibility: "NOT_OBSERVED"`). Credential eligibility is verified only in Gate 8C/8D under separate user authority.
  5. It does not introduce any competing daemon, separate database, or alternative session state store. Direct read/write to AO SQLite (`ao.db`) is strictly prohibited.
  6. Core recovery, checkpoint, preflight, and lease management logic is reused directly from `internal/recovery`. The existing `cmd/recovery` CLI remains maintained for backwards compatibility as a narrow checkpoint/recovery tool during the transition period; recovery logic must not be duplicated into the new command.
  7. Data contract separation:
     * `RunSpec` (JSON format, immutable user input, `schemaVersion = "run-spec/v1-draft"`): defines target root (Product/root checkout path), execution workspace (AO execution worktree path), baseline SHA, `expectedBranch` using placeholder `<AO_SESSION_BRANCH>` (representing the expected AO session/worktree branch), profiles, policies, and delegated authority. It does NOT contain `aoUrl` or `gatewayUrl`; endpoints are resolved exclusively via discovery precedence recorded in `endpointPolicy`. It explicitly includes `requiredTools` (`git` is required; additional tools checked only when declared, verified via `exec.LookPath` without shell invocation; Go/Node are not hardcoded target project prerequisites) and `workspacePolicy` (`productRootMustBeClean: true`, `executionWorkspaceDirtyPolicy`: `require_clean` or `allow_dirty_recorded`).
     * `RunManifest` (JSON format, generated strictly by `coworkers attach`, `schemaVersion = "run-manifest/v1-draft"`): records `runSpecSha256`, `generatedBy: "coworkers attach"`, resolved endpoints (aoUrl, aoIdentity, aoDiscoverySource, aoPid, aoPidStatus), `gatewayProbe` (url, catalogPath: "/v1/models", catalogStatus: "VERIFIED", observedModels, `providerCallPerformed: false`, `credentialEligibility: "NOT_OBSERVED"`), target repository identity, baseline SHA, current HEAD, `sessionBranch` and `worktreeBranch` (both must match), `worktreeBinding` (canonicalPath, worktreeBranch, head, isClean, dirtyPolicy, porcelainSha256, verifiedPorcelain), single `attachedSessionProfile` (kind, harness, model, reasoningEffort, status), `lease` (workspaceRoot, runOwner, taskId: "__run__", ownerId, observedState: "FREE", observedPid: 0), source provenance, and timestamps. Free of secrets and redundant endpoint inputs.
     * Command syntax: `coworkers doctor --spec <run-spec.json>` (spec is mandatory for doctor), `coworkers attach --spec <run-spec.json> --manifest <run-manifest.json> --session <id> --workspace <path>`, and `coworkers status --manifest <run-manifest.json>` (implemented purely using Go standard library `encoding/json`, no YAML dependency).
  8. Lifecycle of RunSpec & RunManifest:
     * `coworkers doctor --spec <run-spec.json>`: `--spec` is required. Validates environment, tools per `requiredTools`, baseline, endpoints, and model catalog. Outputs `DoctorReport` to stdout/JSON. Does NOT create or modify `RunManifest`. Does NOT accept `--manifest`. Does not inspect or acquire lease.
     * `coworkers attach`: Reads `RunSpec`, accepts `--session`, `--workspace`, and `--manifest` (output path). Validates `workspacePolicy`: Product root evaluated with raw `git status --porcelain=v1 -z` without exceptions (any file including under `.agents-coworkers/**` makes Product root dirty -> exit 2); Execution workspace evaluates source-dirty from raw `git status --porcelain=v1 -uall -z` after filtering out strictly the single exact untracked record calculated from `RunSpec.runId` (`?? .agents-coworkers/recovery-leases/<sha256(runId + "\n__run__")>.lease`); any tracked, staged, modified, deleted, type-changed, unmerged, rename or copy record at that path remains source-dirty; do not treat arbitrary `.agents-coworkers/**` files as expected; the exact lease exception exists solely to preserve idempotency when the same run transitions from FREE to OWNED; `require_clean` fails on dirty worktree (exit 2); `allow_dirty_recorded` allows dirty worktree (PASS) and records `porcelainSha256` as the SHA-256 of this canonical filtered source-porcelain (do not infer arbitrary dirty state as expected). Verifies session and worktree using the mandatory 9-step binding algorithm. Inspects lease read-only via API (`workspaceRoot`, `runOwner`, `taskId: "__run__"`). Strictly does NOT call Acquire or Release. Generates `RunManifest` via atomic write (temp file -> fsync/close -> rename). If manifest does not exist, creates it. If manifest already exists with identical `runId`, `runSpecSha256`, session, repository, and worktree binding, returns idempotent success without rewriting. If manifest is corrupt or has mismatched bindings, fails closed with exit code 2. Prior manifest creation is not a prerequisite.
     * `coworkers status`: Strictly reads `RunManifest` and live read-only state. Re-inspects current lease state from `workspaceRoot`, `runOwner`, `taskId: "__run__"`. Does NOT trust old `observedState` in manifest. Does not auto-break stale leases. If manifest is missing or corrupt, exits with code 2. Never reads `RunSpec` to reconstruct binding. Never modifies manifest. Contains no Task DAG fields in Slice 8A.
  9. Strict Worktree Binding Algorithm for `coworkers attach`:
     Do not trust `RunSpec` or `RunManifest` in isolation. The CLI must execute the following binding check:
     (1) Read session via AO API and extract project ID and branch;
     (2) Canonicalize target root and explicit execution workspace path;
     (3) Run `git worktree list --porcelain -z` on the target repository;
     (4) Find exactly one worktree whose branch matches the session branch;
     (5) The canonical path of that worktree must match the execution workspace;
     (6) Worktree HEAD must match the expected HEAD/baseline for the run state;
     (7) Repository common-dir and identity must match the target repository;
     (8) Missing worktree, multiple matching worktrees, detached state outside contract, or path mismatch -> exit 2;
     (9) Never read `ao.db` directly and never infer physical worktree path from AO `SessionView`.
  10. Standardized Exit Codes:
      * `0`: Success.
      * `1`: CLI usage error or unsupported command (`coworkers run` in Slice 8A always exits with code 1).
      * `2`: Validation, identity, baseline, worktree, lease conflict, missing/invalid credential, tool validation error, or invalid catalog.
      * `3`: Endpoint unavailable or timeout.
  11. Run Lease Governance:
      `FileLease` is located under `<executionWorkspace>/.agents-coworkers/recovery-leases`, with lease filename derived from SHA-256 of `runOwner + "\n" + taskId`. Slice 8A run lease uses fixed identity: `runOwner = RunSpec.runId`, `taskId = "__run__"`. Valid states: `FREE` (lease file absent), `OWNED` (lease exists matching run), `LOCKED` (lease exists owned by another run/task). State `ACQUIRED` is NOT used in Slice 8A manifest. `FileLease` uses `O_EXCL` to coordinate mutual exclusion between `coworkers`/`recovery` processes adhering to the wrapper contract. It is not a global AO lock and does not prevent external AO clients from using the session. It has no TTL and does not auto-break stale leases. PID is monitoring metadata only; stale lease cleanup requires separate ownership/liveness verification outside the CLI wrapper. Attach strictly calls read-only inspect API, without calling Acquire or Release. Slice 8A only adds a read-only inspect API, without modifying Acquire/Release semantics.
  12. Exact File Allowlist for Slice 8A Implementation (17 files, no wildcards):
      - `cmd/coworkers/main.go`
      - `cmd/coworkers/main_test.go`
      - `internal/workforce/control/types.go`
      - `internal/workforce/control/run_spec.go`
      - `internal/workforce/control/run_spec_test.go`
      - `internal/workforce/control/manifest.go`
      - `internal/workforce/control/manifest_test.go`
      - `internal/workforce/control/discovery.go`
      - `internal/workforce/control/discovery_test.go`
      - `internal/workforce/control/doctor.go`
      - `internal/workforce/control/doctor_test.go`
      - `internal/workforce/control/attach.go`
      - `internal/workforce/control/attach_test.go`
      - `internal/workforce/control/status.go`
      - `internal/workforce/control/status_test.go`
      - `internal/recovery/lease.go` (only add read-only inspect API accepting workspaceRoot, runOwner, and taskId needed for status and attach inspection; do not modify Acquire/Release semantics; no TTL, no auto-break)
      - `internal/recovery/lease_test.go`
      No modifications to `go.mod` or `go.sum`; only Go standard library is permitted.
- **Reason**: Preserves single source of truth within AO daemon, prevents database split-brain corruption, leverages proven recovery foundations from Phase 7, and enforces strict staged governance before autonomous dispatch.
- **Evidence**: Architectural boundary audit of `internal/recovery`, `cmd/recovery`, and AO HTTP controllers.
- **Revisit Trigger**: AO provides a first-class supervisor extension API that replaces external loopback control.

### D015 — Telemetry-Driven Routing Observability & Evidence-Based Pool Capacity
- **Decision Status**: `ACCEPTED`
- **Evidence Maturity**: `STATIC`
- **Decision**:
  1. Exact source provenance of CLIProxyAPI checkout is commit `2430354330af80b645f9ffb1a51e1e7c72c4cc8e`. Note: source audit of CLIProxyAPI does not prove the binary runtime currently running was built from that exact SHA.
  2. Do not assume `agents-coworkers` can observe selected credentials, cooldown windows, `Retry-After`, or failover trails unless the gateway explicitly exposes them in HTTP responses.
  3. Header `X-CPA-TRACE-ID`: Has format `<YYYYMMDDHHMMSS>-<authIndex>-<requestID>`. `authIndex` is a stable pseudonymous 16-hex identifier (SHA-256 of credential seed) that presents correlation risk across multiple requests. While it does not contain raw tokens, it is sensitive operational metadata requiring redaction and bounded retention; không được coi là an toàn tuyệt đối (must not be claimed as absolutely safe). When header is absent (credential not selected or trace callback bypassed), record as `NOT_OBSERVED` or `UNKNOWN`.
  4. Header `Retry-After`: Only used when actually present in response headers from gateway/upstream; do not assume every 429 or 503 includes `Retry-After`. When absent, fall back to default exponential backoff with jitter.
  5. HTTP Status Code Classification: Do not map every 402 to `deactivated_workspace` or every 403 to `VALIDATION_REQUIRED` (these were observed provider-specific subcodes). Error classifier must combine HTTP status code, structured error code/reason in JSON body, and response headers. Missing data must be classified as `UNKNOWN`.
  6. Solution Hierarchy: (1) Native API / existing response headers; (2) Safe client-side adapter / wrapper tracking; (3) Header patch candidates (e.g. `X-CPA-Failover-Attempts`) only after separate audit and approval. Do not propose patches as default direction.
  7. Pool Capacity & Slice 8E: The existing pool of 1 Codex + 7 Gemini is recorded as historical inventory, not guaranteed usable capacity. Slice 8E is a conditional slice: if Slice 8C/8D telemetry proves the existing pool is sufficient, status is `NOT_TRIGGERED` or `NOT_REQUIRED_WITH_EVIDENCE`, no onboarding is conducted, and Phase 8 can close. Onboarding is triggered only upon genuine telemetry-proven capacity exhaustion under separate user authority.
  8. Use real repository paths: `scripts/auth-inventory.ps1` and `config/cliproxy/`; do not reference non-existent paths.
- **Reason**: Enforces evidence-first principles for infrastructure and routing, prevents credential secret leaks in logs, avoids artificial quota burn, and maintains accurate repo paths.
- **Evidence**: Source audit of `CLIProxyAPI/internal/logging/cpa_trace.go` and `CLIProxyAPI/sdk/api/handlers/handlers_errors.go`.
- **Revisit Trigger**: Gateway exposes new structured telemetry headers or an explicit management API.
