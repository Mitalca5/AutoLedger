-- ============================================================================
-- AutoLedger Tariffs & Pricing Migration (Down)
-- ============================================================================

DROP TABLE IF EXISTS public_charging_presets;

ALTER TABLE vehicles
    DROP COLUMN IF EXISTS is_home_charger_default,
    DROP COLUMN IF EXISTS tariff_plan_id;

DROP TABLE IF EXISTS tariff_plans;
