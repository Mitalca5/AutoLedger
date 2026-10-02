DROP INDEX IF EXISTS uq_odometer_checkpoints_ha_day;
ALTER TABLE odometer_checkpoints DROP COLUMN IF EXISTS source;
