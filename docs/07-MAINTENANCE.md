# 07 — Maintenance & Anti-Drift Governance

This document establishes the binding governance contract for engineers and coding agents modifying this repository. Its primary purpose is to prevent architectural drift, test bloat, and documentation fragmentation over successive iterations.

---

## 1. Mandatory 10-Step Change Sequence

Every modification to this repository must follow this sequence in order:

1. **Read [README.md](README.md)**: Identify the control plane and review reading order.
2. **Locate Authority**: Identify the single authoritative document governing the affected domain from the Documentation Authority Model.
3. **Verify Roadmap Phase**: Check [04-ROADMAP.md](04-ROADMAP.md) to confirm the currently active phase.
4. **Apply Anti-Drift Test**: Verify that the proposed change directly advances an active phase exit criterion. If not, do not make it.
5. **Check Decisions**: Review [03-DECISIONS.md](03-DECISIONS.md) to verify that the change does not violate an accepted or provisional decision.
6. **Execute Surgical Change**: Make the minimal necessary code or documentation change. Avoid drive-by refactorings.
7. **Targeted Verification**: Run only the tests relevant to the affected invariants as defined in [05-VERIFICATION.md](05-VERIFICATION.md).
8. **Update Single Authority**: Update the authoritative document only if technical ground truth has changed.
9. **No Routine Doc Churn**: Do not update documentation merely because a routine test passed or was re-executed.
10. **Record Frozen Evidence**: When executing milestone verifications, record raw execution logs under `/evidence/run-<date>-<name>/` without modifying past evidence.

---

## 2. Mandatory Stop Conditions

An engineer or agent must **STOP execution immediately** and request human guidance rather than broadening scope or speculating when any of the following occur:

1. **Product Repo Boundary Violation**: Any task that would require editing, creating, or deleting files in `AI-Auto-Video-Creator` while under `INV-001`.
2. **Candidate A Architectural Invalidation**: Live empirical evidence proves Candidate A cannot function (e.g. unresolvable tool translation failure), requiring evaluation of dormant fallbacks (Candidate B/C).
3. **Unavoidable Upstream Patch**: A problem cannot be solved via Path A (UI / daemon REST) or the existing minimal CLI patch, requiring a new or larger patch to upstream Agent Orchestrator.
4. **Provider Policy / API Contradiction**: Provider API behavior directly contradicts the selected Responses wire format or model catalog semantics.
5. **Roadmap Alteration**: The proposed work requires inserting, deleting, or reordering phases in [04-ROADMAP.md](04-ROADMAP.md).
6. **New Dependency / Harness Proposal**: A proposal to introduce a new agent harness, translation proxy, or external CLI tool.
7. **Authoritative Conflict**: Two authoritative documents in `/docs` appear to disagree on an invariant or acceptance criterion.

---

## 3. Documentation Rules

### The Single-Home Rule
Every technical fact, schema, gate definition, and invariant has **exactly one authoritative home** in `/docs`. Other files (including root summaries and scripts) link to that home. They must never independently copy, rephrase, or maintain duplicate parallel specifications.

### The Deletion & Consolidation Rule
If two files are found to maintain the same technical truth:
1. Select the single authoritative file defined by the Authority Model.
2. Update that file to be complete and accurate.
3. Replace the duplicate section in the secondary file with a direct markdown link to the authoritative file.

### Prohibition of Documentation Accumulation
Never create incremental snapshot files such as:
- `audit-2.md`, `audit-final.md`, `audit-v3.md`
- `architecture-v2.md`, `architecture-final.md`
- `roadmap-new.md`, `plan-revised.md`
- `notes-fix.md`, `summary-latest.md`

All architectural updates are committed directly in-place to the canonical documents in `/docs`. Historical evidence belongs strictly under `/evidence/` or in Git version history.
