-- TeslaMate drives were stored with TeslaMate's own "consumption_net", which is in Wh per km, in a column
-- that holds kWh per 100 km: every measured consumption read ten times too high (14.5 shown as 145).
-- Recompute it from the stored energy and distance, as the synchronization does now.
UPDATE drives
SET consumption_kwh_100km = ROUND((energy_consumed_kwh / distance_km * 100)::numeric, 2)
WHERE origin = 'TESLAMATE'
  AND NOT energy_estimated
  AND energy_consumed_kwh > 0
  AND distance_km > 0;
