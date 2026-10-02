-- Where a drive or a charge comes from, and the identifier its source gave it. The TeslaMate columns stay until the
-- sources move to their own table; origin and external_id are the generic pair every source writes.
ALTER TABLE drives ADD COLUMN IF NOT EXISTS origin TEXT NOT NULL DEFAULT 'MANUAL';
ALTER TABLE drives ADD COLUMN IF NOT EXISTS external_id TEXT;
ALTER TABLE drives DROP CONSTRAINT IF EXISTS chk_drives_origin;
ALTER TABLE drives ADD CONSTRAINT chk_drives_origin CHECK (origin IN ('TESLAMATE', 'WEBHOOK', 'CSV', 'MANUAL'));

UPDATE drives SET origin = 'TESLAMATE', external_id = teslamate_drive_id::text
WHERE teslamate_drive_id IS NOT NULL AND external_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_drives_origin_external_id
    ON drives (vehicle_id, origin, external_id) WHERE external_id IS NOT NULL;

ALTER TABLE charge_logs ADD COLUMN IF NOT EXISTS origin TEXT NOT NULL DEFAULT 'MANUAL';
ALTER TABLE charge_logs DROP CONSTRAINT IF EXISTS chk_charge_logs_origin;
ALTER TABLE charge_logs ADD CONSTRAINT chk_charge_logs_origin CHECK (origin IN ('TESLAMATE', 'WEBHOOK', 'CSV', 'MANUAL'));

UPDATE charge_logs SET origin = 'TESLAMATE' WHERE teslamate_charge_id IS NOT NULL AND origin = 'MANUAL';
UPDATE charge_logs SET external_id = teslamate_charge_id::text
WHERE teslamate_charge_id IS NOT NULL AND external_id IS NULL;
UPDATE charge_logs SET origin = 'WEBHOOK' WHERE teslamate_charge_id IS NULL AND external_id IS NOT NULL AND origin = 'MANUAL';

-- Replaces uq_charge_logs_external_id, which ignored the origin.
CREATE UNIQUE INDEX IF NOT EXISTS uq_charge_logs_origin_external_id
    ON charge_logs (vehicle_id, origin, external_id) WHERE external_id IS NOT NULL;
DROP INDEX IF EXISTS uq_charge_logs_external_id;
