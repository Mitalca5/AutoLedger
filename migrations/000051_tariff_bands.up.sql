ALTER TABLE tariff_plans
    ADD COLUMN IF NOT EXISTS bands JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS rules JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS default_band VARCHAR(50) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS standing_charge_cents BIGINT,
    ADD COLUMN IF NOT EXISTS valid_from DATE,
    ADD COLUMN IF NOT EXISTS valid_to DATE;

-- Peak / off-peak plans become two bands and one rule per off-peak window; the peak band covers the rest.
UPDATE tariff_plans
SET plan_type = 'BANDS',
    default_band = 'PEAK',
    bands = jsonb_build_array(
        jsonb_build_object('name', 'PEAK', 'rate_cents', COALESCE(peak_rate_cents, 0) / 100.0),
        jsonb_build_object('name', 'OFFPEAK', 'rate_cents', COALESCE(offpeak_rate_cents, 0) / 100.0)
    ),
    rules = (
        SELECT COALESCE(jsonb_agg(jsonb_build_object(
            'start', w->>'start',
            'end', w->>'end',
            'band', CASE WHEN upper(w->>'kind') = 'OFFPEAK' THEN 'OFFPEAK' ELSE 'PEAK' END
        )), '[]'::jsonb)
        FROM jsonb_array_elements(tariff_plans.time_windows) AS w
    )
WHERE plan_type = 'TIME_OF_USE';
