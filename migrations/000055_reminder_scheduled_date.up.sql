-- A reminder can fall on a fixed calendar date (e.g. the 1st of November for the winter tires), once or every year.
ALTER TABLE maintenance_reminders
    ADD COLUMN IF NOT EXISTS scheduled_date DATE,
    ADD COLUMN IF NOT EXISTS repeat_yearly BOOLEAN NOT NULL DEFAULT FALSE;
