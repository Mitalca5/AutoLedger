CREATE UNIQUE INDEX IF NOT EXISTS uq_charge_logs_external_id
    ON charge_logs (vehicle_id, external_id) WHERE external_id IS NOT NULL AND origin <> 'TESLAMATE';
DROP INDEX IF EXISTS uq_charge_logs_origin_external_id;
ALTER TABLE charge_logs DROP CONSTRAINT IF EXISTS chk_charge_logs_origin;
ALTER TABLE charge_logs DROP COLUMN IF EXISTS origin;
DROP INDEX IF EXISTS uq_drives_origin_external_id;
ALTER TABLE drives DROP CONSTRAINT IF EXISTS chk_drives_origin;
ALTER TABLE drives DROP COLUMN IF EXISTS external_id;
ALTER TABLE drives DROP COLUMN IF EXISTS origin;
