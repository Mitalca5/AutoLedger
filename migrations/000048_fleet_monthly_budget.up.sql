-- Household monthly spending target, in hundredths of the fleet currency; NULL means no budget.
ALTER TABLE users ADD COLUMN fleet_monthly_budget BIGINT CHECK (fleet_monthly_budget IS NULL OR fleet_monthly_budget > 0);
