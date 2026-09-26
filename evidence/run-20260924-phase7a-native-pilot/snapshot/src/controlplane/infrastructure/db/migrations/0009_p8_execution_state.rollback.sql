DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM controlplane.cp_operation_executions)
       OR EXISTS (SELECT 1 FROM controlplane.cp_production_batches WHERE revision IS NOT NULL)
    THEN
        RAISE EXCEPTION 'P8_EXECUTION_STATE_ROLLBACK_REQUIRES_AUDITED_EXPORT';
    END IF;
END $$;

DROP TABLE controlplane.cp_operation_executions;
ALTER TABLE controlplane.cp_production_batches
    DROP COLUMN updated_at,
    DROP COLUMN revision;
