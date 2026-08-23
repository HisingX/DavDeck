CREATE TABLE app_metadata (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    created_at TEXT NOT NULL
);

INSERT INTO app_metadata (singleton, created_at)
VALUES (1, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));
