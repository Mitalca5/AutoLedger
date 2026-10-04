ALTER TABLE drives DROP CONSTRAINT IF EXISTS drives_driver_id_fkey;
ALTER TABLE vehicles DROP CONSTRAINT IF EXISTS vehicles_default_driver_id_fkey;

UPDATE drives d SET driver_id = (SELECT p.user_id FROM vehicle_people p WHERE p.id = d.driver_id) WHERE d.driver_id IS NOT NULL;
UPDATE vehicles v SET default_driver_id = (SELECT p.user_id FROM vehicle_people p WHERE p.id = v.default_driver_id) WHERE v.default_driver_id IS NOT NULL;

ALTER TABLE drives ADD CONSTRAINT drives_driver_id_fkey FOREIGN KEY (driver_id) REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE vehicles ADD CONSTRAINT vehicles_default_driver_id_fkey FOREIGN KEY (default_driver_id) REFERENCES users(id) ON DELETE SET NULL;

DROP TABLE IF EXISTS vehicle_people;
