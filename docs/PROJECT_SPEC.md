# DavDeck Project Specification

Status: Draft baseline for MVP development
Audience: Maintainers and contributors
Working project name: DavDeck
Primary development host: macOS Apple Silicon

## 1. Product definition

DavDeck is a cross-platform open-source WebDAV server management application powered by Caddy. It provides a native desktop GUI for desktop users and a complete CLI/headless workflow for server environments.

The product must allow users to operate a secure WebDAV service without understanding Caddy configuration syntax.

## 2. Target users

Primary users:

- macOS and Windows desktop users who want a simple personal WebDAV server.
- Linux server users who administer machines over SSH.
- Home-server and NAS users.
- Small teams that need basic per-user access to shared directories.
- Advanced users who want deterministic CLI/YAML automation.

## 3. Primary goals

A user should be able to:

1. Install DavDeck.
2. Start the local management daemon.
3. Create and manage WebDAV users.
4. Set/change passwords securely.
5. Add one or more local filesystem shares.
6. Grant each user no access, read-only access, or read-write access per share.
7. Configure HTTP/HTTPS without hand-writing Caddy configuration.
8. Use automatic public HTTPS when a suitable domain is available.
9. Use internal/local HTTPS for LAN environments.
10. Use a custom certificate/key if desired.
11. Start, stop, restart, inspect, and install the service for boot-time operation.
12. Diagnose DNS, ports, filesystem permissions, Caddy, TLS, authentication, and WebDAV behavior.
13. Export/import safe configuration.
14. Use the full server feature set from a Linux terminal without any desktop GUI.

## 4. Non-goals for MVP / 1.0

Do not implement unless explicitly re-scoped:

- Browser-based admin UI
- WebView-based desktop shell
- Electron/Tauri/Wails UI
- LDAP / AD / OAuth / SSO
- Cloud account or hosted control plane
- S3 / SMB / FTP backends
- Full file-browser UI
- Multi-node cluster / HA
- Kubernetes operator
- Terraform provider
- Plugin marketplace
- Enterprise IAM/RBAC
- Quotas/billing
- Complex multi-tenant hosting
- Arbitrary user-authored Caddy configuration as the default path

## 5. Product shape

The project produces three logical applications:

### `davd`

Background daemon and application core. Owns business logic, SQLite, Caddy config/runtime, service management, diagnostics, and the local management API.

### `davctl`

Command-line management client. Talks to `davd`. Intended for Linux headless operation and automation.

### Desktop GUI

Flutter desktop application for macOS, Windows, and selected Linux desktop targets. Talks to `davd` through the same management API as the CLI.

## 6. First-class platform matrix

| Platform | Daemon | CLI | GUI | Priority |
|---|---|---|---|---|
| macOS ARM64 | Yes | Yes | Yes | Tier 1 / primary dev |
| Windows x64 | Yes | Yes | Yes | Tier 1 |
| Linux x64 | Yes | Yes | Optional desktop build | Tier 1 server |
| Linux ARM64 | Yes | Yes | Later/optional | Tier 1 server |
| Windows ARM64 | Later | Later | Later | Future |
| macOS Intel | Best effort | Best effort | Best effort | Low |

Linux headless operation is a first-class product requirement, not a fallback mode.

## 7. Core data model

### User

Suggested fields:

- `id`
- `username`
- `username_normalized`
- `password_hash`
- `enabled`
- `created_at`
- `updated_at`

Rules:

- Username is unique after normalization.
- No empty/control-character usernames.
- Password plaintext is never stored.
- Password hashes are never returned through normal API responses.

### Share

Suggested fields:

- `id`
- `name`
- `slug`
- `path`
- `enabled`
- `created_at`
- `updated_at`

Rules:

- Path is absolute.
- Slug is unique.
- Slug cannot contain path traversal or path separators.
- Removing a Share removes metadata/ACLs only; never the physical directory.

### SharePermission

MVP enum:

- `NONE`
- `READ`
- `READ_WRITE`

Do not model this as a boolean. Future models may add CREATE/UPDATE/DELETE/MOVE/LOCK semantics.

### TLSProfile

Modes:

- `automatic`
- `internal`
- `custom`

### ServerSettings

Contains managed listener/domain/runtime preferences needed to compile Caddy configuration.

### ConfigRevision

Stores generated-config revision metadata to support troubleshooting and rollback.

### AuditEvent

Stores security-relevant management actions without secrets.

## 8. Share URL model

MVP treats each Share as an authorization boundary and exposes it under a distinct path, for example:

- `/dav/photos/`
- `/dav/documents/`
- `/dav/backup/`

A single user may have different permissions for different shares.

The public base path also serves as an authenticated discovery collection. A
user sees only enabled Shares for which they have READ or READ_WRITE access;
the collection does not merge physical roots or provide cross-Share file
operations. Individual Share paths remain the authorization boundaries.

A unified virtual filesystem that maps many physical roots into one per-user namespace is intentionally deferred beyond MVP.

## 9. WebDAV permission semantics

MVP UI exposes only `READ` and `READ_WRITE`.

Expected read-only behavior generally permits read/discovery methods required by clients and blocks mutating methods. Exact method handling must be verified against the real Caddy + WebDAV stack in integration tests.

Do not rely on an untested hard-coded assumption about every WebDAV method.

## 10. Authentication

MVP uses WebDAV-compatible HTTP authentication under HTTPS, with server-side password hashing compatible with the selected Caddy authentication design.

Requirements:

- No plaintext password persistence.
- No password/hash leakage through API responses or logs.
- CLI should support secure password input such as stdin/interactive mode.
- Anonymous access is denied by default.

## 11. HTTPS modes

### Automatic public HTTPS

For a public hostname. DavDeck should guide the user through hostname, DNS/port checks, and runtime status. Caddy manages certificate issuance/renewal.

### Internal HTTPS

For LAN/internal use. DavDeck configures Caddy internal PKI and helps the user export or locate the CA certificate for client trust setup.

### Custom certificate

Advanced users may provide certificate and private-key paths. Secrets/private keys must be handled carefully and never copied into logs.

DNS-01 provider integrations are a post-MVP feature.

## 12. Runtime modes

### Portable/development mode

Useful for local development and temporary operation. `davd` launches/manages Caddy without requiring full system-service installation.

### Service mode

For long-running production use:

- macOS: launchd
- Linux: systemd
- Windows: Windows Service Control Manager

Closing the GUI must not imply stopping a properly installed service.

## 13. Management API

MVP uses an authenticated loopback-only HTTP JSON API for GUI/CLI-to-daemon communication.

Reasons:

- same implementation on macOS/Linux/Windows
- straightforward Flutter and Go clients
- easy automated testing
- no GUI dependency on Unix socket or Named Pipe plugins

Security requirements:

- loopback-only binding
- authentication token
- no permissive CORS
- request body limits and timeouts
- stable error codes
- no secrets in logs

This API is not a remote-management interface.

## 14. Configuration source of truth

SQLite is authoritative.

```text
GUI / CLI
   -> Management API
   -> application services
   -> SQLite
   -> domain snapshot
   -> Caddy config compiler
   -> generated Caddy JSON
   -> validate
   -> apply
```

YAML is import/export/automation format only.

## 15. Configuration apply and rollback

Apply flow:

1. Build a consistent domain snapshot.
2. Compile deterministic Caddy JSON.
3. Validate configuration.
4. Record revision/attempt metadata.
5. Apply/reload using backend-only Caddy integration.
6. Verify runtime health.
7. Mark active revision or report failure.

Validation or reload failure must not silently destroy the last known working runtime.

## 16. GUI scope

Recommended navigation:

- Dashboard
- Users
- Shares
- HTTPS
- Service
- Logs
- Diagnostics
- Settings

Dashboard should show service state, URL, HTTPS state, user/share counts, Caddy state, and uptime.

Users page should support add/list/change password/enable/disable/delete.

Shares page should support create/update/remove and per-user permissions.

TLS page should implement a guided mode selector rather than exposing Caddy internals.

Diagnostics should be a first-class product feature.

## 17. CLI scope

Initial command tree:

```text
davctl status

davctl server start|stop|restart

davctl user list|add|delete|enable|disable|passwd

davctl share list|add|update|remove

davctl acl list|set

davctl tls show|automatic|internal|custom

davctl config validate|apply|export|import

davctl revision list|restore

davctl logs

davctl doctor
```

Human-readable output is default. `--json` should be available where useful for automation.

## 18. Diagnostics scope

Diagnostics should be able to check, as applicable:

- daemon state
- SQLite access/migrations
- data/config/runtime directories
- Caddy binary and expected WebDAV module
- generated config validity
- share existence/readability/writability
- port availability
- DNS resolution
- TLS certificate/runtime status
- authentication
- WebDAV OPTIONS/PROPFIND
- read/write behavior using safe temporary test data where possible

Reports must be sanitized by default.

## 19. Logging and audit

Prefer structured logs with component and level.

Components may include `davd`, `api`, `storage`, `caddy`, `webdav`, `platform`, `diagnostics`.

Audit events should include actions such as USER_CREATED, PASSWORD_CHANGED, SHARE_CREATED, ACL_CHANGED, TLS_CHANGED, CONFIG_APPLIED.

Never log sensitive values.

## 20. Import/export

YAML format must include an explicit schema/version field.

Default export should not contain plaintext passwords, management tokens, TLS private keys, or DNS tokens.

Encrypted full migration bundles may be designed later.

## 21. Packaging goals

Release artifacts should eventually include:

- macOS ARM64 desktop package
- Windows x64 desktop installer/package
- Linux x64 desktop package where practical
- Linux x64 server archive/package
- Linux ARM64 server archive/package
- standalone CLI binaries as appropriate
- checksums
- release metadata showing DavDeck/Caddy/caddy-webdav versions

Package-manager distribution (Homebrew, winget, deb/rpm repositories) is a later milestone.

## 22. Security baseline

Before 1.0:

- password hashing verified
- management API loopback-only and authenticated
- Caddy Admin API backend-only and local-only
- no secret logging
- config validate-before-apply
- filesystem/path validation
- ACL integration tests
- symlink/junction behavior explicitly tested/documented
- dependency versions pinned
- release checksums produced
- update mechanism, if present, verifies authenticity/integrity

## 23. Privacy baseline

No telemetry by default.

Any future telemetry must be opt-in and must not collect usernames, file names, filesystem paths, secrets, or raw domain identifiers without an explicit privacy design.

## 24. MVP milestones

### Phase 0 — Bootstrap

- monorepo skeleton
- Go module
- `davd`
- `davctl`
- Flutter skeleton
- logging
- SQLite migrations
- loopback management API
- CI skeleton

Acceptance: daemon starts, CLI can query status, GUI can display daemon status, core builds on primary CI platforms.

### Phase 1 — Users

- user CRUD
- secure password hashing/input
- API + CLI + basic GUI
- tests

### Phase 2 — Shares

- share CRUD
- path validation
- API + CLI + GUI
- tests

### Phase 3 — ACL

- NONE/READ/READ_WRITE
- Caddy compilation
- real WebDAV integration tests

### Phase 4 — Caddy runtime

- pinned custom Caddy build
- caddy-webdav module
- compiler
- validate
- start/stop/restart/reload
- runtime health

### Phase 5 — HTTPS

- automatic
- internal
- custom certificate
- CLI and GUI flows

### Phase 6 — System services

- launchd
- systemd
- Windows SCM

### Phase 7 — Diagnostics

- `davctl doctor`
- GUI diagnostics
- sanitized report

### Phase 8 — Release hardening

- packaging
- CI matrix
- checksums
- upgrade path
- platform smoke tests
- docs

## 25. 1.0 acceptance criteria

1. macOS ARM64 supports complete GUI setup.
2. Windows x64 supports complete GUI setup.
3. Linux x64 and Linux ARM64 support complete CLI/headless setup.
4. No plaintext password persistence.
5. Per-share user permissions work correctly.
6. Read-only users cannot mutate data.
7. Read-write users can use standard WebDAV operations.
8. Anonymous/unauthorized access is denied.
9. HTTPS modes function as documented.
10. Invalid generated Caddy config cannot replace the working runtime.
11. Service can run at boot on supported server platforms.
12. Config import/export works safely.
13. Diagnostic reports are useful and sanitized.
14. CI exercises supported OSes and real WebDAV behavior.
15. Release artifacts include integrity metadata.
16. A new user can create the first share without manually editing Caddy configuration.

## 26. Future roadmap (non-binding)

V1.x may add packaging polish, QR codes, client setup guides, IP allowlists, rate limiting, backups, safe updates, and additional package managers.

V2 may explore remote management over SSH, multi-server profiles, virtual filesystem mapping, fine-grained permissions, quotas, DNS providers, and deeper audit capabilities.

## 27. Product invariant

DavDeck is a managed WebDAV server product, not a generic Caddy editor. Product UI and APIs should expose users, shares, permissions, HTTPS, service state, and diagnostics; Caddy remains an implementation detail managed by the backend.
