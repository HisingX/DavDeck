# CLI Design (`davctl`)

## 1. Purpose

`davctl` is the full-featured headless management client for DavDeck. Linux server administration must be possible through `davctl` without a GUI.

`davctl` talks to the local `davd` Management API and must not bypass it.

## 2. Command tree

Baseline:

```text
davctl status
davctl version [--json]

davctl server start
davctl server stop
davctl server restart
davctl server status
davctl server settings
davctl server ports --http <port> --https <port>

davctl service install
davctl service uninstall
davctl service start
davctl service stop
davctl service status

davctl user list
davctl user add <username>
davctl user delete <username-or-id>
davctl user enable <username-or-id>
davctl user disable <username-or-id>
davctl user passwd <username-or-id>

davctl share list
davctl share add <name> <path>
davctl share update <share>
davctl share remove <share>

davctl acl list [share]
davctl acl set <share> <user> <none|read|read-write>

davctl tls show
davctl tls automatic <hostname> [--challenge auto|dns] [--dns-provider <provider-id>]
davctl tls internal <hostname>
davctl tls custom --hostname <hostname> --cert <path> --key <path>
davctl tls check
davctl tls disable

davctl dns-provider list
davctl dns-provider add <name> --provider <cloudflare|tencentcloud|dnspod|alidns> [--zones <zone[,zone...]>] --secret-stdin
davctl dns-provider remove <provider-id>

davctl config validate
davctl config apply
davctl config status
davctl config export [--output file]
davctl config import <file>

davctl revision list
davctl revision restore <revision>
davctl revision delete <revision>

davctl logs [--follow]
davctl doctor
```

`davctl doctor` runs the daemon-owned diagnostic suite. Human output lists each
check and stable code; `--json` returns the sanitized versioned report. A report
with overall `FAIL` returns a non-zero operational exit code.

Exact flags can evolve but should remain predictable and scriptable.

`davctl version` is local-only and does not require daemon discovery. It reports
the DavDeck/Git build identity, toolchain versions, pinned Caddy/WebDAV versions,
and target OS/architecture; `--json` provides the release automation form.

`davctl config export` writes deterministic, versioned safe YAML to stdout.
`--output <file>` creates a new file without overwriting an existing one.
`davctl user`, `davctl share`, and `davctl acl` mutations automatically apply
the corresponding WebDAV runtime configuration. A failure reports the stable
apply error while retaining the desired state and last known working runtime.

`davctl config import <file>` performs a transactional merge/upsert and reports
which newly created users need a password set through `davctl user passwd`.
Import changes desired state only; run `davctl config apply` separately.

`davctl config validate` compiles and validates the desired state without
changing the runtime. `davctl revision list` lists safe revision metadata, and
`davctl revision restore <revision-id>` asks `davd` to revalidate and activate a
previously valid revision. `davctl revision delete <revision-id>` removes an
unreferenced stored revision without touching share files. Starting, stopping,
or restarting the server does not create a new revision, and applying an
unchanged configuration reuses the existing revision.

`davctl server start`, `stop`, `restart`, and `status` manage or inspect the
daemon-owned Caddy runtime; they do not install or control a system service.

`davctl service` manages the Linux systemd service through `davd`. It is the
current supported system-service workflow; Windows and macOS return the stable
`PLATFORM_UNSUPPORTED` error until their native service lifecycle is validated.
Installation and lifecycle operations may require administrator privileges and
return a structured `PRIVILEGE_REQUIRED` error when elevation is unavailable.

`davctl server ports` validates and applies the managed HTTP/HTTPS listener
ports. It rejects unavailable local ports before any settings are persisted.

When both stdin and stdout are terminals, running `davctl` without a command
opens a line-oriented interactive menu. It covers status, users, shares,
permissions, HTTPS/TLS, configuration, logs, diagnostics, backups, and service
operations, and uses the same API/command paths as explicit commands. A first
interactive connection with no users or shares offers a short setup wizard.
Non-TTY invocations (including pipes and CI) continue to print usage and never
enter the menu.

`davctl logs` retrieves a bounded newest-first page from the sanitized Logs API.
It supports `--limit`, `--cursor`, `--since`, `--level`, and `--component` in
both human and `--json` modes. `--follow` is currently unsupported and returns
the stable `LOG_FOLLOW_UNSUPPORTED` usage error until the API provides a safe
streaming contract.

## 3. Output modes

Default output is human-readable.

Commands that return structured data should support:

```text
--json
```

JSON mode should be stable enough for shell automation and must not include human decoration.

## 4. Exit codes

Recommended baseline:

- `0`: success
- `1`: generic operational failure
- `2`: invalid CLI usage/input
- `3`: authentication/daemon connection failure
- `4`: configuration validation/apply failure

More granular codes may be introduced later, but avoid inconsistent per-command behavior.

## 5. Password input

Prefer:

```bash
davctl user add alice --password-stdin
```

or interactive prompt with no echo.

Do not encourage plaintext password command-line flags.

DNS provider secrets follow the same rule: `davctl dns-provider add` accepts a
JSON object only through `--secret-stdin`; secrets are not accepted as command
line flags. Example: `printf '%s' '{"api_token":"..."}' | davctl dns-provider add production --provider cloudflare --secret-stdin`.
The `dnspod` provider accepts `{"api_token":"APP_ID,APP_TOKEN"}` for the
legacy DNSPod API; `tencentcloud` accepts `{"secret_id":"...","secret_key":"..."}`.

## 6. Daemon discovery

CLI should resolve:

- local management endpoint
- management token file

through shared platform path/config helpers.

Do not duplicate platform path logic in every command.

## 7. Human-readable status

Example:

```text
DavDeck
Daemon:      Running
Caddy:       Running
WebDAV:      Running
Service:     RUNNING (installed: true, starts at boot: true)
HTTPS:       Active
URL:         https://dav.example.com
Users:       3
Shares:      4
Config:      12 desired / 12 active
```

`davctl status` consumes the same state values as the Management API and
prints a safe last error code when a component is `FAILED`, `DEGRADED`, or
`UNKNOWN`. `davctl --json status` returns the API status object without human
decoration.

## 8. Error behavior

CLI should print concise safe messages to stderr and return non-zero status.

In `--json` mode, return structured machine-readable error output using API error codes where possible.

Never print secrets in errors.

## 9. Automation principles

- avoid interactive prompts when all required flags/stdin are provided
- stable `--json`
- predictable exit codes
- no ANSI decoration in machine mode
- support stdout redirection for exports
- do not require GUI environment variables or desktop session

## 10. Help text

Each command should explain side effects. Destructive metadata operations must clarify that physical files are preserved unless a future explicit destructive operation is introduced.
