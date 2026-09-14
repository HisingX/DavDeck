# ADR-0012: Use Semantic Desired-State Identity for Revision Deduplication

Status: Accepted

## Context

Complete revision snapshots retain persistence metadata such as `created_at`
and `updated_at` so a restore can reproduce the stored application state. Those
timestamps change when an operator toggles a permission and later changes it
back, even when the effective desired state is equivalent. Comparing the raw
snapshot bytes therefore creates duplicate revisions. Using only the generated
Caddy configuration hash is also insufficient because disabled users and other
state that does not produce a Caddy route must remain restorable.

## Decision

Store a private `state_hash` alongside `config_hash`. The state hash is computed
from a canonical representation of all behaviorally relevant desired state:
users, password hashes, enabled flags, shares, permissions, server settings,
TLS intent, and DNS provider metadata. Persistence audit timestamps are
excluded. An explicit `NONE` permission and an absent permission row have the
same semantic identity because they have the same MVP access meaning.

Revision lookup uses both hashes. Revisions created before `state_hash` existed
are matched by computing the hash from their complete private snapshot as a
backward-compatible fallback.

Mutation services also treat repeated values as no-ops and report whether a
change occurred, so the API does not trigger an automatic apply for an
idempotent request.

## Consequences

- Reverting a permission or share state can reuse the original revision.
- Disabled or otherwise non-effective application state is not lost during
  deduplication.
- Existing duplicate rows remain available until an explicit history-retention
  policy removes them.
- The state hash must be updated if a new behaviorally relevant desired-state
  field is added.
