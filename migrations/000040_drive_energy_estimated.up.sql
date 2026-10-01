-- Energy of a manual drive entered without a measured value is derived from the vehicle's average
-- consumption. Flag it so the energy statistics do not read it as a measurement.
ALTER TABLE drives ADD COLUMN IF NOT EXISTS energy_estimated BOOLEAN NOT NULL DEFAULT FALSE;

-- Manual drives recorded before this column: their consumption equals the value the server applied
-- (the vehicle's estimate, 16 kWh/100 km without one) when no energy was typed.
UPDATE drives d
SET energy_estimated = TRUE
FROM vehicles v
WHERE v.id = d.vehicle_id
  AND d.is_manual = TRUE
  AND d.consumption_kwh_100km IS NOT NULL
  AND ABS(d.consumption_kwh_100km - COALESCE(NULLIF(v.estimated_kwh_100km, 0), 16.0)) < 0.001;
