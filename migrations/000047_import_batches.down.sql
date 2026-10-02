ALTER TABLE odometer_checkpoints DROP COLUMN IF EXISTS source_batch_id;
ALTER TABLE fuel_logs DROP COLUMN IF EXISTS source_batch_id;
ALTER TABLE drives DROP COLUMN IF EXISTS source_batch_id;
ALTER TABLE charge_logs DROP COLUMN IF EXISTS source_batch_id;
DROP TABLE IF EXISTS import_batches;
