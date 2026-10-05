-- The maintenance record a reminder follows from: its last service date and odometer come from it.
ALTER TABLE maintenance_reminders
    ADD COLUMN IF NOT EXISTS maintenance_id UUID REFERENCES maintenance_expenses(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_maintenance_reminders_maintenance ON maintenance_reminders (maintenance_id) WHERE maintenance_id IS NOT NULL;
