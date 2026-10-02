-- One row per CSV import, so the rows it created can be listed and removed together.
CREATE TABLE import_batches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    import_type VARCHAR(10) NOT NULL CHECK (import_type IN ('CHARGES', 'DRIVES', 'FUEL', 'ODOMETER')),
    row_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_import_batches_vehicle ON import_batches (vehicle_id, created_at DESC);

ALTER TABLE charge_logs ADD COLUMN source_batch_id UUID REFERENCES import_batches(id) ON DELETE SET NULL;
ALTER TABLE drives ADD COLUMN source_batch_id UUID REFERENCES import_batches(id) ON DELETE SET NULL;
ALTER TABLE fuel_logs ADD COLUMN source_batch_id UUID REFERENCES import_batches(id) ON DELETE SET NULL;
ALTER TABLE odometer_checkpoints ADD COLUMN source_batch_id UUID REFERENCES import_batches(id) ON DELETE SET NULL;

CREATE INDEX idx_charge_logs_batch ON charge_logs (source_batch_id) WHERE source_batch_id IS NOT NULL;
CREATE INDEX idx_drives_batch ON drives (source_batch_id) WHERE source_batch_id IS NOT NULL;
CREATE INDEX idx_fuel_logs_batch ON fuel_logs (source_batch_id) WHERE source_batch_id IS NOT NULL;
CREATE INDEX idx_odometer_checkpoints_batch ON odometer_checkpoints (source_batch_id) WHERE source_batch_id IS NOT NULL;
