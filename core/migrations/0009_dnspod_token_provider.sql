ALTER TABLE tls_profiles RENAME TO tls_profiles_v8;
ALTER TABLE dns_provider_secrets RENAME TO dns_provider_secrets_v8;
ALTER TABLE dns_provider_credentials RENAME TO dns_provider_credentials_v8;

CREATE TABLE dns_provider_credentials (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    provider TEXT NOT NULL CHECK (provider IN ('cloudflare', 'tencentcloud', 'dnspod', 'alidns')),
    allowed_zones_json TEXT NOT NULL DEFAULT '[]',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

INSERT INTO dns_provider_credentials(id, name, provider, allowed_zones_json, created_at, updated_at)
SELECT id, name, provider, allowed_zones_json, created_at, updated_at
FROM dns_provider_credentials_v8;

CREATE TABLE dns_provider_secrets (
    credential_id TEXT PRIMARY KEY REFERENCES dns_provider_credentials(id) ON DELETE CASCADE,
    ciphertext BLOB NOT NULL,
    updated_at TEXT NOT NULL
);

INSERT INTO dns_provider_secrets(credential_id, ciphertext, updated_at)
SELECT credential_id, ciphertext, updated_at
FROM dns_provider_secrets_v8;

CREATE TABLE tls_profiles (
    id TEXT PRIMARY KEY,
    mode TEXT NOT NULL CHECK (mode IN ('automatic', 'internal', 'custom')),
    hostname TEXT NOT NULL,
    certificate_path TEXT NOT NULL DEFAULT '',
    private_key_path TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    challenge TEXT NOT NULL DEFAULT 'auto' CHECK (challenge IN ('auto', 'dns')),
    dns_provider_id TEXT REFERENCES dns_provider_credentials(id)
);

INSERT INTO tls_profiles(id, mode, hostname, certificate_path, private_key_path, created_at, updated_at, challenge, dns_provider_id)
SELECT id, mode, hostname, certificate_path, private_key_path, created_at, updated_at, challenge, dns_provider_id
FROM tls_profiles_v8;

DROP TABLE tls_profiles_v8;
DROP TABLE dns_provider_secrets_v8;
DROP TABLE dns_provider_credentials_v8;

CREATE TRIGGER tls_profiles_mark_runtime_dirty AFTER INSERT ON tls_profiles BEGIN
    UPDATE runtime_state SET dirty = 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = 1;
END;

CREATE TRIGGER tls_profiles_update_mark_runtime_dirty AFTER UPDATE ON tls_profiles BEGIN
    UPDATE runtime_state SET dirty = 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = 1;
END;

CREATE TRIGGER tls_profiles_delete_mark_runtime_dirty AFTER DELETE ON tls_profiles BEGIN
    UPDATE runtime_state SET dirty = 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = 1;
END;

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
