-- The TeslaMate connection lives in vehicle_data_sources only. Until now the vehicle columns were
-- written alongside the source, so they are the reference if the previous version wrote last:
-- bring the sources back in line with them, then drop the columns.
DELETE FROM vehicle_data_sources s
USING vehicles v
WHERE s.vehicle_id = v.id AND s.provider = 'TESLAMATE'
  AND COALESCE(v.teslamate_api_url, '') = ''
  AND COALESCE(v.teslamate_grafana_url, '') = ''
  AND v.teslamate_car_id IS NULL
  AND COALESCE(v.teslamate_basic_user, '') = ''
  AND v.teslamate_api_key_encrypted IS NULL
  AND v.teslamate_basic_pass_encrypted IS NULL
  AND v.teslamate_auth_type = 'NONE';

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
   OR teslamate_auth_type <> 'NONE'
ON CONFLICT (vehicle_id) WHERE provider = 'TESLAMATE' DO UPDATE
SET config = EXCLUDED.config, secret_encrypted = EXCLUDED.secret_encrypted,
    basic_secret_encrypted = EXCLUDED.basic_secret_encrypted, updated_at = NOW();

ALTER TABLE vehicles
    DROP COLUMN teslamate_car_id,
    DROP COLUMN teslamate_api_url,
    DROP COLUMN teslamate_auth_type,
    DROP COLUMN teslamate_api_key_encrypted,
    DROP COLUMN teslamate_basic_user,
    DROP COLUMN teslamate_basic_pass_encrypted,
    DROP COLUMN teslamate_grafana_url;
