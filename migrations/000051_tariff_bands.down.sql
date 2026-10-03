UPDATE tariff_plans SET plan_type = 'TIME_OF_USE' WHERE plan_type = 'BANDS';

ALTER TABLE tariff_plans
    DROP COLUMN IF EXISTS valid_to,
    DROP COLUMN IF EXISTS valid_from,
    DROP COLUMN IF EXISTS standing_charge_cents,
    DROP COLUMN IF EXISTS default_band,
    DROP COLUMN IF EXISTS rules,
    DROP COLUMN IF EXISTS bands;
