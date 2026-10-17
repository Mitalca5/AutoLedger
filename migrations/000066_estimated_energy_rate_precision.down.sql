-- Refuse a downgrade that would lose either range or recorded precision.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM vehicles
        WHERE abs(estimated_price_per_kwh) > 99.9999
           OR estimated_price_per_kwh <> round(estimated_price_per_kwh, 4)
    ) THEN
        RAISE EXCEPTION 'Estimated electricity prices cannot be represented at the previous precision';
    END IF;
END $$;
ALTER TABLE vehicles ALTER COLUMN estimated_price_per_kwh TYPE NUMERIC(6, 4);
