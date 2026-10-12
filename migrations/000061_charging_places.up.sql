-- Places where an account charges, with a category. The text a charge reports as its location (a geofence name,
-- an address, a Home Assistant tag) is linked to a place through an alias, so one classification covers every
-- charge reporting that text, past and future. A single charge can also point at a place directly.
CREATE TABLE charging_places (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    category VARCHAR(10) NOT NULL CHECK (category IN ('HOME', 'WORK', 'PUBLIC', 'OTHER')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_charging_places_user_name ON charging_places (user_id, lower(name));

-- alias is the reported text, lowercased with its whitespace collapsed
CREATE TABLE charging_place_aliases (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    alias TEXT NOT NULL,
    place_id UUID NOT NULL REFERENCES charging_places(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, alias)
);
CREATE INDEX idx_charging_place_aliases_place ON charging_place_aliases (place_id);

ALTER TABLE charge_logs ADD COLUMN place_id UUID REFERENCES charging_places(id) ON DELETE SET NULL;
CREATE INDEX idx_charge_logs_place ON charge_logs (place_id) WHERE place_id IS NOT NULL;
