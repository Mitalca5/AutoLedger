-- A mileage allowance scale typed by the user: rate per distance for each slice of the year's cumulative distance.
CREATE TABLE IF NOT EXISTS mileage_rates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label VARCHAR(100) NOT NULL,
    year SMALLINT NOT NULL,
    from_km INTEGER NOT NULL DEFAULT 0 CHECK (from_km >= 0),
    to_km INTEGER CHECK (to_km IS NULL OR to_km > from_km),
    rate_per_km NUMERIC(10, 5) NOT NULL CHECK (rate_per_km >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mileage_rates_user ON mileage_rates (user_id, label, year);
