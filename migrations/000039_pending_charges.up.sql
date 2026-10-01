-- ============================================================================
-- AutoLedger Pending Charges Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

CREATE TABLE IF NOT EXISTS pending_charges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source VARCHAR(50) NOT NULL DEFAULT 'homeassistant',
    charger_name VARCHAR(100),
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    energy_kwh NUMERIC(8, 3) NOT NULL,
    location VARCHAR(100) DEFAULT 'home',
    raw_data JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pending_charges_user_created ON pending_charges(user_id, created_at DESC);
