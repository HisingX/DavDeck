# ADR-0005: Use Explicit Apply for MVP Configuration Changes

Status: Superseded in part by ADR-0007

## Context

Automatically applying Caddy configuration after every CRUD mutation complicates transaction rollback and can create ambiguity when a reload fails after database state has changed.

## Decision

MVP initially used a desired-state plus explicit Apply model.

User/API CRUD operations update desired state in SQLite. The UI/CLI shows pending changes. `config apply` compiles, validates, records a revision, applies the new runtime configuration, verifies health, and then marks that revision active.

ADR-0007 supersedes this decision for ordinary user, share, and ACL mutations;
TLS updates and YAML imports retain explicit Apply.

## Consequences

- Must represent desired vs active revision/state.
- GUI should clearly indicate pending changes.
- Apply operation must be serialized/locked.
- Later auto-apply can be considered as UX sugar if it preserves the same correctness model.
