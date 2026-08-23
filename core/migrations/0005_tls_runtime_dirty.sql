CREATE TRIGGER tls_profiles_mark_runtime_dirty AFTER INSERT ON tls_profiles BEGIN
    UPDATE runtime_state SET dirty = 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = 1;
END;

CREATE TRIGGER tls_profiles_update_mark_runtime_dirty AFTER UPDATE ON tls_profiles BEGIN
    UPDATE runtime_state SET dirty = 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = 1;
END;

CREATE TRIGGER tls_profiles_delete_mark_runtime_dirty AFTER DELETE ON tls_profiles BEGIN
    UPDATE runtime_state SET dirty = 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = 1;
END;
