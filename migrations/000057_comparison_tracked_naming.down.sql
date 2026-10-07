UPDATE comparison_scenarios
SET options = (options - 'tracked_incentives') || jsonb_build_object('ev_incentives', options -> 'tracked_incentives')
WHERE options ? 'tracked_incentives';
ALTER TABLE comparison_scenarios DROP CONSTRAINT IF EXISTS chk_comparison_mode_inputs;
ALTER TABLE comparison_scenarios RENAME COLUMN tracked_inputs TO ev_inputs;
ALTER TABLE comparison_scenarios ADD CONSTRAINT chk_comparison_mode_inputs CHECK (
    (mode = 'RETROSPECTIVE' AND vehicle_id IS NOT NULL)
    OR (mode = 'PROJECTION' AND ev_inputs IS NOT NULL)
);
