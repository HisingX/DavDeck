ALTER TABLE config_revisions
ADD COLUMN state_hash TEXT NOT NULL DEFAULT '';

CREATE INDEX config_revisions_identity_idx
ON config_revisions(config_hash, state_hash, revision_number DESC);
