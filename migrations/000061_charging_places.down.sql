-- Refuse a downgrade while classifications exist rather than discard them silently.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM charging_places) THEN
        RAISE EXCEPTION 'charging places exist: delete them before downgrading';
    END IF;
END $$;

DROP INDEX IF EXISTS idx_charge_logs_place;
ALTER TABLE charge_logs DROP COLUMN IF EXISTS place_id;
DROP TABLE IF EXISTS charging_place_aliases;
DROP TABLE IF EXISTS charging_places;
