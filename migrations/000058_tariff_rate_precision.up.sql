-- Per-kWh prices keep up to six decimals instead of being rounded to the cent; existing values are unchanged.
ALTER TABLE tariff_plans RENAME COLUMN flat_rate_cents TO flat_rate;
ALTER TABLE tariff_plans RENAME COLUMN peak_rate_cents TO peak_rate;
ALTER TABLE tariff_plans RENAME COLUMN offpeak_rate_cents TO offpeak_rate;
ALTER TABLE tariff_plans
    ALTER COLUMN flat_rate TYPE NUMERIC(12, 6) USING flat_rate / 100.0,
    ALTER COLUMN peak_rate TYPE NUMERIC(12, 6) USING peak_rate / 100.0,
    ALTER COLUMN offpeak_rate TYPE NUMERIC(12, 6) USING offpeak_rate / 100.0;

ALTER TABLE public_charging_presets RENAME COLUMN price_per_kwh_cents TO price_per_kwh;
ALTER TABLE public_charging_presets
    ALTER COLUMN price_per_kwh DROP DEFAULT,
    ALTER COLUMN price_per_kwh TYPE NUMERIC(12, 6) USING price_per_kwh / 100.0,
    ALTER COLUMN price_per_kwh SET DEFAULT 0;
