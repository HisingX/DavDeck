INSERT INTO server_settings(id, public_base_path, http_port, https_port, runtime_mode, created_at, updated_at)
SELECT '00000000-0000-4000-8000-000000000001', '/dav', 8080, 8443, 'portable', strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE NOT EXISTS (SELECT 1 FROM server_settings);

CREATE TABLE runtime_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    desired_revision_id TEXT REFERENCES config_revisions(id),
    active_revision_id TEXT REFERENCES config_revisions(id),
    dirty INTEGER NOT NULL DEFAULT 1 CHECK (dirty IN (0, 1)),
    updated_at TEXT NOT NULL
);

INSERT INTO runtime_state(id, dirty, updated_at)
VALUES (1, 1, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));

CREATE TRIGGER users_mark_runtime_dirty AFTER INSERT ON users BEGIN
    UPDATE runtime_state SET dirty = 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = 1;
END;
CREATE TRIGGER users_update_mark_runtime_dirty AFTER UPDATE ON users BEGIN
    UPDATE runtime_state SET dirty = 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = 1;
END;
CREATE TRIGGER users_delete_mark_runtime_dirty AFTER DELETE ON users BEGIN
    UPDATE runtime_state SET dirty = 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = 1;
END;
CREATE TRIGGER shares_mark_runtime_dirty AFTER INSERT ON shares BEGIN
    UPDATE runtime_state SET dirty = 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = 1;
END;
CREATE TRIGGER shares_update_mark_runtime_dirty AFTER UPDATE ON shares BEGIN
    UPDATE runtime_state SET dirty = 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = 1;
END;
CREATE TRIGGER shares_delete_mark_runtime_dirty AFTER DELETE ON shares BEGIN
    UPDATE runtime_state SET dirty = 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = 1;
END;
CREATE TRIGGER permissions_mark_runtime_dirty AFTER INSERT ON share_permissions BEGIN
    UPDATE runtime_state SET dirty = 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = 1;
END;
CREATE TRIGGER permissions_update_mark_runtime_dirty AFTER UPDATE ON share_permissions BEGIN
    UPDATE runtime_state SET dirty = 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = 1;
END;
CREATE TRIGGER permissions_delete_mark_runtime_dirty AFTER DELETE ON share_permissions BEGIN
    UPDATE runtime_state SET dirty = 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = 1;
END;
CREATE TRIGGER settings_update_mark_runtime_dirty AFTER UPDATE ON server_settings BEGIN
    UPDATE runtime_state SET dirty = 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = 1;
END;
