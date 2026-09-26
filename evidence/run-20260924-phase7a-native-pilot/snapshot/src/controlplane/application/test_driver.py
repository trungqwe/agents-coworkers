"""Private fixture-only P8 driver; never registered as an API route."""

from __future__ import annotations

from typing import Any

from controlplane.application.admin_execution import AdminExecutionService, StartEvidence
from controlplane.infrastructure.db.admin_execution import PostgresAdminExecution


class P8TestDriver:
    def __init__(self, *, manager: Any, run_id: str) -> None:
        if not run_id:
            raise ValueError("run_id required")
        self._run_id = run_id
        self._execution = AdminExecutionService(
            uow_factory=manager.unit_of_work,
            repository_factory=PostgresAdminExecution,
        )

    def apply_start_acceptance(self, *, evidence: StartEvidence) -> str:
        if evidence.run_id != self._run_id:
            raise ValueError("evidence belongs to a different fixture run")
        return self._execution.apply_start_acceptance(evidence=evidence)

    def claim_start(self, *, evidence: StartEvidence, expected_revision: int) -> dict[str, object]:
        if evidence.run_id != self._run_id:
            raise ValueError("evidence belongs to a different fixture run")
        return self._execution.claim_start(evidence=evidence, expected_revision=expected_revision)

    def confirm_batch_running(self, *, evidence: StartEvidence,
                              expected_revision: int) -> dict[str, object]:
        if evidence.run_id != self._run_id:
            raise ValueError("evidence belongs to a different fixture run")
        return self._execution.confirm_batch_running(
            evidence=evidence, expected_revision=expected_revision,
        )

    @staticmethod
    def arm_one_shot_error(app: Any, error: BaseException) -> None:
        """Inject one pre-commit failure through the real HTTP command service."""
        original = app.state.command_service

        class OneShot:
            def start_batch(self, **kwargs: Any) -> dict[str, object]:
                app.state.command_service = original
                return original.start_batch(**kwargs, inject_before_commit=error)

            def __getattr__(self, name: str) -> Any:
                return getattr(original, name)

        app.state.command_service = OneShot()
