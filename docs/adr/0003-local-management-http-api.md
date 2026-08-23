# ADR-0003: Use Authenticated Loopback HTTP JSON for Local Management

Status: Accepted for MVP

## Context

GUI and CLI require a portable IPC mechanism to manage `davd` across macOS, Windows, and Linux. Unix sockets and Windows Named Pipes would require separate transports and additional Flutter integration complexity.

## Decision

Use an authenticated HTTP JSON API bound only to loopback for MVP.

API namespace begins at `/api/v1/`.

Authentication uses a cryptographically random local management token stored with restrictive OS permissions.

## Rejected alternatives

- Unix Domain Socket + Windows Named Pipe: potentially stronger local IPC semantics but higher cross-platform client complexity for MVP.
- Public TCP API: rejected due to remote attack surface.
- Browser admin UI: out of scope.

## Consequences

- Strong loopback-only enforcement is mandatory.
- CORS should not be enabled permissively.
- Remote management requires a separate future design; it must not be implemented by rebinding this API to all interfaces.
