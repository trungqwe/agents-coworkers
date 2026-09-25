# Phase 7D bounded recovery prototype

Status: `SIMULATED_PASS__LIVE_PROVIDER_RECOVERY_NOT_OBSERVED`. Scope is the isolated `agents-coworkers` worktree `phase7d-recovery-prototype`; AO/CLIProxyAPI/Product source and shared daemon state were not modified.

## Native AO audit and decision

| Option | Source-supported behavior | Decision |
|---|---|---|
| Native AO only | `RetryTurn` creates an idempotent replacement for one failed durable human turn, but it deliberately refuses uncertain delivery and does not distinguish a provider outage from a user Stop policy. `steer-or-send` reserves `clientMessageId`; `recoverOnly` reads the settled receipt without contacting the provider. | Reuse `clientMessageId`/`recoverOnly`, but native APIs alone do not schedule workload retry/checkpoint reconciliation. |
| Bounded dispatcher in `agents-coworkers` | Can own task policy, retry budget and next retry while reading AO session/delivery state and Git before a send. It need not access `ao.db` or alter interrupt semantics. | **Selected smallest delta.** |
| Patch AO | AO's transition-message dispatcher already implements a durable outbox and periodic scan, but only for interface-transition messages. Generalizing it changes AO lifecycle/API semantics. | Rejected for this slice; no upstream patch authority or demonstrated need. |

`/conversation/interrupt` remains terminal expected cancellation. The dispatcher does not drain or replay an interrupted turn. A human Stop is written durably as `CANCELLED` before any later dispatcher process can act.

## Prototype contract

- `internal/recovery/checkpoint.go`: atomic JSON checkpoint with task/delivery/session/turn IDs, artifact SHA-256, Git HEAD, side-effect state, retry budget, attempts and `nextRetryAt`.
- `internal/recovery/dispatcher.go`: single-owner `Step`/`Cancel` serialization; terminal `BLOCKED/CANCELLED/COMPLETED`; Git/artifact and AO observation before send; write-ahead uncertain checkpoint before provider I/O; recover-only/backoff with the same delivery ID. Only an explicit request-not-accepted result permits bounded resend.
- `internal/recovery/ao_http.go`: loopback-only client for official session/conversation and `steer-or-send` APIs. It stores AO turn handles, maps turn states and extracts a strict `TASK_RECEIPT` from the assistant reply.
- `internal/recovery/testdata/phase7d-fixture-checkpoint.json`: one bounded read-only fixture task, bound to baseline `0190e925...` and `go.mod` SHA-256 `0f96d491...`.
- AO stays authoritative for session/delivery state. The prototype uses ports and never reads or writes `ao.db`.

## Verification

Commands: `go test ./... -count=1 -v`; `go test -race ./internal/recovery -count=1`; `go vet ./...`; all exit 0. Recovery package 20/20 top-level tests PASS (plus receipt/terminal subcases); existing integration package 3/3 PASS.
Edited command receipt and exit codes: `verification.txt` in this evidence directory.

| Oracle | Result |
|---|---|
| Simulated provider failure | `SIMULATED PASS`: one send attempt, durable `WAITING_RETRY`, bounded attempts and `nextRetryAt`. |
| Dispatcher restart | `SIMULATED PASS`: new dispatcher reads the same checkpoint, sends nothing before deadline, then performs exactly one eligible attempt. |
| Delivery uncertain | `SIMULATED PASS`: recover-only called once with the original delivery ID; no ordinary send. |
| Completed task | `SIMULATED PASS`: completed side effect causes no send/recover after restart. |
| Lost receipt after accepted delivery | `SIMULATED PASS`: dispatcher blocks instead of replaying the previously delivered task. |
| Explicit human Stop | `SIMULATED PASS`: `Cancel` persists `CANCELLED`; a later dispatcher performs no I/O. |
| Terminal states | `SIMULATED PASS`: `BLOCKED/CANCELLED/COMPLETED` remain terminal through repeated Step/restart; zero Send/RecoverOnly. |
| Cancel/Step race | `SIMULATED PASS`: synchronized single-owner race proves Stop-first yields zero Send and final `CANCELLED`; race detector PASS. |
| AO completion versus task completion | `SIMULATED PASS`: missing/mismatched receipt blocks; only accepted receipt with exact task ID and artifact SHA completes the task. |
| AO HTTP adapter | `SIMULATED PASS`: loopback restriction, stable clientMessageId, recover-only payload, AO state mapping, strict receipt parsing and definitive validation classification. |
| Adapter transport failure | `SIMULATED PASS`: first failure becomes write-ahead `DELIVERY_UNCERTAIN`; restart issues recover-only with the same ID, never a second ordinary Send. |
| AO + Git preflight | `SIMULATED PASS`: AO observation precedes send; fixture binds artifact hash and Git HEAD; path escape rejected. |

No live provider/daemon recovery was attempted: the HTTP adapter is implemented and tested with a controlled transport, but no dedicated disposable AO session was available without mutating the shared daemon/session set. This preserves shared sessions and the distinction between `SIMULATED` and `LIVE`.

## Corrections and limits

- Historical T1/T2 were deliberately cancelled by `/conversation/interrupt`; they are expected cancellation, not proof that auto-wake is missing. Same-session resume and explicit wake remain `LIVE`; provider failure, daemon restart and unattended recovery remain `NOT OBSERVED`.
- Old `AO_BROWSER_CAPABILITY` invalidation is inferred from source plus observed exit/resume verifier rotation. The old token was not tested; rejection of that token is `NOT OBSERVED`.
- The prototype is a bounded library slice, not a daemon. Its mutex enforces one in-process owner; it does not yet implement cross-process lease/lock, periodic process ownership or live provider-recovery receipt. Those are the remaining integration gaps.

Minimal next step: wrap this library in one run-owned process with an explicit single-owner lease, then run the exact read-only checkpoint against a dedicated disposable AO fixture session. Inject transport loss after AO acceptance, verify recover-only returns the original turn and validate the artifact SHA receipt. Do not patch AO until that live fixture demonstrates a missing native primitive.
