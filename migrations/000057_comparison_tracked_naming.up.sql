-- The comparison's second vehicle is the user's tracked vehicle (electric, hybrid or other), not necessarily an EV.
ALTER TABLE comparison_scenarios DROP CONSTRAINT IF EXISTS chk_comparison_mode_inputs;
ALTER TABLE comparison_scenarios RENAME COLUMN ev_inputs TO tracked_inputs;
ALTER TABLE comparison_scenarios ADD CONSTRAINT chk_comparison_mode_inputs CHECK (
    (mode = 'RETROSPECTIVE' AND vehicle_id IS NOT NULL)
    OR (mode = 'PROJECTION' AND tracked_inputs IS NOT NULL)
);
UPDATE comparison_scenarios
SET options = (options - 'ev_incentives') || jsonb_build_object('tracked_incentives', options -> 'ev_incentives')
WHERE options ? 'ev_incentives';
