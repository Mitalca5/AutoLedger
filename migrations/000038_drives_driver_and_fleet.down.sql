-- ============================================================================
-- AutoLedger Drives Driver & Fleet Migration (Down)
-- ============================================================================

DROP INDEX IF EXISTS idx_drives_driver;

ALTER TABLE vehicles
    DROP COLUMN IF EXISTS default_driver_id;

ALTER TABLE drives
    DROP COLUMN IF EXISTS driver_id;
