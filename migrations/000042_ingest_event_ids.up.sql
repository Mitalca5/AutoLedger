-- Identifier an integration gives an event (Home Assistant, scripts), so a resent event is recognised
-- instead of recorded twice, before and after a pending charge is assigned to a vehicle.
ALTER TABLE charge_logs ADD COLUMN IF NOT EXISTS external_id TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS uq_charge_logs_external_id
    ON charge_logs (vehicle_id, external_id) WHERE external_id IS NOT NULL;

ALTER TABLE pending_charges ADD COLUMN IF NOT EXISTS external_id TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS uq_pending_charges_external_id
    ON pending_charges (user_id, external_id) WHERE external_id IS NOT NULL;

-- A location is stored only when the event sends one.
ALTER TABLE pending_charges ALTER COLUMN location DROP DEFAULT;
