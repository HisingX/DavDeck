# ADR-0010: Separate Configuration Revisions from Runtime Lifecycle

Status: Accepted

## Context

The managed Caddy runtime can be started, stopped, or restarted without any
change to the generated configuration. Treating each lifecycle operation as a
configuration revision creates noisy history and makes the revision number
look like a process restart counter.

## Decision

`config_revisions` represents distinct generated configuration snapshots. The
daemon deduplicates successful applications by deterministic configuration
hash. Starting, stopping, and restarting Caddy reuse the active revision and
do not create a new revision. An initial start may create the first revision
when no active revision exists.

Applying an unchanged configuration is idempotent. Validation failures do not
create new revision rows. Runtime activation failures may retain the newly
created snapshot with failure metadata so the operator can inspect or delete
it; a later cleanup can move attempt history into a separate table.

Active and desired revisions are protected from deletion. Other stored
revision snapshots may be deleted through the authenticated Management API;
deletion never touches user files or physical share directories. Revision
numbers are monotonic and are not reused.

## Consequences

- Revision history represents configuration changes rather than process
  activity.
- Runtime controls can be safely retried without growing history.
- Restore and delete operations must use the daemon-owned serialized apply
  lock and protect active/desired pointers.
- The GUI and CLI must expose lifecycle status separately from configuration
  revision state.
