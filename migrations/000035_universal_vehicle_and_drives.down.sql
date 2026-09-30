-- ============================================================================
-- AutoLedger Universal Vehicles & Drives Migration (Down)
-- Database: PostgreSQL 14+
-- ============================================================================

DROP INDEX IF EXISTS idx_drives_manual;

ALTER TABLE vehicles
    DROP COLUMN IF EXISTS telemetry_mode,
    DROP COLUMN IF EXISTS make,
    DROP COLUMN IF EXISTS model;
