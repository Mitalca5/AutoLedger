-- ============================================================================
-- AutoLedger Universal Vehicles & Drives Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

-- Add telemetry tracking mode, make and model to vehicles
ALTER TABLE vehicles
    ADD COLUMN telemetry_mode TEXT NOT NULL DEFAULT 'MANUAL' CHECK (telemetry_mode IN ('CONNECTED', 'SEMI_AUTO', 'MANUAL')),
    ADD COLUMN make TEXT NOT NULL DEFAULT 'Generic',
    ADD COLUMN model TEXT NOT NULL DEFAULT '';

-- Backfill existing vehicles:
-- If connected to TeslaMate, mark as CONNECTED and Tesla make.
-- Otherwise, mark as MANUAL.
UPDATE vehicles
SET telemetry_mode = 'CONNECTED', make = 'Tesla'
WHERE teslamate_car_id IS NOT NULL;

UPDATE vehicles
SET telemetry_mode = 'MANUAL'
WHERE teslamate_car_id IS NULL;

-- Index manual drives for performance and duplicate detection
CREATE INDEX IF NOT EXISTS idx_drives_manual ON drives (vehicle_id, start_time) WHERE is_manual = TRUE;
