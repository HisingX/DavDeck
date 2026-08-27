# Testing Strategy

## 1. Goals

Testing must prove more than compilation. DavDeck controls authentication, filesystem access, TLS, services, and a separate Caddy runtime; correctness depends on integration behavior.

## 2. Test pyramid

### Unit tests

Use for:

- domain validation
- username/slug normalization
- permission mapping
- config transformation
- error mapping
- pure utility functions
- CLI formatting
- Flutter view-model/state logic

### Integration tests

Use for:

- SQLite repositories and migrations
- local management API auth/routing
- Caddy config validation
- actual WebDAV requests
- filesystem behavior
- TLS preflight logic

### Platform/system tests

Use for:

- Linux systemd integration
- Windows/macOS desktop tray and window lifecycle
- OS permissions/paths
- installer/package behavior
- reboot/startup scenarios

## 3. Core unit test requirements

Each domain rule should have table-driven tests where practical.

Examples:

- invalid usernames
- duplicate-normalized usernames
- invalid share slug
- relative share path rejected
- permission enum validation
- TLS mode validation

## 4. Database tests

For each schema release:

- fresh DB migration succeeds
- upgrade from previous schema succeeds
- foreign keys enforced
- unique constraints behave correctly
- transaction rollback works on failure
- migration failure is reported and does not silently recreate DB

Use temporary databases.

## 5. Management API tests

Test:

- valid token
- missing token
- invalid token
- invalid JSON
- oversized body
- duplicate username
- invalid share path
- missing resources
- stable error codes
- no password/hash fields in responses
- concurrent apply conflict behavior

## 6. Config compiler golden tests

Maintain deterministic fixtures, for example:

```text
testdata/caddy/
  single_share/
  multiple_users/
  read_only/
  read_write/
  multiple_shares/
  tls_automatic/
  tls_internal/
  tls_custom/
```

Each fixture contains canonical input and expected generated JSON.

Normalize non-semantic ordering only if the compiler cannot make it deterministic; deterministic output is preferred.

## 7. Real Caddy validation

Compiler output should also be run through the pinned real Caddy validation/provisioning path in integration tests.

Golden comparison alone is insufficient.

## 8. WebDAV integration matrix

Create temporary filesystem roots and test users.

Example:

- `alice`: READ_WRITE
- `bob`: READ
- `charlie`: NONE/no ACL
- anonymous

Test at least:

### Discovery/read

- OPTIONS
- PROPFIND
- GET
- HEAD where relevant

### Mutations

- PUT
- MKCOL
- DELETE
- MOVE
- COPY where supported/meaningful
- PROPPATCH/LOCK/UNLOCK as client compatibility requires

Expected matrix is determined by the real pinned runtime and documented permission policy.

The WebDAV discovery root must also be tested with multiple shares. `PROPFIND`
at the public base path returns only the authenticated user's enabled READ or
READ_WRITE shares, supports `Depth: 0` and `Depth: 1`, and rejects mutations.
The response must not contain physical filesystem paths or another user's
share names.

## 9. Authentication tests

Verify:

- correct credentials accepted
- incorrect password denied
- disabled user denied
- anonymous denied
- credentials for one user cannot inherit another user's ACL

## 10. Path-security tests

Include:

- `..` traversal attempts
- URL-encoded traversal attempts
- mixed separators
- Unicode names
- spaces
- symlink escape on Unix/macOS
- junction/symlink behavior on Windows when environment allows

Do not claim confinement without these tests.

The pinned runtime suite covers raw/encoded/double-encoded parent traversal,
mixed separators, encoded absolute paths, Unicode/spaces, and a real Unix
symlink escape assertion. `make caddy-security-release-gate` must pass;
Windows junction/reparse-point behavior remains a native release gate.

## 11. TLS tests

CI may use local/internal TLS for deterministic testing.

Test:

- compiler mode selection
- invalid custom cert/key paths
- internal TLS endpoint starts
- certificate metadata can be inspected without exposing private keys

Public ACME tests should not run on every PR unless a safe dedicated environment exists.

## 12. CLI tests

Test:

- command parsing
- API client invocation
- `--json`
- exit codes
- password stdin behavior
- stderr/stdout separation
- daemon connection failure

## 13. Flutter tests

At minimum:

- state/view-model unit tests
- API error mapping
- form validation
- key widget tests for user/share/ACL flows

Avoid making all behavior depend on expensive end-to-end UI tests.

## 14. Cross-platform CI matrix

Suggested PR jobs:

### Go

- macOS
- Ubuntu
- Windows

Run:

```text
gofmt check
go vet
go test
build davd
davctl build
```

### Flutter

- analyze/test on a stable CI platform
- native build jobs for macOS, Windows, Linux where practical

### Caddy/WebDAV

Run real integration tests at least on Linux CI for every PR, and add Windows/macOS coverage for platform-sensitive behavior.

## 15. Release-candidate smoke tests

Manual or automated checklist:

### macOS ARM64

- install
- close window to menu bar and Exit from the status-bar menu
- create user/share
- apply ACL
- connect via WebDAV client
- HTTPS/internal TLS smoke
- restart app/service
- uninstall preserves data by default

### Windows x64

Same functional path plus close-to-tray and Exit behavior. Windows native
service integration is deferred.

### Linux x64 headless

- install server package/archive
- systemd service
- full configuration via CLI
- WebDAV client operations
- reboot/startup if environment permits

### Linux ARM64

At least binary start, DB migration, CLI API interaction, Caddy/WebDAV smoke.

`make platform-smoke` cross-builds the four supported Go targets, verifies the
embedded target metadata, then starts the native target's `davd` and queries it
with the native `davctl`. Cross-built non-native binaries are build evidence,
not runtime evidence; CI runs the native smoke on each OS family.

## 16. Test reporting

AI agents and contributors must report exactly what was run. If a platform-specific test cannot run locally, state that and rely on CI rather than pretending success.
