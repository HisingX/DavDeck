CREATE TABLE dns_provider_credentials (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    provider TEXT NOT NULL CHECK (provider IN ('cloudflare', 'tencentcloud', 'alidns')),
    allowed_zones_json TEXT NOT NULL DEFAULT '[]',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE dns_provider_secrets (
    credential_id TEXT PRIMARY KEY REFERENCES dns_provider_credentials(id) ON DELETE CASCADE,
    ciphertext BLOB NOT NULL,
    updated_at TEXT NOT NULL
);

ALTER TABLE tls_profiles
ADD COLUMN challenge TEXT NOT NULL DEFAULT 'auto' CHECK (challenge IN ('auto', 'dns'));

ALTER TABLE tls_profiles
ADD COLUMN dns_provider_id TEXT REFERENCES dns_provider_credentials(id);

CREATE TRIGGER dns_provider_credentials_mark_runtime_dirty AFTER INSERT ON dns_provider_credentials BEGIN
    UPDATE runtime_state SET dirty = 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = 1;
END;

CREATE TRIGGER dns_provider_credentials_update_mark_runtime_dirty AFTER UPDATE ON dns_provider_credentials BEGIN
    UPDATE runtime_state SET dirty = 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = 1;
END;

CREATE TRIGGER dns_provider_credentials_delete_mark_runtime_dirty AFTER DELETE ON dns_provider_credentials BEGIN
    UPDATE runtime_state SET dirty = 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = 1;
END;

CREATE TRIGGER dns_provider_secrets_mark_runtime_dirty AFTER INSERT ON dns_provider_secrets BEGIN
    UPDATE runtime_state SET dirty = 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = 1;
END;

CREATE TRIGGER dns_provider_secrets_update_mark_runtime_dirty AFTER UPDATE ON dns_provider_secrets BEGIN
    UPDATE runtime_state SET dirty = 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = 1;
END;

CREATE TRIGGER dns_provider_secrets_delete_mark_runtime_dirty AFTER DELETE ON dns_provider_secrets BEGIN
    UPDATE runtime_state SET dirty = 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = 1;
END;
