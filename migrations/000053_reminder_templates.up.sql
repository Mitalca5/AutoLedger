-- A set of reminders (title, intervals) the user saves once and applies to any vehicle.
CREATE TABLE IF NOT EXISTS reminder_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    items JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_reminder_templates_user ON reminder_templates (user_id);

-- Every completion of a reminder, to learn the intervals the user really follows.
CREATE TABLE IF NOT EXISTS reminder_completions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reminder_id UUID NOT NULL REFERENCES maintenance_reminders(id) ON DELETE CASCADE,
    vehicle_id UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    completed_on DATE NOT NULL,
    odometer DOUBLE PRECISION NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_reminder_completions_reminder ON reminder_completions (reminder_id, completed_on);
