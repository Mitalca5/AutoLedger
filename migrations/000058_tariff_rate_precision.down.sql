ALTER TABLE public_charging_presets
    ALTER COLUMN price_per_kwh DROP DEFAULT,
    ALTER COLUMN price_per_kwh TYPE BIGINT USING ROUND(price_per_kwh * 100),
    ALTER COLUMN price_per_kwh SET DEFAULT 0;
ALTER TABLE public_charging_presets RENAME COLUMN price_per_kwh TO price_per_kwh_cents;

ALTER TABLE tariff_plans
    ALTER COLUMN flat_rate TYPE BIGINT USING ROUND(flat_rate * 100),
    ALTER COLUMN peak_rate TYPE BIGINT USING ROUND(peak_rate * 100),
    ALTER COLUMN offpeak_rate TYPE BIGINT USING ROUND(offpeak_rate * 100);
ALTER TABLE tariff_plans RENAME COLUMN flat_rate TO flat_rate_cents;
ALTER TABLE tariff_plans RENAME COLUMN peak_rate TO peak_rate_cents;
ALTER TABLE tariff_plans RENAME COLUMN offpeak_rate TO offpeak_rate_cents;
