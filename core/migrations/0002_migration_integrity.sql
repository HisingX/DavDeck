ALTER TABLE schema_migrations ADD COLUMN name TEXT;
ALTER TABLE schema_migrations ADD COLUMN checksum TEXT;

UPDATE schema_migrations
SET name = '0001_bootstrap.sql',
    checksum = 'e5a30efee1f5fceb0567b0d9e0d727aa88603bc280207caca8d0201541a9d21f'
WHERE version = 1;
