# ADR-0011: Configuration Revisions Capture Complete Desired State

Status: Accepted

## Context

The generated Caddy JSON is a runtime artifact. It contains only the users and
permissions needed by the active routes, so it cannot represent every
authoritative SQLite row. In particular, a disabled user or a permission that
does not produce a route can be absent from the generated JSON. Restoring only
that JSON can therefore make Caddy appear rolled back while the Users and
Shares pages still show newer SQLite state.

## Decision

Every newly applied configuration revision stores two private database-only
artifacts:

1. the generated Caddy JSON; and
2. a versioned complete desired-state snapshot containing server settings, TLS
   intent, users, shares, and share permissions.

Restore must validate that the snapshot deterministically reproduces the
stored Caddy JSON, validate and activate the generated configuration, then
replace the authoritative SQLite state in one transaction. Runtime rollback is
attempted if that transaction fails. Password hashes may be present in this
internal snapshot so deleted users can be restored, but the snapshot is never
returned by the Management API or written to logs.

Revisions created before this migration have no complete state snapshot and
are reported as runtime-only. They cannot be restored safely because DavDeck
must not guess application state from Caddy JSON.

## Alternatives considered

- Reconstruct users and permissions by parsing Caddy JSON: rejected because it
  loses disabled users, non-effective permissions, metadata, and future
  application fields.
- Keep restoring only Caddy JSON and refresh the GUI: rejected because the
  SQLite source of truth would remain inconsistent and a daemon restart could
  reintroduce the mismatch.
- Make YAML export the revision format: rejected because YAML intentionally
  omits password hashes and is an external import/export format, not the
  internal runtime source of truth.

## Consequences

- Revision storage grows with the number of users, shares, and ACL entries.
- Restore is a full application-state operation and refreshes dependent GUI
  pages after success.
- Existing runtime-only revisions remain visible for history but cannot be
  safely restored; a safe YAML export/import is the migration path.
