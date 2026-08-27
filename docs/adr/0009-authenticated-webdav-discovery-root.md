# ADR-0009: Add an Authenticated WebDAV Discovery Root

Status: Accepted

## Context

DavDeck exposes each Share at a distinct URL such as `/dav/photos/`. This keeps
each physical directory as an independent authorization boundary, but a user
with several Shares must know every slug before connecting from a WebDAV
client. The public base path currently has no route of its own.

## Decision

The configured public base path serves an authenticated virtual discovery
collection. For example, `PROPFIND /dav/` returns the root collection and only
the enabled Shares for which the authenticated user has `READ` or
`READ_WRITE` permission. `GET` returns a simple HTML listing and `OPTIONS`
reports the read-only WebDAV capabilities of the collection.

The discovery collection contains links to Share routes only. It does not
merge physical roots, perform file operations, or change the existing
per-Share authentication and ACL routes. The root supports `Depth: 0` and
`Depth: 1`; deeper discovery is rejected. `/dav` and `/dav/` are both handled
without relying on a redirect.

The behavior is implemented by the managed Caddy module
`http.handlers.davdeck_index`. The compiler supplies a deterministic mapping
from authenticated usernames to Share slugs and display names. The handler
does not access SQLite, the Management API, or the filesystem.

## Alternatives considered

- A global static directory listing: rejected because it would disclose Share
  names to users who are not authorized to access them.
- A symlink or bind-mount based merged directory: rejected because it would
  complicate root confinement, portability, and filesystem lifecycle.
- A full per-user virtual filesystem: deferred because cross-Share MOVE, COPY,
  locking, rename, and path-security semantics require a larger design.
- Redirecting to the first Share: rejected because it cannot represent a
  user's complete set of Shares.

## Consequences

- `/dav/` becomes the recommended connection URL for clients that support
  WebDAV directory discovery.
- Individual `/dav/<slug>/` URLs remain available for compatibility.
- The root collection is read-only and cross-Share MOVE/COPY is unsupported.
- The managed Caddy binary must contain both the WebDAV and discovery modules.
- Client compatibility must be validated against the real pinned Caddy runtime;
  clients that do not discover collections can use a direct Share URL.
