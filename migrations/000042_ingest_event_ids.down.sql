ALTER TABLE pending_charges ALTER COLUMN location SET DEFAULT 'home';
DROP INDEX IF EXISTS uq_pending_charges_external_id;
ALTER TABLE pending_charges DROP COLUMN IF EXISTS external_id;
DROP INDEX IF EXISTS uq_charge_logs_external_id;
ALTER TABLE charge_logs DROP COLUMN IF EXISTS external_id;
