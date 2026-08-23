# ADR-0008: Use a Root-Confined WebDAV Filesystem

Status: Accepted

## Context

The pinned upstream `caddy-webdav` handler constructs `golang.org/x/net/webdav.Dir`
for each request. `webdav.Dir` explicitly follows symbolic links inside the
configured root, so an authorized WebDAV user could read or modify a target
outside a share through an in-share link. A recursive preflight scan cannot
close this time-of-check/time-of-use race.

## Decision

DavDeck vendors a minimal, fixed-source replacement of the pinned
`github.com/mholt/caddy-webdav` module under `caddy/caddy-webdav`. The module
keeps the upstream Caddy module ID (`http.handlers.webdav`) and public JSON
configuration, but replaces `webdav.Dir` with a `webdav.FileSystem` backed by
Go's `os.Root`.

The root is opened once during Caddy provisioning and retained as an OS
descriptor/handle. Every WebDAV filesystem operation is therefore constrained
by the operating system to that original root. Symlinks may be followed only
when their target remains inside the root; a root-path replacement after
provisioning does not redirect an existing share.

The build retains the upstream immutable version as the base dependency and
uses an explicit local `replace` in the reproducible xcaddy build command.

## Alternatives considered

- Leave `webdav.Dir` and scan share trees for links: rejected because the scan
  is raceable and filesystem contents can change after validation.
- Reject all shares containing links: rejected for the same TOCTOU reason and
  because it cannot safely inspect every operation.
- Implement separate `openat` and Windows reparse-point code: rejected in
  favor of `os.Root`, which provides one maintained OS-confinement abstraction
  for DavDeck's supported macOS, Linux, and Windows targets.

## Consequences

- Caddy/WebDAV is now a deliberately maintained DavDeck security fork; future
  upstream upgrades must rebase and rerun the confinement and ACL suites.
- WebDAV roots must be concrete directories at provision time; Caddy runtime
  placeholders are not supported by this managed module.
- Unix/macOS confinement is covered by the pinned real-runtime integration
  test. Windows junction/reparse-point behavior still requires native CI or
  release-host evidence before a public 1.0 release claim.
