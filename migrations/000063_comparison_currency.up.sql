ALTER TABLE comparison_scenarios ADD COLUMN currency VARCHAR(3) NOT NULL DEFAULT ''
    CHECK (currency = '' OR currency ~ '^[A-Z]{3}$');
-- Tracked scenarios have a reliable source: the currency of their vehicle.
UPDATE comparison_scenarios s SET currency = v.currency FROM vehicles v WHERE s.vehicle_id = v.id;
-- A legacy projection was labelled with the currency of the active vehicle. When all the vehicles of its owner share one
-- currency, that label is known; otherwise the projection stays unknown until its owner confirms the currency.
UPDATE comparison_scenarios s SET currency = c.currency
FROM (SELECT user_id, MIN(currency) AS currency FROM vehicles GROUP BY user_id HAVING COUNT(DISTINCT currency) = 1) c
WHERE s.currency = '' AND s.user_id = c.user_id;
