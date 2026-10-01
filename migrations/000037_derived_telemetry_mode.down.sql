ALTER TABLE vehicles ALTER COLUMN make SET DEFAULT 'Generic';
UPDATE vehicles SET make = 'Generic' WHERE LENGTH(make) = 0;
