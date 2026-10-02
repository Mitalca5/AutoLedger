-- A CSV column mapping a user saved to reuse on files with the same headers.
CREATE TABLE import_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(60) NOT NULL,
    import_type VARCHAR(10) NOT NULL CHECK (import_type IN ('CHARGES', 'DRIVES', 'FUEL', 'ODOMETER')),
    -- Header text -> field the column feeds; an empty field ignores the column.
    columns JSONB NOT NULL DEFAULT '{}'::jsonb,
    date_order VARCHAR(3) NOT NULL DEFAULT '' CHECK (date_order IN ('', 'dmy', 'mdy', 'ymd')),
    decimal_separator VARCHAR(1) NOT NULL DEFAULT '' CHECK (decimal_separator IN ('', '.', ',')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, name)
);
