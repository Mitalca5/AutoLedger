ALTER TABLE odometer_checkpoints
    ADD COLUMN IF NOT EXISTS source VARCHAR(16) NOT NULL DEFAULT 'MANUAL'
    CHECK (source IN ('MANUAL', 'HA'));

-- An integration reports its readings all day long: it keeps a single point per vehicle and day.
CREATE UNIQUE INDEX IF NOT EXISTS uq_odometer_checkpoints_ha_day
    ON odometer_checkpoints (vehicle_id, date) WHERE source = 'HA';
