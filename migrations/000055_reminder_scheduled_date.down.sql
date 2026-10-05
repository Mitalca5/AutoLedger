ALTER TABLE maintenance_reminders
    DROP COLUMN IF EXISTS repeat_yearly,
    DROP COLUMN IF EXISTS scheduled_date;
