"""Behavioral RED test for P8 execution state tracking on StartProductionBatch.

Verifies that StartProductionBatch command initializes operation execution state
with revision=0. In CP1, _operation_row does not provide revision and the execution
state table (migration 0009) is not yet implemented, causing this test to fail
with a genuine behavioral failure (assert op['revision'] == 0), rather than a
setup/import/schema error.
"""

from __future__ import annotations

from contextlib import contextmanager
import os
from pathlib import Path
import uuid

import psycopg
import pytest
from psycopg import sql
from psycopg.conninfo import make_conninfo
from starlette.testclient import TestClient

from controlplane.api.main import create_app
from controlplane.application.admin_execution import StartEvidence
from controlplane.application.test_driver import P8TestDriver
from controlplane.domain.concurrency import RevisionConflictError
from controlplane.application.session_security import BootstrapCapabilityRegistry
from controlplane.infrastructure.db.migration_runner import MigrationRunner
from controlplane.infrastructure.db.uow import TransactionManager

MIGRATIONS = Path(__file__).parents[2] / "src/controlplane/infrastructure/db/migrations"
WORKSPACE_ID = "00000000-0000-0000-0000-000000000801"
ACTOR_ID = "00000000-0000-0000-0000-000000000802"


@contextmanager
def disposable_database():
    admin_dsn = os.environ.get("M2_TEST_PG_DSN")
    if not admin_dsn:
        raise RuntimeError(
            "PREREQUISITE_FAILURE: M2_TEST_PG_DSN environment variable is not configured. "
            "PostgreSQL 18.6 administrative DSN required."
        )

    name = f"m2_p8_exec_{uuid.uuid4().hex}"
    with psycopg.connect(admin_dsn, autocommit=True) as admin:
        version = admin.execute("SHOW server_version").fetchone()[0].split()[0]
        if version != "18.6":
            raise RuntimeError(f"PREREQUISITE_FAILURE: PostgreSQL version must be 18.6, got {version}")
        has_createdb = admin.execute(
            "SELECT rolcreatedb FROM pg_roles WHERE rolname=current_user"
        ).fetchone()[0]
        if not has_createdb:
            raise RuntimeError("PREREQUISITE_FAILURE: User lacks rolcreatedb permission")
        admin.execute(sql.SQL("CREATE DATABASE {}").format(sql.Identifier(name)))

    dsn = make_conninfo(admin_dsn, dbname=name)
    try:
        MigrationRunner(dsn, MIGRATIONS, is_test_env=True).migrate_up()
        with psycopg.connect(dsn) as connection:
            connection.execute(
                "INSERT INTO controlplane.cp_workspaces(workspace_id, name, status) "
                "VALUES (%s, 'P8 Workspace', 'ACTIVE')",
                (WORKSPACE_ID,),
            )
            connection.execute(
                "INSERT INTO controlplane.cp_actors"
                "(actor_id, workspace_id, actor_type, display_name, status) "
                "VALUES (%s, %s, 'user', 'P8 Actor', 'ACTIVE')",
                (ACTOR_ID, WORKSPACE_ID),
            )
            connection.commit()
        yield dsn
    finally:
        with psycopg.connect(admin_dsn, autocommit=True) as admin:
            admin.execute(
                "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname=%s",
                (name,),
            )
            admin.execute(sql.SQL("DROP DATABASE {}").format(sql.Identifier(name)))


@contextmanager
def authenticated_app(dsn: str):
    manager = TransactionManager(dsn)
    registry = BootstrapCapabilityRegistry()
    app = create_app(
        manager=manager,
        allowed_origins={"https://localhost:8443"},
        bootstrap_registry=registry,
    )
    capability = registry.issue(workspace_id=WORKSPACE_ID, actor_id=ACTOR_ID)
    with TestClient(app, base_url="https://localhost:8443", raise_server_exceptions=False) as client:
        response = client.post(
            "/v1/session/bootstrap",
            headers={
                "Origin": "https://localhost:8443",
                "X-Launch-Capability": capability,
                "Cookie": "csrf_token=bootstrap-token",
                "X-CSRF-Token": "bootstrap-token",
            },
        )
        assert response.status_code == 200, response.text
        try:
            yield client, manager
        finally:
            manager.close()


def test_tst_m2_p8_004_start_batch_operation_execution_state_initial_revision():
    """CP1 Backend RED Oracle.

    Verifies that starting a production batch records an authoritative operation
    execution state starting at revision=0. Fails in CP1 because _operation_row does
    not provide revision=0 (missing behavior), not because of import or setup failure.
    """
    with disposable_database() as dsn, authenticated_app(dsn) as (client, _):
        headers = {
            "Origin": "https://localhost:8443",
            "X-CSRF-Token": client.cookies["csrf_token"],
            "Idempotency-Key": f"p8-start-{uuid.uuid4().hex}",
        }

        # 1. Issue StartProductionBatch command
        post_resp = client.post(
            "/v1/batches",
            json={"target_completed_videos": 5},
            headers=headers,
        )
        assert post_resp.status_code == 202, post_resp.text
        receipt = post_resp.json()
        assert "operation_id" in receipt, "Receipt must contain operation_id"
        operation_id = receipt["operation_id"]

        # 2. Query operation detail
        get_resp = client.get(f"/v1/operations/{operation_id}")
        assert get_resp.status_code == 200, get_resp.text
        operation = get_resp.json()

        # 3. Behavioral RED assertion:
        # P8-START-001 requires initial operation execution state PREPARED at revision=0.
        # Existing P7A queries return status='accepted' without revision field (or revision=None).
        # This assert must fail on missing behavior (revision == 0), proving Behavioral RED.
        assert "revision" in operation, "Operation detail must include revision field"
        assert operation["revision"] == 0, (
            f"Expected initial operation revision 0 for PREPARED state, got {operation.get('revision')}"
        )
        assert operation["status"] == "prepared"
        batch = client.get(f"/v1/batches/{receipt['resource_ref']['batch_id']}")
        assert batch.status_code == 200
        assert batch.json()["status"] == "CREATED"
        assert batch.json()["revision"] == 1


def test_tst_m2_p8_005_claim_and_confirm_start_command_transitions():
    with disposable_database() as dsn, authenticated_app(dsn) as (client, manager):
        headers = {
            "Origin": "https://localhost:8443",
            "X-CSRF-Token": client.cookies["csrf_token"],
            "Idempotency-Key": f"p8-start-{uuid.uuid4().hex}",
        }
        response = client.post("/v1/batches", json={"target_completed_videos": 2}, headers=headers)
        assert response.status_code == 202, response.text
        receipt = response.json()
        evidence = StartEvidence(
            workspace_id=WORKSPACE_ID,
            operation_id=receipt["operation_id"],
            receipt_id=receipt["receipt_id"],
            batch_id=receipt["resource_ref"]["batch_id"],
            actor_id=ACTOR_ID,
            run_id="cp2a-test-run",
            evidence_ref=f"claim-{uuid.uuid4()}",
        )
        service = P8TestDriver(manager=manager, run_id="cp2a-test-run")
        with pytest.raises(ValueError, match="accepted event checkpoint"):
            service.claim_start(evidence=evidence, expected_revision=0)
        service.apply_start_acceptance(evidence=evidence)
        claimed = service.claim_start(evidence=evidence, expected_revision=0)
        assert claimed["status"] == "STARTED"
        assert claimed["revision"] == 1
        running = client.get(f"/v1/operations/{evidence.operation_id}")
        assert running.status_code == 200
        assert running.json()["status"] == "started"
        assert running.json()["revision"] == 1
        activation = StartEvidence(
            workspace_id=evidence.workspace_id, operation_id=evidence.operation_id,
            receipt_id=evidence.receipt_id, batch_id=evidence.batch_id,
            actor_id=evidence.actor_id, run_id=evidence.run_id,
            evidence_ref=f"activation-{uuid.uuid4()}",
        )
        confirmed = service.confirm_batch_running(evidence=activation, expected_revision=1)
        assert service.confirm_batch_running(evidence=activation, expected_revision=1) == confirmed
        assert confirmed["status"] == "SUCCEEDED"
        assert confirmed["revision"] == 2
        operation = client.get(f"/v1/operations/{evidence.operation_id}")
        assert operation.json()["status"] == "succeeded"
        assert operation.json()["revision"] == 2
        batch = client.get(f"/v1/batches/{evidence.batch_id}")
        assert batch.status_code == 200
        assert batch.json()["status"] == "RUNNING"
        assert batch.json()["revision"] == 2
        with manager.borrow_connection() as connection:
            assert connection.execute(
                "SELECT count(*) FROM controlplane.cp_outbox_events WHERE workspace_id=%s",
                (WORKSPACE_ID,),
            ).fetchone()[0] == 4
            assert connection.execute(
                "SELECT count(*) FROM controlplane.cp_event_checkpoints "
                "WHERE workspace_id=%s AND consumer_id LIKE %s AND status='APPLIED'",
                (WORKSPACE_ID, f"p8-admin-execution:{WORKSPACE_ID}:%"),
            ).fetchone()[0] == 4
            assert connection.execute(
                "SELECT count(*) FROM controlplane.cp_operation_stream WHERE workspace_id=%s",
                (WORKSPACE_ID,),
            ).fetchone()[0] == 4


def test_tst_m2_p8_006_idempotency_rollback_and_rejected_evidence():
    with disposable_database() as dsn, authenticated_app(dsn) as (client, manager):
        headers = {
            "Origin": "https://localhost:8443",
            "X-CSRF-Token": client.cookies["csrf_token"],
            "Idempotency-Key": f"p8-start-{uuid.uuid4().hex}",
        }
        payload = {"target_completed_videos": 3}
        receipt = client.post("/v1/batches", json=payload, headers=headers).json()
        replay = client.post("/v1/batches", json=payload, headers=headers)
        assert replay.status_code == 202
        assert replay.json()["receipt_id"] == receipt["receipt_id"]
        with manager.borrow_connection() as connection:
            assert connection.execute(
                "SELECT count(*) FROM controlplane.cp_outbox_events WHERE workspace_id=%s",
                (WORKSPACE_ID,),
            ).fetchone()[0] == 1
            assert connection.execute(
                "SELECT count(*) FROM controlplane.cp_operation_stream WHERE workspace_id=%s",
                (WORKSPACE_ID,),
            ).fetchone()[0] == 0
        evidence = StartEvidence(
            workspace_id=WORKSPACE_ID, operation_id=receipt["operation_id"],
            receipt_id=receipt["receipt_id"],
            batch_id=receipt["resource_ref"]["batch_id"],
            actor_id=ACTOR_ID, run_id="cp2a-test-run",
            evidence_ref=f"claim-{uuid.uuid4()}",
        )
        driver = P8TestDriver(manager=manager, run_id=evidence.run_id)
        driver.apply_start_acceptance(evidence=evidence)
        with pytest.raises(ValueError):
            driver.claim_start(
                evidence=StartEvidence(**{**evidence.__dict__, "receipt_id": str(uuid.uuid4())}),
                expected_revision=0,
            )
        with pytest.raises(ValueError):
            driver.confirm_batch_running(evidence=evidence, expected_revision=0)
        with pytest.raises(ValueError):
            P8TestDriver(manager=manager, run_id="other-run").claim_start(
                evidence=evidence, expected_revision=0,
            )
        claimed = driver.claim_start(evidence=evidence, expected_revision=0)
        assert claimed["revision"] == 1
        assert driver.claim_start(evidence=evidence, expected_revision=0) == claimed
        with pytest.raises(RevisionConflictError):
            driver.claim_start(
                evidence=StartEvidence(**{**evidence.__dict__, "evidence_ref": "different-claim"}),
                expected_revision=0,
            )
        with pytest.raises(ValueError):
            driver.confirm_batch_running(
                evidence=StartEvidence(**{**evidence.__dict__, "run_id": "other-run"}),
                expected_revision=1,
            )
        with manager.borrow_connection() as connection:
            assert connection.execute(
                "SELECT count(*) FROM controlplane.cp_outbox_events WHERE workspace_id=%s",
                (WORKSPACE_ID,),
            ).fetchone()[0] == 2
            assert connection.execute(
                "SELECT count(*) FROM controlplane.cp_operation_stream WHERE workspace_id=%s",
                (WORKSPACE_ID,),
            ).fetchone()[0] == 2

def test_tst_m2_p8_007_legacy_workspace_and_rollback_guard():
    from psycopg.types.json import Jsonb

    with disposable_database() as dsn, authenticated_app(dsn) as (client, manager):
        other_workspace = str(uuid.uuid4())
        other_batch = str(uuid.uuid4())
        legacy_batch = str(uuid.uuid4())
        legacy_operation = str(uuid.uuid4())
        legacy_receipt = str(uuid.uuid4())
        with manager.unit_of_work() as uow:
            connection = uow.connection
            connection.execute(
                "INSERT INTO controlplane.cp_workspaces(workspace_id,name,status) "
                "VALUES (%s,'Other Workspace','ACTIVE')",
                (other_workspace,),
            )
            connection.execute(
                "INSERT INTO controlplane.cp_production_batches "
                "(workspace_id,batch_id,status,target_count) VALUES (%s,%s,'CREATED',1)",
                (WORKSPACE_ID, legacy_batch),
            )
            connection.execute(
                "INSERT INTO controlplane.cp_command_receipts "
                "(receipt_id,workspace_id,command_id,disposition,operation_id,resource_ref,accepted_at) "
                "VALUES (%s,%s,%s,'accepted',%s,%s,CURRENT_TIMESTAMP)",
                (legacy_receipt, WORKSPACE_ID, str(uuid.uuid4()), legacy_operation,
                 Jsonb({"kind": "production_batch", "batch_id": legacy_batch})),
            )
            connection.execute(
                "INSERT INTO controlplane.cp_production_batches "
                "(workspace_id,batch_id,status,target_count) VALUES (%s,%s,'CREATED',1)",
                (other_workspace, other_batch),
            )
        old = client.get(f"/v1/operations/{legacy_operation}")
        assert old.status_code == 200
        assert old.json()["status"] == "unknown_legacy"
        assert old.json()["revision"] is None
        batch = client.get(f"/v1/batches/{legacy_batch}")
        assert batch.json()["revision"] is None
        assert len(client.get("/v1/batches").json()["items"]) == 1
        assert client.get(f"/v1/batches/{other_batch}").status_code == 404
        rollback = (MIGRATIONS / "0009_p8_execution_state.rollback.sql").read_text(encoding="utf-8")
        with psycopg.connect(dsn, autocommit=True) as connection:
            # Legacy rows have no P8 revision and can be rolled back without fabricated state.
            connection.execute(rollback)
            assert connection.execute(
                "SELECT to_regclass('controlplane.cp_operation_executions')"
            ).fetchone()[0] is None


def test_tst_m2_p8_010_test_driver_arms_one_shot_safe_error():
    with disposable_database() as dsn, authenticated_app(dsn) as (client, manager):
        driver = P8TestDriver(manager=manager, run_id="cp2a-test-run")
        driver.arm_one_shot_error(client.app, RuntimeError("private fixture failure"))
        headers = {
            "Origin": "https://localhost:8443",
            "X-CSRF-Token": client.cookies["csrf_token"],
            "Idempotency-Key": f"p8-error-{uuid.uuid4().hex}",
        }
        response = client.post("/v1/batches", json={"target_completed_videos": 1}, headers=headers)
        assert response.status_code == 500
        body = response.json()
        assert body["code"] == "INTERNAL_ERROR"
        assert body["technical_detail_ref"]
        assert "private fixture failure" not in response.text
        with manager.borrow_connection() as connection:
            assert connection.execute(
                "SELECT count(*) FROM controlplane.cp_operation_executions WHERE workspace_id=%s",
                (WORKSPACE_ID,),
            ).fetchone()[0] == 0


def test_tst_m2_p8_009_initial_uow_rollback_preserves_no_state():
    with disposable_database() as dsn, authenticated_app(dsn) as (client, manager):
        with pytest.raises(RuntimeError, match="p8-injected-rollback"):
            client.app.state.command_service.start_batch(
                workspace_id=WORKSPACE_ID, actor_id=ACTOR_ID,
                correlation_id=str(uuid.uuid4()),
                idempotency_key=f"p8-rollback-{uuid.uuid4()}",
                payload={"target_completed_videos": 1},
                inject_before_commit=RuntimeError("p8-injected-rollback"),
            )
        with manager.borrow_connection() as connection:
            for table in ("cp_production_batches", "cp_operation_executions",
                          "cp_command_receipts", "cp_idempotency_records",
                          "cp_outbox_events", "cp_operation_stream"):
                assert connection.execute(
                    f"SELECT count(*) FROM controlplane.{table} WHERE workspace_id=%s",
                    (WORKSPACE_ID,),
                ).fetchone()[0] == 0


def test_tst_m2_p8_011_transition_crash_rolls_back_source_outbox_checkpoint_stream(monkeypatch):
    from controlplane.infrastructure.db.admin_execution import PostgresAdminExecution

    with disposable_database() as dsn, authenticated_app(dsn) as (client, manager):
        headers = {
            "Origin": "https://localhost:8443",
            "X-CSRF-Token": client.cookies["csrf_token"],
            "Idempotency-Key": f"p8-start-{uuid.uuid4().hex}",
        }
        receipt = client.post("/v1/batches", json={"target_completed_videos": 1},
                              headers=headers).json()
        evidence = StartEvidence(
            workspace_id=WORKSPACE_ID, operation_id=receipt["operation_id"],
            receipt_id=receipt["receipt_id"],
            batch_id=receipt["resource_ref"]["batch_id"],
            actor_id=ACTOR_ID, run_id="cp2a-test-run",
            evidence_ref=f"claim-{uuid.uuid4()}",
        )
        driver = P8TestDriver(manager=manager, run_id=evidence.run_id)
        assert driver.apply_start_acceptance(evidence=evidence) == "applied"
        original_publish = PostgresAdminExecution.publish

        def crash_after_projection(self, *, event):
            original_publish(self, event=event)
            raise RuntimeError("fixture crash before UoW commit")

        monkeypatch.setattr(PostgresAdminExecution, "publish", crash_after_projection)
        with pytest.raises(RuntimeError, match="fixture crash before UoW commit"):
            driver.claim_start(evidence=evidence, expected_revision=0)
        monkeypatch.setattr(PostgresAdminExecution, "publish", original_publish)
        assert client.get(f"/v1/operations/{evidence.operation_id}").json()["revision"] == 0
        with manager.borrow_connection() as connection:
            assert connection.execute(
                "SELECT count(*) FROM controlplane.cp_outbox_events WHERE workspace_id=%s",
                (WORKSPACE_ID,),
            ).fetchone()[0] == 1
            assert connection.execute(
                "SELECT count(*) FROM controlplane.cp_event_checkpoints "
                "WHERE workspace_id=%s AND consumer_id LIKE %s AND status='APPLIED'",
                (WORKSPACE_ID, f"p8-admin-execution:{WORKSPACE_ID}:%"),
            ).fetchone()[0] == 1
            assert connection.execute(
                "SELECT count(*) FROM controlplane.cp_operation_stream WHERE workspace_id=%s",
                (WORKSPACE_ID,),
            ).fetchone()[0] == 1
        assert driver.claim_start(evidence=evidence, expected_revision=0)["revision"] == 1


def test_tst_m2_p8_008_rollback_rejects_active_execution_state():
    with disposable_database() as dsn, authenticated_app(dsn) as (client, _):
        headers = {
            "Origin": "https://localhost:8443",
            "X-CSRF-Token": client.cookies["csrf_token"],
            "Idempotency-Key": f"p8-start-{uuid.uuid4().hex}",
        }
        assert client.post("/v1/batches", json={"target_completed_videos": 1},
                           headers=headers).status_code == 202
        rollback = (MIGRATIONS / "0009_p8_execution_state.rollback.sql").read_text(encoding="utf-8")
        with psycopg.connect(dsn, autocommit=True) as connection:
            with pytest.raises(psycopg.errors.RaiseException,
                               match="P8_EXECUTION_STATE_ROLLBACK_REQUIRES_AUDITED_EXPORT"):
                connection.execute(rollback)
            assert connection.execute(
                "SELECT to_regclass('controlplane.cp_operation_executions')"
            ).fetchone()[0] is not None
