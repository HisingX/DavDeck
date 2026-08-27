# Development Roadmap

This roadmap is execution-oriented. It should be updated as tasks complete. It does not override `PROJECT_SPEC.md` or ADRs.

Bootstrap Tasks 001–025 are implemented or executed. This does not mean all
1.0 product acceptance criteria pass. The evidence-based release decision and
remaining preview boundaries are maintained in `docs/KNOWN_LIMITATIONS.md`.

## Milestone 0 — Repository bootstrap

Status: Implemented (Phase 0 baseline; native CI execution remains the source
of cross-platform build confirmation).

Deliverables:

- monorepo structure
- Go module
- `davd` skeleton
- `davctl` skeleton
- Flutter desktop skeleton
- structured logging
- SQLite migration framework
- loopback Management API skeleton
- CI baseline

Exit criteria:

- `davd` starts
- `davctl status` reaches daemon
- GUI shows daemon status
- core builds on macOS/Linux/Windows CI

## Milestone 1 — User management

Deliverables:

- User domain model
- repository/migration
- secure password hashing
- user CRUD application services
- API routes
- CLI commands
- GUI user page
- tests

Exit criteria:

- no plaintext passwords persisted
- duplicate usernames handled deterministically
- disabled users represented correctly

## Milestone 2 — Shares

Deliverables:

- Share model/repository
- slug/path validation
- share CRUD API/CLI/GUI
- path diagnostics
- tests

Exit criteria:

- multiple shares can be configured
- deleting a share never deletes physical data

## Milestone 3 — Permissions / ACL

Deliverables:

- NONE/READ/READ_WRITE model
- ACL repository/API/CLI/GUI
- compiler mapping
- real WebDAV integration tests

Exit criteria:

- read-only user cannot mutate
- read-write user can use expected WebDAV operations
- unauthorized/anonymous user denied

## Milestone 4 — Caddy runtime

Deliverables:

- pinned custom Caddy build
- module inspection
- Caddy compiler
- validate-before-apply
- process/runtime manager
- revisions
- health state

Exit criteria:

- first functional WebDAV share runs without user-authored Caddy config
- invalid generated config cannot replace working runtime

## Milestone 5 — TLS

Deliverables:

- automatic public TLS configuration
- internal TLS
- custom certificate mode
- preflight diagnostics
- GUI wizard
- CLI commands

Exit criteria:

- each documented TLS mode has validation and smoke coverage

## Milestone 6 — Service integration

Deliverables:

- systemd
- Linux CLI service lifecycle
- install/uninstall/start/stop/status abstractions
- Linux privilege/error UX

Exit criteria:

- Linux service starts the daemon and restores the active Caddy runtime
- desktop GUI service management remains explicitly out of scope until a
  separate platform validation milestone

## Milestone 7 — Diagnostics and logs

Deliverables:

- `davctl doctor`
- GUI diagnostics
- sanitized diagnostic report
- log viewing/filtering
- actionable platform/Caddy/TLS/filesystem checks

Exit criteria:

- common failures are diagnosable without reading raw Caddy config

## Milestone 8 — Import/export and release hardening

Status: Implemented as release-candidate infrastructure; 1.0 exit criteria are
blocked by the preview boundaries documented in `docs/KNOWN_LIMITATIONS.md`.

Deliverables:

- versioned YAML export/import
- config revisions/restore UX
- packaging
- release CI
- checksums
- installation docs
- smoke tests

Exit criteria:

- 1.0 acceptance criteria from PROJECT_SPEC are met

## Post-1.0 candidates

Prioritize based on actual issues/users:

- Homebrew/winget/deb/rpm polish
- client setup helpers and QR codes
- IP allowlists / rate limiting
- encrypted full backup bundles
- safe automatic updates
- remote management over SSH
- multi-server profiles
- advanced virtual filesystem/ACL design
- DNS-01 providers

Do not start these merely because they are listed here.
