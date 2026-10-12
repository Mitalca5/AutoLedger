ALTER TABLE comparison_scenarios ADD COLUMN currency VARCHAR(3) NOT NULL DEFAULT ''
    CHECK (currency = '' OR currency ~ '^[A-Z]{3}$');
-- Tracked scenarios have a reliable source; legacy projections remain unknown until confirmed by their owner.
UPDATE comparison_scenarios s SET currency = v.currency FROM vehicles v WHERE s.vehicle_id = v.id;
