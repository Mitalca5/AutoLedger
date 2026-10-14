-- Automatic toll detection relies on a French motorway dataset. 'auto' shows it where the vehicle drives in France
-- (or its owner uses the French interface), 'on' always shows it and 'off' never does. Manual tolls are not affected.
ALTER TABLE vehicles ADD COLUMN toll_detection TEXT NOT NULL DEFAULT 'auto'
    CHECK (toll_detection IN ('auto', 'on', 'off'));
