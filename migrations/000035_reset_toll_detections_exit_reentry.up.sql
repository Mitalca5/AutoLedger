-- ============================================================================
-- TeslaCost Reset Toll Detections Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

-- The detection reads the speed near each closed-network gate: a gate passed at motorway speed is not a crossing, and
-- the crossed gates of one network alternate entries and exits (leaving the motorway and coming back on it is two
-- tickets). The stored results are a cache of the previous detection: drop them, a new detection recomputes them.
-- Tolls already applied to drives are expenses and are not touched.
DELETE FROM toll_detections WHERE detected_at < NOW();
