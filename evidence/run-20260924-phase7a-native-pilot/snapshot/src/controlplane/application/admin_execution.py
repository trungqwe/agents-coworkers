"""P8-START-001 application boundary for test-driver batch activation."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any, Callable
import uuid

from controlplane.domain.concurrency import RevisionConflictError
from controlplane.domain.events import DomainEvent


@dataclass(frozen=True)
class StartEvidence:
    workspace_id: str
    operation_id: str
    receipt_id: str
    batch_id: str
    actor_id: str
    run_id: str
    evidence_ref: str


class AdminExecutionService:
    def __init__(self, *, uow_factory: Callable[[], Any], repository_factory: Callable[[Any], Any]) -> None:
        self._uow_factory = uow_factory
        self._repository_factory = repository_factory

    def apply_start_acceptance(self, *, evidence: StartEvidence) -> str:
        self._validate_evidence(evidence)
        with self._uow_factory() as uow:
            repository = self._repository_factory(uow.connection)
            self._bound_state(repository, evidence)
            return repository.apply_accepted(
                workspace_id=evidence.workspace_id, operation_id=evidence.operation_id,
                receipt_id=evidence.receipt_id, batch_id=evidence.batch_id,
            )

    def claim_start(self, *, evidence: StartEvidence, expected_revision: int) -> dict[str, object]:
        self._validate_evidence(evidence)
        with self._uow_factory() as uow:
            repository = self._repository_factory(uow.connection)
            state = self._bound_state(repository, evidence)
            if (state["status"] == "STARTED" and expected_revision == 0
                    and state["run_id"] == evidence.run_id
                    and state["evidence_kind"] == "claim"
                    and state["evidence_ref"] == evidence.evidence_ref):
                return {"operation_id": evidence.operation_id, "status": "STARTED", "revision": 1}
            if state["revision"] != expected_revision:
                raise RevisionConflictError(state["revision"])
            if not repository.accepted_applied(
                    workspace_id=evidence.workspace_id,
                    operation_id=evidence.operation_id,
                    receipt_id=evidence.receipt_id, batch_id=evidence.batch_id):
                raise ValueError("accepted event checkpoint required before claim")
            if state["status"] != "PREPARED" or state["batch_status"] != "CREATED":
                raise ValueError("invalid claim transition")
            if state["batch_revision"] != 1:
                raise ValueError("legacy batch requires reconciliation")
            repository.claim(
                workspace_id=evidence.workspace_id, operation_id=evidence.operation_id,
                expected_revision=expected_revision, actor_id=evidence.actor_id,
                run_id=evidence.run_id, evidence_ref=evidence.evidence_ref,
            )
            repository.publish(event=self._event(
                evidence, kind="operation.start_claimed", aggregate_type="operation",
                aggregate_id=evidence.operation_id, revision=1,
            ))
            return {"operation_id": evidence.operation_id, "status": "STARTED", "revision": 1}

    def confirm_batch_running(self, *, evidence: StartEvidence,
                              expected_revision: int) -> dict[str, object]:
        self._validate_evidence(evidence)
        with self._uow_factory() as uow:
            repository = self._repository_factory(uow.connection)
            state = self._bound_state(repository, evidence)
            if (state["status"] == "SUCCEEDED" and expected_revision == 1
                    and state["run_id"] == evidence.run_id
                    and state["evidence_kind"] == "activation"
                    and state["evidence_ref"] == evidence.evidence_ref
                    and state["batch_status"] == "RUNNING" and state["batch_revision"] == 2):
                return {"operation_id": evidence.operation_id, "status": "SUCCEEDED",
                        "revision": 2, "batch_id": evidence.batch_id,
                        "batch_status": "RUNNING", "batch_revision": 2}
            if state["revision"] != expected_revision:
                raise RevisionConflictError(state["revision"])
            if state["status"] != "STARTED" or state["batch_status"] != "CREATED":
                raise ValueError("invalid confirm transition")
            if state["batch_revision"] != 1 or state["run_id"] != evidence.run_id:
                raise ValueError("activation evidence is not bound to claim")
            repository.confirm(
                workspace_id=evidence.workspace_id, operation_id=evidence.operation_id,
                batch_id=evidence.batch_id, expected_revision=expected_revision,
                actor_id=evidence.actor_id, run_id=evidence.run_id,
                evidence_ref=evidence.evidence_ref,
            )
            repository.publish(event=self._event(
                evidence, kind="production_batch.start_activated",
                aggregate_type="production_batch", aggregate_id=evidence.batch_id,
                revision=2,
            ))
            repository.publish(event=self._event(
                evidence, kind="operation.start_succeeded", aggregate_type="operation",
                aggregate_id=evidence.operation_id, revision=2,
            ))
            return {"operation_id": evidence.operation_id, "status": "SUCCEEDED",
                    "revision": 2, "batch_id": evidence.batch_id, "batch_status": "RUNNING",
                    "batch_revision": 2}

    @staticmethod
    def _validate_evidence(evidence: StartEvidence) -> None:
        if not all((evidence.workspace_id, evidence.operation_id, evidence.receipt_id,
                    evidence.batch_id, evidence.actor_id, evidence.run_id,
                    evidence.evidence_ref)):
            raise ValueError("incomplete start evidence")

    @staticmethod
    def _bound_state(repository: Any, evidence: StartEvidence) -> dict[str, object]:
        state = repository.load(
            workspace_id=evidence.workspace_id, operation_id=evidence.operation_id,
        )
        if state is None:
            raise LookupError("operation not found")
        resource_ref = state["resource_ref"]
        if (state["receipt_id"] != evidence.receipt_id
                or state["batch_id"] != evidence.batch_id
                or state["actor_id"] != evidence.actor_id
                or state["receipt_operation_id"] != evidence.operation_id
                or not isinstance(resource_ref, dict)
                or resource_ref.get("batch_id") != evidence.batch_id):
            raise ValueError("evidence does not match receipt, actor, or batch")
        return state

    @staticmethod
    def _event(evidence: StartEvidence, *, kind: str, aggregate_type: str,
               aggregate_id: str, revision: int) -> DomainEvent:
        event_id = str(uuid.uuid4())
        return DomainEvent(
            contract_name="controlplane.p8.admin_execution", contract_version=1,
            message_id=event_id, workspace_id=evidence.workspace_id,
            correlation_id=evidence.run_id, causation_id=evidence.receipt_id,
            trace_context=None,
            occurred_at=datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
            actor=evidence.actor_id, recovery_epoch=None,
            payload={"operation_id": evidence.operation_id, "batch_id": evidence.batch_id,
                     "receipt_id": evidence.receipt_id, "evidence_ref": evidence.evidence_ref},
            event_id=event_id, event_name=kind, aggregate_type=aggregate_type,
            aggregate_id=aggregate_id, aggregate_revision=revision,
            producer="controlplane.admin_execution", schema_version=1,
            sensitivity="internal",
        )
