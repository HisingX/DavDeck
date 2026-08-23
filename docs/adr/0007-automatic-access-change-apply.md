# ADR-0007: Automatically Apply Ordinary WebDAV Access Changes

Status: Accepted

## Context

ADR-0005 selected explicit Apply for every desired-state mutation. In routine
use this meant that creating a user, changing a password, editing a share, or
changing an ACL had no effect on the running WebDAV service until the operator
navigated to a separate screen and applied configuration manually. This made
authentication and authorization appear broken and separated the action from
its required runtime effect.

## Decision

`davd` automatically applies configuration after a successful mutation to a
user, share, or share permission. The same serialized apply path remains in
use: snapshot, compile, validate, record a revision, reload or start Caddy,
then verify runtime health.

TLS profile updates and YAML configuration imports remain desired-state-only
changes and require explicit Apply. They can represent broader deployment
changes and are commonly prepared or reviewed in batches.

If automatic application fails, DavDeck preserves both the newly persisted
desired state and the last known working Caddy runtime. The request returns the
stable validation/runtime failure code; it must not report that the change is
active. An explicit retry through `config apply` remains available.

## Alternatives considered

- Keep explicit Apply for all changes: rejected because it makes normal user,
  password, share, and ACL management unexpectedly ineffective.
- Roll back the database mutation when reload fails: rejected because it can
  discard an intentional desired-state change and requires a broad
  cross-repository transaction redesign. Desired and active state already
  model this condition safely.
- Apply configuration in GUI or CLI clients: rejected because only `davd` owns
  Caddy management and headless/API clients must have identical behavior.

## Consequences

- Successful ordinary access-management responses mean the runtime has passed
  validation, reload, and health verification.
- Failure responses can mean the desired state was stored but is pending;
  clients should show the stable error and offer a retry rather than silently
  treating the change as active.
- ADR-0005 is superseded for user, share, and ACL mutations. Its explicit
  Apply behavior remains for TLS updates, YAML imports, and direct `config
  apply` use.
