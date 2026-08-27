CREATE TABLE revision_sequence (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    next_number INTEGER NOT NULL CHECK (next_number > 0)
);

INSERT INTO revision_sequence(singleton, next_number)
SELECT 1, COALESCE(MAX(revision_number), 0) + 1
FROM config_revisions;
