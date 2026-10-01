-- The telemetry mode is derived from the vehicle's configuration: CONNECTED for an electric vehicle
-- with a teslamateapi URL (the condition the synchronization uses), MANUAL otherwise.
UPDATE vehicles
SET telemetry_mode = CASE
    WHEN powertrain <> 'ICE' AND LENGTH(TRIM(COALESCE(teslamate_api_url, ''))) > 0 THEN 'CONNECTED'
    ELSE 'MANUAL'
END
WHERE id IS NOT NULL;

-- Make and model are free text: no placeholder value when none was given.
ALTER TABLE vehicles ALTER COLUMN make SET DEFAULT '';
UPDATE vehicles SET make = '' WHERE make = 'Generic';
