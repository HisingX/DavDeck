# ADR-0004: SQLite Is the Desired-State Source of Truth

Status: Accepted

## Context

DavDeck manages users, shares, ACLs, TLS settings, revisions, and service state. Caddy configuration is an implementation artifact rather than the product model.

## Decision

Use SQLite as the authoritative application desired-state store.

Generate Caddy JSON from a canonical domain snapshot derived from SQLite.

YAML is import/export/automation format only.

## Consequences

- Schema changes require migrations.
- GUI and CLI never write SQLite directly.
- Generated Caddy configuration can be replaced/rebuilt from application state.
- Import/export must map through domain validation rather than bypassing it.
