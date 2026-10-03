UPDATE vehicles SET powertrain = 'ICE' WHERE powertrain IN ('PHEV', 'REEV');
ALTER TABLE vehicles DROP CONSTRAINT chk_vehicle_powertrain;
ALTER TABLE vehicles ADD CONSTRAINT chk_vehicle_powertrain CHECK (powertrain IN ('EV', 'ICE'));
