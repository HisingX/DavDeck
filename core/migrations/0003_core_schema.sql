CREATE TABLE users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL,
    username_normalized TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE shares (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    path TEXT NOT NULL,
    enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE share_permissions (
    share_id TEXT NOT NULL REFERENCES shares(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    permission TEXT NOT NULL CHECK (permission IN ('NONE', 'READ', 'READ_WRITE')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (share_id, user_id)
);

CREATE TABLE server_settings (
    id TEXT PRIMARY KEY,
    public_base_path TEXT NOT NULL,
    http_port INTEGER NOT NULL CHECK (http_port BETWEEN 1 AND 65535),
    https_port INTEGER NOT NULL CHECK (https_port BETWEEN 1 AND 65535),
    runtime_mode TEXT NOT NULL CHECK (runtime_mode IN ('portable', 'service')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK (http_port <> https_port)
);

CREATE TABLE tls_profiles (
    id TEXT PRIMARY KEY,
    mode TEXT NOT NULL CHECK (mode IN ('automatic', 'internal', 'custom')),
    hostname TEXT NOT NULL,
    certificate_path TEXT NOT NULL DEFAULT '',
    private_key_path TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE config_revisions (
    id TEXT PRIMARY KEY,
    revision_number INTEGER NOT NULL UNIQUE CHECK (revision_number > 0),
    created_at TEXT NOT NULL,
    config_json BLOB NOT NULL,
    config_hash TEXT NOT NULL,
    validation_status TEXT NOT NULL CHECK (validation_status IN ('PENDING', 'VALID', 'INVALID')),
    apply_status TEXT NOT NULL CHECK (apply_status IN ('NOT_APPLIED', 'APPLIED', 'FAILED')),
    app_version TEXT NOT NULL,
    error_code TEXT NOT NULL DEFAULT '',
    error_summary TEXT NOT NULL DEFAULT ''
);

CREATE TABLE audit_events (
    id TEXT PRIMARY KEY,
    created_at TEXT NOT NULL,
    event_type TEXT NOT NULL,
    subject_type TEXT NOT NULL,
    subject_id TEXT NOT NULL,
    safe_metadata_json TEXT NOT NULL DEFAULT '{}'
);
