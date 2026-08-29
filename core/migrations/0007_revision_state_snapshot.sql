ALTER TABLE config_revisions
ADD COLUMN state_snapshot_json BLOB NOT NULL DEFAULT '';
