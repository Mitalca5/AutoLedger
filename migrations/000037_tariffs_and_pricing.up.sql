-- ============================================================================
-- AutoLedger Tariffs & Pricing Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

CREATE TABLE IF NOT EXISTS tariff_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    plan_type VARCHAR(30) NOT NULL DEFAULT 'TIME_OF_USE',
    currency VARCHAR(3) NOT NULL DEFAULT 'EUR',
    flat_rate_cents BIGINT,
    peak_rate_cents BIGINT,
    offpeak_rate_cents BIGINT,
    time_windows JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tariff_plans_user ON tariff_plans(user_id);

ALTER TABLE vehicles
    ADD COLUMN IF NOT EXISTS tariff_plan_id UUID REFERENCES tariff_plans(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS is_home_charger_default BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS public_charging_presets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    connection_fee_cents BIGINT NOT NULL DEFAULT 0,
    price_per_kwh_cents BIGINT NOT NULL DEFAULT 0,
    price_per_minute_cents BIGINT NOT NULL DEFAULT 0,
    idle_fee_per_minute_cents BIGINT NOT NULL DEFAULT 0,
    idle_grace_minutes INT NOT NULL DEFAULT 0,
    currency VARCHAR(3) NOT NULL DEFAULT 'EUR',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_public_presets_user ON public_charging_presets(user_id);
