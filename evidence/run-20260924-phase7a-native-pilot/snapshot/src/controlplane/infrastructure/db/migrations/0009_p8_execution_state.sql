ALTER TABLE controlplane.cp_production_batches
    ADD COLUMN revision BIGINT NULL CHECK (revision >= 1),
    ADD COLUMN updated_at TIMESTAMPTZ NULL;

CREATE TABLE controlplane.cp_operation_executions (
    workspace_id UUID NOT NULL,
    operation_id UUID NOT NULL,
    receipt_id UUID NOT NULL,
    batch_id TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('PREPARED', 'STARTED', 'SUCCEEDED')),
    revision BIGINT NOT NULL CHECK (revision >= 0),
    evidence_kind TEXT NULL,
    evidence_ref TEXT NULL,
    run_id TEXT NULL,
    actor_ref TEXT NOT NULL CHECK (btrim(actor_ref) <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (workspace_id, operation_id),
    UNIQUE (workspace_id, receipt_id),
    FOREIGN KEY (workspace_id, receipt_id)
        REFERENCES controlplane.cp_command_receipts(workspace_id, receipt_id) ON DELETE RESTRICT,
    FOREIGN KEY (workspace_id, batch_id)
        REFERENCES controlplane.cp_production_batches(workspace_id, batch_id) ON DELETE RESTRICT,
    CHECK (
        (status = 'PREPARED' AND revision = 0 AND evidence_kind IS NULL AND evidence_ref IS NULL
            AND run_id IS NULL)
        OR (status <> 'PREPARED' AND revision > 0 AND evidence_kind IS NOT NULL
            AND evidence_ref IS NOT NULL AND btrim(evidence_ref) <> ''
            AND run_id IS NOT NULL AND btrim(run_id) <> '')
    )
);

CREATE INDEX cp_operation_executions_workspace_created_idx
    ON controlplane.cp_operation_executions(workspace_id, created_at DESC, operation_id DESC);
