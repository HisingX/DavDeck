# ADR-0012: Direct Caddy Force Renewal for Managed Certificates

Status: Accepted

## Context

The first one-shot renewal implementation changed a temporary Caddy config and
used a local TLS handshake to try to make automatic HTTPS notice the renewal.
That was not a reliable trigger: ordinary automatic HTTPS does not necessarily
run CertMagic's on-demand handshake path, so the daemon could report an active
renewal while Caddy had not started ACME at all.

## Decision

The managed Caddy binary exposes a loopback-only admin module for DavDeck's
daemon. The module starts, reports, and cancels one-shot operations. The Caddy
TLS app is patched at the pinned version to expose a narrow operation that
selects the active ACME automation policy, calls CertMagic's forced
`RenewCertSync(..., force=true)` method, and refreshes the managed certificate
cache by replacing older same-subject entries. `davd` keeps the operation state, polls the module and public certificate
file, and preserves the saved TLS profile and active configuration throughout.

The operation is available for concrete hostnames with an existing managed
certificate. DNS provider credentials remain owned by the existing Caddy
configuration path and are never returned through the renewal API.

## Alternatives considered

- Keep the temporary-policy plus local-handshake trigger: rejected because it
  depends on Caddy's ordinary handshake behavior and can produce a false
  in-progress state.
- Implement a separate ACME client in `davd`: rejected because it would
  duplicate Caddy's certificate storage, renewal policy, challenge providers,
  and private-key lifecycle.
- Delete the old certificate before requesting a new one: rejected because a
  failed renewal would unnecessarily take down a working HTTPS deployment.

## Security and platform impact

The new endpoint is available only through Caddy's already loopback-only Admin
API; the Management API remains the only externally reachable control surface
and remains authenticated. Operation responses contain only a hostname,
phase, safe message, and stable error code. DNS credentials, private keys, and
raw Caddy errors are not returned or logged. The Caddy patch is pinned to
v2.11.4 and applied by the reproducible build script, so release builds must
use the project build script rather than an arbitrary stock Caddy binary.

## Migration impact

Existing TLS profiles and Caddy storage are unchanged. A running DavDeck
instance must be restarted with a newly built custom Caddy binary before the
renewal action can use this operation; the daemon reports the missing module as
an actionable renewal error instead of silently falling back to the old
trigger.
