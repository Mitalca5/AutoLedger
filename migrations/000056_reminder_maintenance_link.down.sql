DROP INDEX IF EXISTS idx_maintenance_reminders_maintenance;
ALTER TABLE maintenance_reminders DROP COLUMN IF EXISTS maintenance_id;
