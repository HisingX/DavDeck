# Data Model

## 1. Purpose

This document defines the logical persistence model. SQLite is the authoritative source of desired application state.

All schema changes require migrations.

## 2. General conventions

Recommended conventions:

- IDs: canonical lowercase UUID text identifiers, treated as opaque values.
- Timestamps: non-zero UTC values serialized as RFC3339Nano.
- Enums: stored as stable text values unless a migration-safe integer scheme is deliberately chosen.
- Foreign keys: enabled and enforced.
- Secrets: store only what is necessary; never plaintext passwords.

## 3. Users

Logical fields:

```text
id
username
username_normalized
password_hash
enabled
created_at
updated_at
```

Constraints:

- unique normalized username
- non-empty username
- no control characters
- password hash required for password-authenticated users

MVP normalization trims surrounding whitespace and applies Unicode-aware
lowercasing. The stored `username_normalized` must exactly match this result.

API models must omit `password_hash`.

## 4. Shares

Logical fields:

```text
id
name
slug
path
enabled
created_at
updated_at
```

Constraints:

- unique slug
- absolute path
- valid slug without separators/traversal

Removing a row does not remove physical files.

## 5. Share permissions

Logical fields:

```text
share_id
user_id
permission
created_at
updated_at
```

Composite uniqueness on `(share_id, user_id)`.

Permission enum:

```text
NONE
READ
READ_WRITE
```

Absence of a row should be treated consistently. Recommended semantic: absence = NONE. If implemented this way, the API should still be explicit and tests must cover it.

## 6. Server settings

Prefer a structured table/model rather than an unbounded key/value store for core settings.

Potential fields:

```text
id (singleton)
public_base_path
http_port
https_port
runtime_mode
created_at
updated_at
```

Only include settings actually required by current features.

## 7. TLS profile

Logical fields vary by mode.

Common:

```text
id
mode
hostname
created_at
updated_at
```

Custom mode may reference certificate/key paths, not copy private key material into general-purpose DB fields unless a deliberate encrypted secret design exists.

## 8. Config revisions

Suggested fields:

```text
id / revision_number
created_at
config_json
config_hash
validation_status
apply_status
app_version
error_code
error_summary
```

Track active/desired pointers either here or in a separate singleton runtime-state table.

Do not store secret-bearing debug payloads in revisions.

## 9. Audit events

Suggested fields:

```text
id
created_at
event_type
subject_type
subject_id
safe_metadata_json
```

Never include passwords/tokens/private keys.

## 10. Runtime state

Persist enough state to distinguish desired configuration from active runtime.

Possible singleton fields:

```text
desired_revision
active_revision
dirty
last_apply_at
last_apply_status
```

The runtime's live health state may remain in memory and be recomputed after restart.

DavDeck uses a singleton runtime-state row for desired/active revision pointers
and a dirty bit. Database triggers mark the state dirty when users, shares,
permissions, or server settings change. A successful automatic or explicit
Apply advances the active pointer; failed validation/reload preserves the
previous active revision.

## 11. Migration principles

- New schema change = new migration.
- Never edit a migration that has shipped.
- Test fresh install and upgrade.
- Migration failure must be visible and non-destructive.
- Before destructive transformations, preserve rollback/recovery data where practical.
