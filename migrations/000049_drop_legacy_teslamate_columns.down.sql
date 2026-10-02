ALTER TABLE vehicles
    ADD COLUMN teslamate_car_id INT,
    ADD COLUMN teslamate_api_url VARCHAR(255),
    ADD COLUMN teslamate_auth_type auth_mode NOT NULL DEFAULT 'NONE',
    ADD COLUMN teslamate_api_key_encrypted TEXT,
    ADD COLUMN teslamate_basic_user VARCHAR(100),
    ADD COLUMN teslamate_basic_pass_encrypted TEXT,
    ADD COLUMN teslamate_grafana_url TEXT;

UPDATE vehicles v
SET teslamate_car_id = (s.config->>'car_id')::int,
    teslamate_api_url = s.config->>'api_url',
    teslamate_auth_type = COALESCE(NULLIF(s.config->>'auth_type', ''), 'NONE')::auth_mode,
    teslamate_api_key_encrypted = s.secret_encrypted,
    teslamate_basic_user = s.config->>'basic_user',
    teslamate_basic_pass_encrypted = s.basic_secret_encrypted,
    teslamate_grafana_url = s.config->>'grafana_url'
FROM vehicle_data_sources s
WHERE s.vehicle_id = v.id AND s.provider = 'TESLAMATE';

-- The constraint is per account: keep the first vehicle of a duplicated car id and clear the others.
UPDATE vehicles v SET teslamate_car_id = NULL
WHERE teslamate_car_id IS NOT NULL AND EXISTS (
    SELECT 1 FROM vehicles o
    WHERE o.user_id = v.user_id AND o.teslamate_car_id = v.teslamate_car_id AND o.id < v.id);

ALTER TABLE vehicles ADD CONSTRAINT uq_user_teslamate_car UNIQUE (user_id, teslamate_car_id);
