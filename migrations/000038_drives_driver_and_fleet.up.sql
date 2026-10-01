-- ============================================================================
-- AutoLedger Drives Driver & Fleet Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

ALTER TABLE drives
    ADD COLUMN IF NOT EXISTS driver_id UUID REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE vehicles
    ADD COLUMN IF NOT EXISTS default_driver_id UUID REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_drives_driver ON drives(driver_id);

-- Backfill default driver with vehicle owner
UPDATE vehicles
SET default_driver_id = user_id
WHERE default_driver_id IS NULL;
