# Architecture

This document defines the target internal architecture and dependency rules for DavDeck.

## 1. Architectural goals

The architecture must be:

- cross-platform
- testable from macOS as the primary development host
- usable on Linux headless systems
- secure by default
- deterministic in configuration generation
- maintainable by a small open-source team
- friendly to incremental AI-assisted development

## 2. System context

```text
+------------------------+       +------------------------+
| Flutter Desktop GUI    |       | davctl CLI             |
| macOS/Windows/Linux    |       | all server platforms   |
+-----------+------------+       +-----------+------------+
            |                                |
            +---------- local API -----------+
                             |
                             v
                   +--------------------+
                   | davd               |
                   | application core   |
                   +---+-----+------+---+
                       |     |      |
                       |     |      +------ Platform adapters
                       |     +------------- SQLite repositories
                       +------------------- Caddy runtime/config
                                           |
                                           v
                                  +-------------------+
                                  | Caddy + WebDAV    |
                                  +---------+---------+
                                            |
                                            v
                                      Local filesystem
```

## 3. Monorepo layout

Recommended baseline:

```text
/
├── README.md
├── CONTRIBUTING.md
├── SECURITY.md
├── docs/
│   ├── INDEX.md
│   ├── PROJECT_SPEC.md
│   ├── ARCHITECTURE.md
│   ├── SECURITY.md
│   ├── API.md
│   ├── CLI.md
│   ├── PLATFORM.md
│   ├── CADDY.md
│   ├── TESTING.md
│   ├── RELEASE.md
│   ├── ROADMAP.md
│   ├── adr/
│   └── tasks/
├── core/
│   ├── go.mod
│   ├── cmd/
│   │   ├── davd/
│   │   └── davctl/
│   ├── internal/
│   │   ├── domain/
│   │   ├── app/
│   │   ├── storage/
│   │   ├── caddy/
│   │   ├── api/
│   │   ├── platform/
│   │   ├── config/
│   │   └── diagnostics/
│   ├── migrations/
│   └── testdata/
├── gui/
│   ├── lib/
│   ├── macos/
│   ├── windows/
│   └── linux/
├── packaging/
│   ├── macos/
│   ├── windows/
│   └── linux/
├── scripts/
└── .github/workflows/
```

Exact folder names may evolve, but dependency direction must remain stable.

## 4. Core package responsibilities

### `internal/domain`

Contains entities/value objects and pure domain validation.

Examples:

- `User`
- `Share`
- `SharePermission`
- `TLSProfile`
- `ServerSettings`
- `ConfigRevision`

Rules:

- no DB drivers
- no OS process calls
- no HTTP handler dependencies
- no Caddy Admin client
- no UI concerns

### `internal/app`

Application/use-case services.

Examples:

- CreateUser
- ChangePassword
- CreateShare
- SetSharePermission
- UpdateTLS
- ValidateConfiguration
- ApplyConfiguration
- RestoreRevision
- RunDiagnostics

Application services depend on interfaces such as repositories, config compiler, runtime manager, and platform services.

### `internal/storage`

Persistence adapters.

SQLite implementation handles:

- repository queries
- transactions
- migrations integration
- schema-specific serialization

Application code should not embed raw SQL.

### `internal/caddy`

Caddy integration boundary.

Suggested components:

- `Compiler`
- `Validator`
- `RuntimeManager`
- `AdminClient`
- `ProcessManager`
- `ModuleInspector`
- `LogAdapter`

The compiler should accept an immutable domain snapshot and produce deterministic Caddy JSON.

### `internal/api`

Local management HTTP server.

Responsibilities:

- routing
- auth middleware
- request validation
- response mapping
- body limits/timeouts
- mapping application errors to stable API error codes

HTTP handlers must stay thin and call application services.

### `internal/platform`

Operating-system integration.

Responsibilities may include:

- standard application paths
- system service install/start/stop/status
- privilege elevation hooks
- filesystem capabilities
- secure local secret-file permissions
- platform-specific process behavior

Use small interfaces and build-tagged implementations.

### `internal/diagnostics`

Composes checks from other subsystems. It should avoid duplicating low-level platform or Caddy logic.

## 5. Dependency direction

Allowed high-level direction:

```text
api -> app -> domain
         |      ^
         v      |
     interfaces |
         |      |
         +------+

storage/platform/caddy -> implement app/domain-facing interfaces
```

Avoid circular dependencies.

Domain must be the least coupled layer.

## 6. Management API transport

MVP uses authenticated loopback HTTP JSON.

Rationale:

- portable across all three desktop/server OSes
- trivial for Flutter and Go clients
- easier to integration-test than separate Unix socket/Named Pipe stacks
- keeps IPC implementation simple for MVP

Expected listener:

- random or configured high local port, bound only to loopback
- management token read from an OS-protected local file

Future remote management must not reuse this interface by simply changing the bind address.

## 7. Daemon startup lifecycle

Suggested sequence:

1. Resolve platform paths.
2. Initialize structured logging.
3. Acquire singleton lock if needed.
4. Open SQLite database.
5. Apply pending migrations.
6. Load/generate management token if needed.
7. Initialize repositories/application services.
8. Locate/verify bundled Caddy binary/module set.
9. Inspect saved runtime state.
10. Start management API on loopback.
11. Start/attach to managed Caddy runtime according to service mode/state.
12. Begin health monitoring.

Startup errors must be explicit and safe. Corrupt/migration-failed databases must not be silently recreated.

## 8. Configuration transaction model

A configuration-changing API operation may change application state and optionally apply runtime configuration.

Recommended pattern:

1. Validate request.
2. Execute DB transaction for domain changes.
3. Re-read/build canonical domain snapshot.
4. Compile deterministic Caddy JSON.
5. Validate Caddy JSON.
6. Reuse a matching validated config revision, or record a new revision.
7. Apply Caddy config.
8. Verify health.
9. Mark revision active on success.
10. On apply failure, keep DB state but mark runtime out-of-sync, or roll DB transaction back if operation design supports atomic apply.

The project must make the chosen consistency model explicit before implementation of automatic apply. Two acceptable approaches:

### Approach A — explicit Apply

CRUD changes are saved to SQLite as desired state. Runtime only changes when user invokes Apply. This is simpler and makes desired-vs-active state visible.

### Approach B — transactional auto-apply

Each mutation attempts compile/validate/apply. Failure must restore previous application state or clearly represent desired-vs-active divergence.

DavDeck uses a hybrid of these approaches (ADR-0007): user, share, and ACL
mutations automatically apply through the daemon-owned pipeline. TLS updates
and YAML imports remain explicit-Apply changes. Failed automatic application
retains the desired state and last known working runtime, and returns a stable
failure code rather than claiming the change is active. Runtime lifecycle
operations do not create revisions; revisions are deduplicated by generated
configuration hash.

## 9. Desired vs active state

For both automatic and explicit Apply, represent:

- `desired_revision`
- `active_revision`
- `runtime_state`

GUI/CLI can show “configuration changes pending”.

This avoids lying to users when DB changes exist but Caddy still runs a previous configuration.

## 10. Configuration compiler design

Compiler input should be a canonical immutable struct, for example:

```text
RuntimeConfigInput
  ServerSettings
  TLSProfile
  []User
  []ShareWithPermissions
```

Compiler output:

- canonical Caddy JSON bytes/object
- optional warnings
- deterministic hash

Do not let the compiler read SQLite, environment-specific UI state, or CLI flags directly.

Compiler must have deterministic ordering to make golden tests/revisions meaningful.

## 11. Caddy process model

`davd` owns Caddy process/runtime management.

Support:

- locate expected binary
- verify version/modules
- start
- stop
- restart
- query status
- validate config
- load/reload config
- collect/route logs

Caddy should run as a separate process, not be embedded into the GUI process.

## 12. Database and migrations

SQLite is the source of truth for application configuration/state.

Rules:

- enable foreign keys
- use explicit migrations
- keep migration history immutable after release
- use transactions for multi-table updates
- back up or checkpoint as appropriate before destructive migration
- store timestamps consistently

Suggested tables:

- `schema_meta`
- `users`
- `shares`
- `share_permissions`
- `server_settings`
- `tls_profiles`
- `config_revisions`
- `audit_events`

## 13. Password flow

Client sends plaintext password only over loopback management API for the immediate operation.

`davd` hashes password using the approved scheme and stores only the hash.

Password plaintext should not be retained longer than needed and must never be echoed back.

CLI should prefer interactive/stdin input.

## 14. GUI architecture

Recommended Flutter layers:

```text
UI widgets
  -> view models/state controllers
  -> repositories/use-case facades
  -> generated or handwritten management API client
```

Do not put backend rules in widgets.

Use localization from the start for English and Simplified Chinese.

## 15. CLI architecture

Suggested structure:

```text
command parsing
  -> management API client
  -> output formatter
```

All CLI commands should share a common API client, auth-token resolution, error mapper, and output mode handling.

## 16. Platform abstraction examples

Prefer small interfaces:

```go
type ServiceManager interface {
    Install(ctx context.Context) error
    Uninstall(ctx context.Context) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Restart(ctx context.Context) error
    Status(ctx context.Context) (ServiceStatus, error)
}
```

```go
type PathProvider interface {
    DataDir() string
    ConfigDir() string
    RuntimeDir() string
    LogDir() string
}
```

Avoid one large platform God interface.

## 17. Error model

Application errors should carry:

- stable code
- human-readable safe message
- optional structured details
- wrapped internal cause for logs/debugging

Do not expose raw internal stack traces or secret-bearing errors to GUI/CLI.

## 18. Health state model

Server/runtime status should be more expressive than a boolean.

Recommended states:

- `STOPPED`
- `STARTING`
- `RUNNING`
- `DEGRADED`
- `STOPPING`
- `ERROR`

`DEGRADED` may represent a running Caddy process whose configured endpoint/health check is failing.

## 19. Concurrency and locking

`davd` is the only writer to application state. GUI and CLI should never bypass it.

Protect:

- config apply/reload sequence
- migrations/startup
- service installation operations
- Caddy process lifecycle

Concurrent conflicting mutations should return a deterministic error rather than produce overlapping reloads.

## 20. Extensibility boundaries

Do not build a general plugin architecture for MVP.

Design clean interfaces around Caddy compiler/runtime, repositories, platform services, and diagnostics so future features can evolve without rewriting the whole application.

Potential V2 virtual filesystem support should remain outside MVP architecture until a concrete implementation is selected.
