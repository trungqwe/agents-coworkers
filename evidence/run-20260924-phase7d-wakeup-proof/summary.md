# Phase 7D fixture recovery and capability incident audit

Status: `SESSION_RESUME_LIVE__EXPLICIT_WAKE_LIVE__PROVIDER_RECOVERY_NOT_OBSERVED`. The proof is limited to existing AO fixture session `ao-phase5-repo-21`; no daemon restart, Product access, source edit, pool/model/default change, or commit/push/merge.

## Profile and durable checkpoint

- Existing AO project: `ao-phase5-repo`. Orchestrator `ao-phase5-repo-1` readback remained GPT-5.5/low; fixture worker C `ao-phase5-repo-21` remained worker/Codex/Gemini `gemini-3.8-flash-high/high`. No Astra profile was used in this continuation; GPT-6-Astra/low refers to earlier history only.
- Fixture root remained clean at `60aa78498d2e5325c96545d73d006e1f10b812a4`. C stayed on combined commit `1ab3169fb41e74e50e64fb47bf4ffc974d01a47f`, clean before and after proof.
- Checkpoint file: `checkpoint.json`; task ID `P7D-FIXTURE-READONLY-01`; delivery/clientMessageId `72343ba3-9b28-4a58-a98b-230bb762aea3`; artifact `tests/test_phase7_integration.py`, SHA-256 `7a615219b6d721655a337153c0f717e13fc6bca153cc989de3d6571c9fe25224`; next action was observe post-interrupt wake without replay.
- Checkpoint file SHA-256: `d213cacb1145d833101890f1155ca617d4179fb8db8d3c73052ad31a1f3a5bd5`.

## Capability incident — metadata only

- AO durable activity: session `ao-phase5-repo-21`, conversation command activity sequence 12, turn `0a331e95-04d1-468d-a117-98f689f0a49e`, timestamp `2026-09-24T10:52:27Z`, status completed. Exact child command PID was not retained in the activity (`NOT OBSERVED`); the owner boundary is the C session/controller. AO daemon process was PID 42608 on loopback port 3001.
- Capability type: `AO_BROWSER_CAPABILITY`, injected into the worker environment. AO source (`browser/authority.go`, `browser/service.go`, `session_manager/manager.go`) defines a random per-session bearer and persists only a one-way verifier. It scopes browser status/commands and session-owned preview-server operations for that session; it is not general shell or daemon authority.
- Source has no TTL, expiry timestamp, or single-use/consumed state. Authorization binds it to the session verifier and rejects a terminated session. Before remediation C was idle but not terminated, so the bearer was treated as potentially active; no attempt was made to replay or hash it.
- The command result remains in AO conversation history. It is retrievable through the unauthenticated loopback `GET /api/v1/sessions/{sessionId}/conversation` surface observed in this run; exposure scope is local processes able to reach the AO daemon on this machine and principals able to read the user's AO data store. No remote listener access was tested or claimed.
- Owner remediation, without direct DB edits/history deletion: official `POST /sessions/ao-phase5-repo-21/exit-agent` returned success (`exited`, not terminated); immediately followed by official `POST /sessions/ao-phase5-repo-21/resume-agent`, success with `resumeMode=native`, same session `idle`, worker profile retained. Source inspection shows this path issues and persists a fresh verifier before controller start, so invalidation of the previous bearer is an inference from source plus observed exit/resume. The old value was deliberately not re-read or tested; token-old rejection therefore remains `NOT OBSERVED`. Worktree/commit remained unchanged.
- Prevention: no full environment dump; inspect explicit allowlisted keys only, prefer presence/status over values, redact before persistence, and keep stdout/tool output to 4 KiB by default. The command-history value itself was not copied into this evidence; history was not removed.

## Controlled interruption and wake-up observations

| Step | AO delivery/message and turn | Durable result |
|---|---|---|
| T1 `P7D-FIXTURE-READONLY-01` | delivery `72343ba3-9b28-4a58-a98b-230bb762aea3`; user message `bf8bafc7-b752-490a-bceb-003c8f39829f`; turn `6e7385da-440b-4104-a6e2-fa42241844e2` | `running` then `interrupted`; only bounded sleep/read-only acknowledgement requested, no file edit or completed side effect |
| T2 `P7D-FIXTURE-RESUME-01` | delivery `731b1692-ffe9-462f-a106-f7e7327a0834`; user message `07c3fb9f-4b11-4f74-8259-e35c075c0c7f`; turn `9b9d2648-3fb3-40e3-a198-b25801bc085c` | accepted as `queued` before interrupt; after official interrupt HTTP 204 it became `interrupted` with no start time; no automatic wake |
| T3 `P7D-FIXTURE-MANUAL-WAKE-01` | delivery `5e38299d-65b7-4f04-a900-ed0e68a9e014`; user message `d415fde8-b838-4c0b-a1e2-9415fd6e12cc`; turn `52823805-d619-4eeb-a148-577ef6b000ee`; assistant reply `7f47928e-3359-464b-b6fd-eeae4f892b79` | an explicit new send after interruption returned `running`, then `completed`; response contained checkpoint task ID, artifact SHA and next action, but did not echo the original delivery ID. No tools, file changes, commit or replay occurred. |

LIVE: session controller exit/native resume; explicit new delivery waking the idle session; T1/T2/T3 durable turn states. T1/T2 were deliberately cancelled through `/conversation/interrupt`; this is expected Stop behavior and is not an auto-wake oracle. NOT OBSERVED: daemon restart/crash, provider outage, all-pool cooldown, unattended recovery, delivery-ID echo by resumed model, or recovery of external side effects.

## Exact source seam and audit conclusion

- Runtime binary SHA-256 `dce49a699c848761a4f23d28a7e6f7218ab6530345062c99b6d356c63725b371`; Go build revision `1140dd62dc7bb588b987e2c44aa1ff4796fa732b`, `vcs.modified=true`. This binds the binary to the current checkout revision but not a clean build tree.
- AO source: `backend/internal/service/chat/controller.go:3079` calls `drainLocked(ctx, settledTurnState(event) == TurnStateCompleted)`; `drainLocked` refuses queue dispatch when false. The interrupt path (`:2126`, `:2138`, `:2236`) records a queue cutoff and cancels pre-cutoff queued messages. That matches T1/T2 becoming interrupted. A post-cutoff explicit T3 send runs immediately; this is not unattended wake-up.
- `session_manager/manager.go:2248,2336`, `chat_spawn.go:373`, and `manager.go:4706` show owner exit preserves session/worktree and native resume issues/persists the replacement capability before controller start. `browser/service.go:77` binds authorization to session/verifier and rejects terminated sessions.
- Result: same-session resume and explicit wake are `LIVE`; provider failure, daemon restart and unattended recovery are `NOT OBSERVED`. A user/coordinator interrupt remains terminal cancellation and must never cause automatic replay. The subsequent bounded prototype is recorded separately; this historical run does not claim it.
- No credential value, raw environment, capability hash, token, or private runtime config is present in this evidence. No `ao.db` mutation or history deletion was performed.
