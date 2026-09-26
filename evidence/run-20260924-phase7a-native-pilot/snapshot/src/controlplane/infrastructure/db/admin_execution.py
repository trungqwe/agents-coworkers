"""Authoritative P8 execution state on the caller's transaction connection."""

from __future__ import annotations

from typing import Any

from controlplane.domain.events import DomainEvent
from controlplane.infrastructure.db.outbox.postgres import PostgresOutboxRepository
from controlplane.infrastructure.db.projections.postgres import PostgresEventProcessor


class _CheckpointProjection:
    """P8 source state is the projection; this adapter only tracks applied revisions."""

    def __init__(self, connection: Any) -> None:
        self._connection = connection

    def current_revision(self, consumer_id: str, aggregate_id: str) -> int | None:
        row = self._connection.execute(
            "SELECT max(current_applied_revision) FROM controlplane.cp_event_checkpoints "
            "WHERE consumer_id=%s AND aggregate_id=%s AND status='APPLIED'",
            (consumer_id, aggregate_id),
        ).fetchone()
        return row[0]

    def apply(self, consumer_id: str, aggregate_id: str, revision: int) -> None:
        # The source CAS was already executed on this same transaction connection.
        return None


class PostgresInitialExecution:
    def __init__(self, connection: Any) -> None:
        self._connection = connection

    def create(self, *, workspace_id: str, operation_id: str, receipt_id: object,
               batch_id: str, actor_id: str) -> None:
        batch = self._connection.execute(
            "UPDATE controlplane.cp_production_batches SET revision=1,updated_at=CURRENT_TIMESTAMP "
            "WHERE workspace_id=%s AND batch_id=%s AND status='CREATED' AND revision IS NULL "
            "RETURNING batch_id",
            (workspace_id, batch_id),
        ).fetchone()
        if batch is None:
            raise RuntimeError("initial batch revision could not be assigned")
        self._connection.execute(
            "INSERT INTO controlplane.cp_operation_executions "
            "(workspace_id,operation_id,receipt_id,batch_id,status,revision,actor_ref) "
            "VALUES (%s,%s,%s,%s,'PREPARED',0,%s)",
            (workspace_id, operation_id, receipt_id, batch_id, actor_id),
        )


class PostgresAdminExecution:
    def __init__(self, connection: Any) -> None:
        self._connection = connection

    def load(self, *, workspace_id: str, operation_id: str) -> dict[str, object] | None:
        row = self._connection.execute(
            "SELECT e.receipt_id::text,e.batch_id,e.status,e.revision,e.actor_ref,"
            "e.evidence_kind,e.evidence_ref,e.run_id,b.status,b.revision,"
            "r.operation_id::text,r.resource_ref "
            "FROM controlplane.cp_operation_executions e "
            "JOIN controlplane.cp_production_batches b "
            "ON b.workspace_id=e.workspace_id AND b.batch_id=e.batch_id "
            "JOIN controlplane.cp_command_receipts r "
            "ON r.workspace_id=e.workspace_id AND r.receipt_id=e.receipt_id "
            "WHERE e.workspace_id=%s AND e.operation_id=%s "
            "FOR UPDATE OF e,b",
            (workspace_id, operation_id),
        ).fetchone()
        if row is None:
            return None
        return dict(zip(
            ("receipt_id","batch_id","status","revision","actor_id","evidence_kind",
             "evidence_ref","run_id","batch_status","batch_revision","receipt_operation_id",
             "resource_ref"), row,
        ))

    def accepted_applied(self, *, workspace_id: str, operation_id: str,
                         receipt_id: str, batch_id: str) -> bool:
        row = self._connection.execute(
            "SELECT EXISTS (SELECT 1 FROM controlplane.cp_outbox_events o "
            "JOIN controlplane.cp_event_checkpoints c ON c.event_id=o.event_id "
            "AND c.workspace_id=o.workspace_id "
            "WHERE o.workspace_id=%s AND o.event_name='production_batch.start_accepted' "
            "AND o.aggregate_id=%s AND o.payload->>'operation_id'=%s "
            "AND o.payload->>'receipt_id'=%s "
            "AND c.consumer_id=%s AND c.status='APPLIED')",
            (workspace_id, batch_id, operation_id, receipt_id,
             f"p8-admin-execution:{workspace_id}:production_batch"),
        ).fetchone()
        return bool(row[0])

    def apply_accepted(self, *, workspace_id: str, operation_id: str,
                       receipt_id: str, batch_id: str) -> str:
        row = self._connection.execute(
            "SELECT event_id::text,contract_name,contract_version,message_id,"
            "workspace_id::text,correlation_id,causation_id,trace_context,occurred_at,"
            "actor,recovery_epoch,payload,event_name,aggregate_type,aggregate_id,"
            "aggregate_revision,producer,schema_version,sensitivity "
            "FROM controlplane.cp_outbox_events "
            "WHERE workspace_id=%s AND event_name='production_batch.start_accepted' "
            "AND aggregate_id=%s AND payload->>'operation_id'=%s "
            "AND payload->>'receipt_id'=%s",
            (workspace_id, batch_id, operation_id, receipt_id),
        ).fetchone()
        if row is None:
            raise ValueError("accepted event does not match receipt and batch")
        event = DomainEvent(
            event_id=row[0], contract_name=row[1], contract_version=row[2],
            message_id=row[3], workspace_id=row[4], correlation_id=row[5],
            causation_id=row[6], trace_context=row[7], occurred_at=row[8].isoformat(),
            actor=row[9], recovery_epoch=row[10], payload=row[11],
            event_name=row[12], aggregate_type=row[13], aggregate_id=row[14],
            aggregate_revision=row[15], producer=row[16], schema_version=row[17],
            sensitivity=row[18],
        )
        return PostgresEventProcessor(
            self._connection, _CheckpointProjection(self._connection),
        ).process(event=event, consumer_id=f"p8-admin-execution:{workspace_id}:production_batch")

    def claim(self, *, workspace_id: str, operation_id: str, expected_revision: int,
              actor_id: str, run_id: str, evidence_ref: str) -> None:
        row = self._connection.execute(
            "UPDATE controlplane.cp_operation_executions "
            "SET status='STARTED',revision=1,evidence_kind='claim',evidence_ref=%s,"
            "run_id=%s,updated_at=CURRENT_TIMESTAMP "
            "WHERE workspace_id=%s AND operation_id=%s AND status='PREPARED' "
            "AND revision=%s AND actor_ref=%s RETURNING operation_id",
            (evidence_ref,run_id,workspace_id,operation_id,expected_revision,actor_id),
        ).fetchone()
        if row is None:
            raise RuntimeError("claim CAS failed")

    def confirm(self, *, workspace_id: str, operation_id: str, batch_id: str,
                expected_revision: int, actor_id: str, run_id: str,
                evidence_ref: str) -> None:
        batch = self._connection.execute(
            "UPDATE controlplane.cp_production_batches "
            "SET status='RUNNING',revision=2,updated_at=CURRENT_TIMESTAMP "
            "WHERE workspace_id=%s AND batch_id=%s AND status='CREATED' AND revision=1 "
            "RETURNING batch_id",
            (workspace_id,batch_id),
        ).fetchone()
        if batch is None:
            raise RuntimeError("batch activation CAS failed")
        operation = self._connection.execute(
            "UPDATE controlplane.cp_operation_executions "
            "SET status='SUCCEEDED',revision=2,evidence_kind='activation',"
            "evidence_ref=%s,updated_at=CURRENT_TIMESTAMP "
            "WHERE workspace_id=%s AND operation_id=%s AND batch_id=%s "
            "AND status='STARTED' AND revision=%s AND actor_ref=%s AND run_id=%s "
            "RETURNING operation_id",
            (evidence_ref,workspace_id,operation_id,batch_id,expected_revision,
             actor_id,run_id),
        ).fetchone()
        if operation is None:
            raise RuntimeError("confirm CAS failed")

    def publish(self, *, event: Any) -> None:
        # Reuse the accepted P3/P7B writer and cursor ordering fence.
        PostgresOutboxRepository(self._connection).enqueue(event)
        result = PostgresEventProcessor(
            self._connection, _CheckpointProjection(self._connection),
        ).process(
            event=event,
            consumer_id=f"p8-admin-execution:{event.workspace_id}:{event.aggregate_type}",
        )
        if result != "applied":
            raise RuntimeError(f"P8 event was not applied: {result}")
