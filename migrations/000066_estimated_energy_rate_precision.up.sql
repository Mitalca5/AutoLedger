-- Estimated prices use the same range and precision as other electricity rates.
ALTER TABLE vehicles ALTER COLUMN estimated_price_per_kwh TYPE NUMERIC(12, 6);
