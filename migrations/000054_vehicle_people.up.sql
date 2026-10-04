-- ============================================================================
-- AutoLedger Vehicle People Migration (Up)
-- A drive's driver is a person of the vehicle, who may or may not have an account.
-- ============================================================================

CREATE TABLE IF NOT EXISTS vehicle_people (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_vehicle_people_user ON vehicle_people(vehicle_id, user_id) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_vehicle_people_vehicle ON vehicle_people(vehicle_id);

-- One person per member of each vehicle
INSERT INTO vehicle_people (vehicle_id, name, user_id)
SELECT vm.vehicle_id, COALESCE(NULLIF(BTRIM(u.display_name), ''), u.email), vm.user_id
FROM vehicle_members vm
JOIN users u ON u.id = vm.user_id;

-- Users who were set as driver without being members (or whose access was removed)
INSERT INTO vehicle_people (vehicle_id, name, user_id)
SELECT DISTINCT refs.vehicle_id, COALESCE(NULLIF(BTRIM(u.display_name), ''), u.email), CAST(NULL AS UUID)
FROM (
    SELECT vehicle_id, driver_id AS user_id FROM drives WHERE driver_id IS NOT NULL
    UNION
    SELECT id AS vehicle_id, default_driver_id AS user_id FROM vehicles WHERE default_driver_id IS NOT NULL
) refs
JOIN users u ON u.id = refs.user_id
WHERE NOT EXISTS (
    SELECT 1 FROM vehicle_people p WHERE p.vehicle_id = refs.vehicle_id AND p.user_id = refs.user_id
);

-- Repoint the driver columns from users to people
ALTER TABLE drives DROP CONSTRAINT IF EXISTS drives_driver_id_fkey;
ALTER TABLE vehicles DROP CONSTRAINT IF EXISTS vehicles_default_driver_id_fkey;

UPDATE drives d
SET driver_id = COALESCE(
    (SELECT p.id FROM vehicle_people p WHERE p.vehicle_id = d.vehicle_id AND p.user_id = d.driver_id),
    (SELECT p.id FROM vehicle_people p JOIN users u ON u.id = d.driver_id
       WHERE p.vehicle_id = d.vehicle_id AND p.user_id IS NULL AND p.name = COALESCE(NULLIF(BTRIM(u.display_name), ''), u.email) LIMIT 1)
)
WHERE d.driver_id IS NOT NULL;

UPDATE vehicles v
SET default_driver_id = COALESCE(
    (SELECT p.id FROM vehicle_people p WHERE p.vehicle_id = v.id AND p.user_id = v.default_driver_id),
    (SELECT p.id FROM vehicle_people p JOIN users u ON u.id = v.default_driver_id
       WHERE p.vehicle_id = v.id AND p.user_id IS NULL AND p.name = COALESCE(NULLIF(BTRIM(u.display_name), ''), u.email) LIMIT 1)
)
WHERE v.default_driver_id IS NOT NULL;

-- A vehicle without a default driver defaults to its owner
UPDATE vehicles v
SET default_driver_id = (SELECT p.id FROM vehicle_people p WHERE p.vehicle_id = v.id AND p.user_id = v.user_id)
WHERE v.default_driver_id IS NULL;

ALTER TABLE drives ADD CONSTRAINT drives_driver_id_fkey
    FOREIGN KEY (driver_id) REFERENCES vehicle_people(id) ON DELETE SET NULL;
ALTER TABLE vehicles ADD CONSTRAINT vehicles_default_driver_id_fkey
    FOREIGN KEY (default_driver_id) REFERENCES vehicle_people(id) ON DELETE SET NULL;
