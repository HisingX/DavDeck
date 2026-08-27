# Security Design

This document defines DavDeck's security baseline and threat model for MVP/1.0.

## 1. Security goals

DavDeck should:

- prevent unauthorized management changes
- prevent unauthorized WebDAV access
- avoid storing plaintext credentials
- minimize exposure of administrative interfaces
- prevent accidental access outside configured shares
- avoid leaking secrets through logs, exports, diagnostics, or crash output
- preserve the last known working runtime on bad configuration
- minimize privilege use
- make dangerous states visible to users

## 2. Trust boundaries

Key boundaries:

1. GUI/CLI client -> local management API
2. `davd` -> SQLite/filesystem
3. `davd` -> Caddy Admin API/process
4. WebDAV client -> Caddy/WebDAV handler -> shared filesystem
5. service manager -> privileged OS operations
6. import/export files -> application state

## 3. Local management API

Requirements:

- bind to loopback only
- authenticate every mutating request
- use a cryptographically random management token
- do not enable permissive CORS
- set request body limits and server timeouts
- reject unexpected content types for JSON endpoints
- sanitize errors returned to clients
- never log Authorization header/token

The local API is not designed for untrusted remote networks.

## 4. Management token storage

Generate at least 256 bits of cryptographically secure randomness.

Unix-like systems:

- token file readable only by intended user/service account (`0600` or stricter equivalent)
- parent runtime/config directory should also have restrictive permissions

Windows:

- restrict file ACL to intended user/service identity where practical

Do not embed the token in command-line arguments or logs.

## 5. Caddy Admin API

Only `davd` may call it.

Requirements:

- local-only
- not exposed to GUI/CLI directly
- not exposed on LAN/WAN
- endpoint details not treated as a public API

## 6. Password handling

Requirements:

- plaintext password never stored
- strong password hash compatible with current supported runtime design
- salt handled by chosen password hashing algorithm
- hashes excluded from normal API responses
- passwords excluded from logs, audit events, diagnostics, exports, crash reports
- CLI prefers `--password-stdin` or interactive input
- GUI clears password fields after operation completion

## 7. WebDAV authorization

MVP authorization boundary is per Share.

Tests must verify:

- valid user + READ_WRITE: expected read/write operations succeed
- valid user + READ: read/discovery succeeds, mutations fail
- user with NONE/no ACL: cannot access share
- anonymous request: denied
- disabled user: denied

Authorization must be tested with the actual runtime, not only unit-tested policy mapping.

The authenticated WebDAV discovery root must apply the same filtering: it may
list only enabled Shares for which the authenticated user has READ or
READ_WRITE access. It must not expose physical filesystem paths or reveal
other users' Share names.

## 8. Path safety

Threats:

- `../` traversal
- encoded traversal
- mixed path separators
- Windows drive/UNC quirks
- symlink/junction escape
- case-sensitivity differences
- Unicode normalization edge cases

Rules:

- Share path must be absolute.
- Slug is validated separately from filesystem path.
- URL routing data must never be concatenated directly into filesystem paths without canonical handling.
- Removing a share must never delete physical content.
- Symlink/junction behavior must be explicitly tested and documented before 1.0.

If the underlying WebDAV implementation cannot safely enforce the intended boundary, the feature must be constrained or redesigned rather than documented as secure without evidence.

### Symlink/junction confinement

DavDeck's fixed Caddy WebDAV module uses Go `os.Root` for every WebDAV
filesystem operation. The root is opened at Caddy provisioning time, so an
in-share symbolic link cannot escape it and replacing the configured root path
later cannot redirect the active share. A real pinned-runtime integration test
enforces this on Unix/macOS.

Windows junction/reparse-point behavior must still be executed on a native
Windows release host before public 1.0 release approval. A one-time recursive
scan is not used because it is vulnerable to filesystem changes and TOCTOU
races.

## 9. TLS

Automatic/public TLS should be delegated to Caddy's supported mechanisms.

Internal TLS:

- clearly explain client trust requirements
- protect local CA private material
- diagnostics may expose certificate metadata but not private keys

Custom certificate mode:

- validate file existence/readability
- do not copy private-key content into logs or API responses

DNS provider credentials are deferred; if added later they require OS secure storage or encrypted secrets design.

## 10. Privilege model

DavDeck should run with the minimum privileges needed.

Do not run the GUI permanently as administrator/root.

Privileged operations such as installing a system service, binding privileged ports, or modifying protected locations should use a scoped elevation mechanism.

Do not recursively chmod/chown user shares as a shortcut.

## 11. Import/export

YAML import is untrusted input.

Requirements:

- schema/version validation
- strict type/field validation
- path validation
- duplicate user/share detection
- no arbitrary shell commands
- no arbitrary Caddy config passthrough in default format

Default export must exclude:

- plaintext passwords
- management token
- TLS private keys
- DNS tokens

## 12. Logs and diagnostics

Redact:

- Authorization headers
- management token
- passwords
- password hashes
- TLS private keys
- DNS/API secrets
- sensitive request bodies

Diagnostic reports default to sanitized mode.

The Management API exposes only a daemon-owned bounded in-memory recent-log
store. It must not read arbitrary user-selected files, and it must not expose
raw Caddy or platform log paths. Records are sanitized before persistence in
the store and at the API response boundary; unavailable or rotated external
logs are reported as unavailable rather than guessed or copied into the API.

Optionally allow a clearly labeled local-only full-path report, but never include secrets.

## 13. Audit events

Audit events should capture management actions without secret payloads.

Examples:

- USER_CREATED
- USER_DISABLED
- PASSWORD_CHANGED
- SHARE_CREATED
- SHARE_REMOVED
- ACL_CHANGED
- TLS_CHANGED
- CONFIG_APPLIED
- SERVICE_INSTALLED

## 14. Dependency and supply-chain security

- Pin production dependencies.
- Isolate Caddy/caddy-webdav upgrades.
- Run tests after dependency updates.
- Generate release checksums.
- Prefer signed release artifacts when signing infrastructure is available.
- Do not implement unsafe auto-update that downloads and executes unverified binaries.

## 15. Security-sensitive review checklist

For any auth/ACL/path/TLS/privilege/update change, answer:

1. What trust boundary changes?
2. What new input is attacker-controlled?
3. Can this expose data outside a share?
4. Can this expose management APIs remotely?
5. Can this leak secrets through logs/errors/export?
6. Does it add privilege requirements?
7. What integration tests verify the behavior?

## 16. Vulnerability reporting

Public repository should include root `SECURITY.md` with supported-version and private-reporting guidance. Do not instruct reporters to publish unpatched exploit details in public issues. User-facing preview boundaries are documented in `docs/KNOWN_LIMITATIONS.md`.

Private vulnerability reporting is not configured yet. Configure a private
reporting channel before treating the public repository as ready for regular
security disclosures.
