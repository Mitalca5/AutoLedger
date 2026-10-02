-- A vehicle's TeslaMate connection becomes a data source of its own. The vehicle columns stay in
-- place and are still written alongside the source until they are dropped.
CREATE TABLE vehicle_data_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    provider VARCHAR(20) NOT NULL CHECK (provider IN ('TESLAMATE')),
    -- Non-secret settings: api_url, auth_type, car_id, grafana_url, basic_user.
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    -- Encrypted with the application key: the bearer token, and the basic-auth password.
    secret_encrypted TEXT,
    basic_secret_encrypted TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    last_used_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_vehicle_teslamate_source ON vehicle_data_sources (vehicle_id) WHERE provider = 'TESLAMATE';

INSERT INTO vehicle_data_sources (vehicle_id, provider, config, secret_encrypted, basic_secret_encrypted)
SELECT id, 'TESLAMATE',
       jsonb_strip_nulls(jsonb_build_object(
           'api_url', teslamate_api_url,
           'auth_type', teslamate_auth_type::text,
           'car_id', teslamate_car_id,
           'grafana_url', teslamate_grafana_url,
           'basic_user', teslamate_basic_user
       )),
       teslamate_api_key_encrypted,
       teslamate_basic_pass_encrypted
FROM vehicles
WHERE COALESCE(teslamate_api_url, '') <> ''
   OR COALESCE(teslamate_grafana_url, '') <> ''
   OR teslamate_car_id IS NOT NULL
   OR COALESCE(teslamate_basic_user, '') <> ''
   OR teslamate_api_key_encrypted IS NOT NULL
   OR teslamate_basic_pass_encrypted IS NOT NULL
   OR teslamate_auth_type <> 'NONE';
