# Local Management API

Status: MVP design baseline

## 1. Purpose

The Management API is the local control interface used by the Flutter GUI and `davctl` to manage `davd`.

It is not a remote administration API and must remain loopback-only in MVP/1.0.

Base path:

```text
/api/v1
```

## 2. Transport and authentication

- HTTP JSON over loopback
- Bind only to `127.0.0.1` and/or `::1`
- Bearer management token
- JSON request/response bodies
- No permissive CORS
- Request body limits and timeouts

Header:

```http
Authorization: Bearer <management-token>
```

## 3. Common response envelope

Success:

```json
{
  "success": true,
  "data": {}
}
```

Failure:

```json
{
  "success": false,
  "error": {
    "code": "SHARE_PATH_NOT_FOUND",
    "message": "Share directory does not exist",
    "details": {}
  }
}
```

The `code` field is stable and machine-readable. Client logic must not depend on English message text.

## 4. Suggested endpoint set

### Status

`GET /api/v1/status`

Returns the live daemon/runtime/service contract and desired-vs-active
configuration state. The endpoint remains safe to call when Caddy or the
native service is stopped; unavailable components are reported as `UNKNOWN`
with a stable `last_error_code` instead of failing the whole status request.
The desktop GUI ignores native service fields because desktop service
installation is not part of the current GUI milestone.

Component states are:

`NOT_INSTALLED`, `STOPPED`, `STARTING`, `RUNNING`, `STOPPING`, `DEGRADED`,
`FAILED`, and `UNKNOWN`.

Example fields:

```json
{
  "daemon": "RUNNING",
  "database": "READY",
  "caddy": "RUNNING",
  "webdav": "RUNNING",
  "service": {
    "installed": true,
    "state": "RUNNING",
    "starts_at_boot": true
  },
  "desired_revision": 12,
  "active_revision": 11,
  "pending_changes": true,
  "portable_daemon_owned": true
}
```

### Server settings

- `GET /api/v1/server/settings`
- `PUT /api/v1/server/settings`

The update body contains `http_port` and `https_port`. DavDeck validates both
ports, checks that newly selected local listener ports are available, persists
the desired state, then validates and applies the generated Caddy configuration.
An occupied port returns `SERVER_PORT_UNAVAILABLE` without changing saved settings.
The response also includes the configured `public_base_path`, which is the
recommended WebDAV discovery entry point.

`GET /api/v1/server/endpoints` returns the user-facing HTTP and HTTPS endpoint
summary. Each endpoint includes `configured`, `active`, `state`, and
`copyable`. HTTPS is `NOT_CONFIGURED` until a TLS profile exists. A configured
endpoint is copyable only after the desired configuration is active and the
local listener/protocol probe succeeds.

### Users

- `GET /api/v1/users`
- `POST /api/v1/users`
- `GET /api/v1/users/{id}`
- `PATCH /api/v1/users/{id}`
- `DELETE /api/v1/users/{id}`
- `POST /api/v1/users/{id}/password`

Password hashes must never be returned.

### Shares

- `GET /api/v1/shares`
- `POST /api/v1/shares`
- `GET /api/v1/shares/{id}`
- `PATCH /api/v1/shares/{id}`
- `DELETE /api/v1/shares/{id}`

Deletion removes metadata, not physical files.

### ACL

- `GET /api/v1/shares/{shareId}/permissions`
- `PUT /api/v1/shares/{shareId}/permissions/{userId}`
- optional batch endpoint later if needed

Permission values:

- `NONE`
- `READ`
- `READ_WRITE`

### TLS

- `GET /api/v1/tls`
- `PUT /api/v1/tls`
- `DELETE /api/v1/tls` to remove the desired TLS profile and return to HTTP-only
  mode after an explicit Apply
- `POST /api/v1/tls/check` for preflight/diagnostic checks

`GET /api/v1/tls` returns JSON `null` in the response data field until a TLS
profile has been configured. Updating TLS changes desired state only; clients
must explicitly apply configuration before the Caddy runtime changes.

Modes:

- `automatic`
- `internal`
- `custom`

### Configuration

- `POST /api/v1/config/validate`
- `POST /api/v1/config/apply`
- `GET /api/v1/config/state`
- `GET /api/v1/config/export`
- `POST /api/v1/config/import`

`POST /api/v1/config/validate` compiles and validates the current desired
state without writing a revision or changing the managed runtime. A successful
response returns `valid`, the deterministic generated `config_hash`, and any
safe compiler warnings. Validation failures use the same stable configuration
error codes as Apply.

User, share, and ACL mutations automatically compile, validate, reload, and
health-check Caddy before returning success. If that application fails, the
desired-state mutation is retained, the prior runtime remains active, and the
endpoint returns the stable application failure code.

TLS updates and YAML imports remain desired-state-only changes; use explicit
Apply to activate them.

`GET /api/v1/config/export` returns the deterministic, versioned safe YAML in
the normal JSON response envelope:

```json
{
  "success": true,
  "data": {
    "format": "yaml",
    "content": "version: 1\n...",
    "contains_secrets": false
  }
}
```

`POST /api/v1/config/import` accepts the YAML document directly with
`Content-Type: application/yaml`. It parses strictly, validates the complete
document, and merges/upserts it in one transaction. The result reports created
and updated resources, new users requiring a separate password reset, and
`pending_apply: true`. The endpoint never activates Caddy automatically.

### Revisions

- `GET /api/v1/revisions`
- `GET /api/v1/revisions/{id}`
- `POST /api/v1/revisions/{id}/restore`
- `DELETE /api/v1/revisions/{id}`

Raw generated config may be restricted to advanced/debug contexts.

Revision responses omit raw generated configuration. `config/state` reports
desired and active revision numbers, the persisted dirty flag, and whether an
Apply is pending. A concurrent Apply returns `CONFIG_APPLY_IN_PROGRESS`.
Restore accepts only a previously valid revision, validates its stored Caddy
JSON again, activates it through the daemon-owned runtime, and makes it both
the desired and active revision. Runtime or metadata failures leave the
previous active runtime in place where possible.

Revision creation is idempotent by generated configuration hash. Starting,
stopping, or restarting Caddy does not create a revision; those operations
reuse the active revision. Applying an unchanged desired configuration also
returns the existing matching revision. Configuration validation failures do
not create a revision.

Delete removes only the stored revision metadata and generated snapshot. The
active or desired revision cannot be deleted and returns `REVISION_ACTIVE` or
`REVISION_DESIRED`; physical share directories and user files are never
deleted. Revision numbers are monotonic and are not reused after deletion.

### Runtime/service

- `POST /api/v1/server/start`
- `POST /api/v1/server/stop`
- `POST /api/v1/server/restart`
- `GET /api/v1/server/status`

These endpoints manage only DavDeck's Caddy runtime. `davd` remains running so
the local Management API can start it again after a stop.

`GET /api/v1/server/status` returns `caddy`, `webdav`, `last_error_code` when
available, and the desired/active revision pointers. It does not report native
service state.

Linux system-service installation uses separate endpoints:

- `POST /api/v1/service/install`
- `POST /api/v1/service/uninstall`
- `POST /api/v1/service/start`
- `POST /api/v1/service/stop`
- `GET /api/v1/service/status`

The native desktop runner may request graceful shutdown of the daemon process
it started in portable mode with:

- `POST /api/v1/daemon/shutdown`

This is authenticated and loopback-only like the other management endpoints.
The native runner invokes it only for a daemon process it launched in portable
mode; it does not invoke it for a separately installed system service.

These endpoints are authenticated and remain loopback-only. Privileged
operations return the stable `PRIVILEGE_REQUIRED` error when the daemon does
not have administrator rights; the API does not attempt implicit elevation.
On Windows and macOS these endpoints return `PLATFORM_UNSUPPORTED`.
`GET /api/v1/service/status` reports `installed`, `state`, and
`starts_at_boot`. A service query failure is represented as `UNKNOWN` in the
aggregate status response with `SERVICE_STATUS_FAILED`.

### Logs

- `GET /api/v1/logs`

Returns the newest sanitized daemon-owned records first. The first version is
bounded in-memory retrieval; it does not read arbitrary daemon or Caddy log
paths. Streaming and rotated-log access can be added later if needed.

Optional query parameters:

- `limit`: number of records, default `100`, maximum `200`
- `cursor`: exclusive record-ID cursor returned by the previous page
- `since`: RFC3339 timestamp lower bound
- `level`: `DEBUG`, `INFO`, `WARN`, or `ERROR`
- `component`: component filter such as `daemon`, `runtime`, `platform`, or
  `caddy`

The response uses the normal envelope. `data.records` is newest first;
`data.next_cursor` and `data.has_more` describe the next page:

```json
{
  "success": true,
  "data": {
    "records": [
      {
        "id": 42,
        "timestamp": "2026-08-23T01:02:03Z",
        "level": "ERROR",
        "component": "runtime",
        "message": "managed Caddy operation failed",
        "fields": {"error_code": "CADDY_START_FAILED"}
      }
    ],
    "next_cursor": 41,
    "has_more": true
  }
}
```

Records are sanitized before they enter the bounded store and again at the
response boundary. Passwords, password hashes, bearer tokens, Authorization
values, private keys, DNS/API secrets, and sensitive structured fields are not
returned. An unavailable log store returns `LOGS_UNAVAILABLE`; malformed
query parameters return `INVALID_LOG_QUERY`.

### Diagnostics

- `GET /api/v1/diagnostics`
- `POST /api/v1/diagnostics/run`
- `GET /api/v1/diagnostics/report?mode=redacted`

Diagnostic reports are schema-versioned and sanitized. They contain platform,
application version, overall `PASS`/`WARN`/`FAIL` state, stable check codes, and
safe summaries. The `build` object records Git commit, build date, Go/Flutter,
pinned Caddy/WebDAV versions, and target OS/architecture. Full-path or
secret-bearing report modes are not supported.

## 5. Error code baseline

User:

- `USER_NOT_FOUND`
- `USER_ALREADY_EXISTS`
- `INVALID_USERNAME`
- `INVALID_PASSWORD`
- `USER_DISABLED`

Share:

- `SHARE_NOT_FOUND`
- `SHARE_ALREADY_EXISTS`
- `INVALID_SHARE_SLUG`
- `SHARE_PATH_NOT_FOUND`
- `SHARE_PATH_NOT_READABLE`
- `SHARE_PATH_NOT_WRITABLE`

ACL:

- `INVALID_PERMISSION`
- `PERMISSION_NOT_FOUND`

Caddy/runtime:

- `CADDY_NOT_FOUND`
- `CADDY_MODULE_MISSING`
- `CADDY_VALIDATE_FAILED`
- `CADDY_START_FAILED`
- `CADDY_STOP_FAILED`
- `CADDY_RELOAD_FAILED`
- `RUNTIME_UNHEALTHY`
- `ENDPOINT_UNAVAILABLE`

TLS:

- `TLS_CONFIGURATION_ERROR`
- `TLS_CERTIFICATE_NOT_FOUND`
- `TLS_PRIVATE_KEY_NOT_FOUND`
- `DNS_CHECK_FAILED`
- `PORT_CHECK_FAILED`

Database/config:

- `DATABASE_ERROR`
- `MIGRATION_FAILED`
- `CONFIG_IMPORT_INVALID`
- `CONFIG_VERSION_UNSUPPORTED`
- `CONFIG_EXPORT_FAILED`
- `CONFIG_APPLY_IN_PROGRESS`

Platform/service:

- `SERVICE_INSTALL_FAILED`
- `SERVICE_UNINSTALL_FAILED`
- `PRIVILEGE_REQUIRED`
- `PLATFORM_UNSUPPORTED`

Logs:

- `LOGS_UNAVAILABLE`
- `INVALID_LOG_QUERY`

## 6. Validation rules

HTTP handlers validate shape and basic syntax, then application/domain layers enforce business invariants.

Do not duplicate complex rules only in GUI/CLI.

## 7. Concurrency

Only one configuration apply/reload should run at a time.

Concurrent apply request should return a stable conflict error such as `CONFIG_APPLY_IN_PROGRESS`.

## 8. Versioning

Breaking API changes require a new API version or an explicit coordinated migration across daemon, CLI, GUI, docs, and tests.
