# DavDeck User Guide

This guide covers the `0.1.0-rc.1` preview. It applies to the macOS ARM64
desktop build, the Windows x64 desktop target, and the Linux x64/ARM64
headless builds. Platform-specific gaps are called out explicitly in each
section.

## 1. Choose an operating mode

DavDeck has one backend and two management clients:

- `davd` owns the database, business rules, generated Caddy configuration,
  runtime lifecycle, and local Management API.
- The native GUI and `davctl` are clients of that API.
- Neither client opens SQLite or calls the Caddy Admin API directly.

Use the GUI on a supported desktop when you want visual setup and diagnostics.
Use the daemon and CLI on a server, in a container-like environment, or over
SSH. The WebDAV service itself may be reachable from the LAN or through a
reverse proxy, but the Management API and Caddy Admin API remain loopback-only.

## 2. Install or build

### Release archive

Each archive contains the following common files:

```text
bin/davd
bin/davctl
libexec/caddy
manifest.json
README.md
README.zh-CN.md
LICENSE
NOTICE
SECURITY.md
```

Windows uses `.exe` suffixes. The Windows desktop archive places `DavDeck.exe`,
`flutter_windows.dll`, and the Flutter `data/` directory at the archive root;
macOS and Linux desktop archives may additionally contain a native application
bundle under `desktop/`.

Before running an archive:

1. Verify the SHA-256 checksum against the separately published checksum file.
2. Read `manifest.json` and confirm the target OS, architecture, and versions.
3. Keep the archive's directory structure intact so `davd` can find the pinned
   Caddy binary.

Preview archives are unsigned. macOS may show a Gatekeeper warning and
Windows may show an unsigned-binary warning.

### Build from source

Install the versions listed in the project documentation, then run:

```bash
make core-build
```

For the GUI:

```bash
make gui-build-macos
```

The current GUI build target is macOS ARM64. Windows GUI packaging is a build
target, but manual Windows GUI validation is deferred for this preview.

## 3. Start the daemon

The packaged daemon should be started with the bundled Caddy binary. From the
root of an extracted archive:

```bash
./bin/davd \
  --caddy-binary ./libexec/caddy \
  --data-dir ./data \
  --config-dir ./config \
  --runtime-dir ./run
```

The Management API binds to a loopback address. The default `--listen`
argument is `127.0.0.1:0`, so the operating system selects a free local port.
The daemon writes its endpoint to `run/management.endpoint` and creates the
management token at `config/management.token`.

For a normal installed desktop or service, omit the portable directory flags
and let the platform path rules select the application directories. Do not
copy the management token into a shell script, issue tracker, or shared
filesystem.

To stop a foreground daemon, press `Ctrl-C`. The daemon shuts down its managed
Caddy process and removes the temporary endpoint file.

## 4. Use the GUI

The GUI is a management client; it does not need direct access to the database.

Typical first-run sequence:

1. Start or install the daemon for the current user.
2. Open DavDeck and wait for the daemon connection to become healthy.
3. Create a user and set its password.
4. Add a share with an absolute filesystem path.
5. Set the user's share permission to `READ` or `READ_WRITE`.
6. Select an HTTPS mode and confirm the listener ports.
7. Apply the configuration and verify the dashboard status.
8. Connect with a WebDAV client using the displayed endpoint and credentials.

When a user has access to multiple shares, use the Dashboard's unified entry
point (`/dav/` by default). WebDAV clients can discover the shares available
to that user; each share remains directly available at its corresponding
`/dav/<slug>/` URL. The unified entry point is read-only and does not support
moving or copying files between shares.

The dashboard, users, shares, TLS, logs, diagnostics, and revision views all
use the same daemon-owned state. If an apply fails, review the
structured error and runtime status before retrying; the last known working
runtime is preserved whenever possible.

## 5. Use `davctl`

`davctl` discovers the local endpoint and token from the platform paths. It
also accepts explicit values:

```bash
./bin/davctl --endpoint http://127.0.0.1:12345 \
  --token-file ./config/management.token status
```

Use `--json` before the command for machine-readable output. `version` is
local-only and does not need a running daemon.

### Inspect health and diagnostics

```bash
./bin/davctl version --json
./bin/davctl status
./bin/davctl --json status
./bin/davctl doctor
./bin/davctl --json doctor
./bin/davctl logs --limit 50
./bin/davctl logs --level ERROR --component caddy
```

`doctor` returns a non-zero exit code when the overall report fails. Logs are
bounded and sanitized. `davctl logs --follow` is currently unsupported.

### Manage the server and ports

```bash
./bin/davctl server status
./bin/davctl server start
./bin/davctl server stop
./bin/davctl server restart
./bin/davctl server settings
./bin/davctl server ports --http 8080 --https 8443
```

These commands manage the daemon-owned Caddy runtime. They do not install or
control the operating-system service.

### Manage users

Prefer stdin or the interactive no-echo prompt for passwords:

```bash
printf '%s\n' 'use-a-secret-from-a-secure-input' | \
  ./bin/davctl user add alice --password-stdin
./bin/davctl user list
./bin/davctl user passwd alice
./bin/davctl user enable alice
./bin/davctl user disable alice
./bin/davctl user delete alice
```

Do not put passwords in command-line arguments. Disabling or deleting a user
does not delete files in any share.

### Manage shares and ACLs

Share paths must be absolute. Metadata operations do not remove physical files.

```bash
./bin/davctl share add Documents /srv/davdeck/documents
./bin/davctl share list
./bin/davctl acl set Documents alice read-write
./bin/davctl acl set Documents bob read
./bin/davctl acl list Documents
./bin/davctl share update Documents --disable
./bin/davctl share remove Documents
```

Permission meanings:

- `none`: the user cannot use the share;
- `read`: listing and reads are allowed, mutations are denied;
- `read-write`: reads and expected write operations are allowed.

### Configure HTTPS

Inspect the current profile and validate certificate files before applying:

```bash
./bin/davctl tls show
./bin/davctl tls check
```

Internal HTTPS is useful for a private network when clients can trust the
generated local CA:

```bash
./bin/davctl tls internal dav.local
```

Use a certificate and private key supplied by your organization or certificate
authority for custom HTTPS:

```bash
./bin/davctl tls custom \
  --hostname dav.example.test \
  --cert /etc/davdeck/tls/fullchain.pem \
  --key /etc/davdeck/tls/privatekey.pem
```

Automatic/public HTTPS is delegated to Caddy:

```bash
./bin/davctl tls automatic dav.example.com
```

Public ACME issuance requires a publicly usable challenge path or a supported
DNS challenge provider. DavDeck does not currently integrate DNS provider
credentials. For a LAN deployment, use internal/custom certificates or put
DavDeck behind a reverse proxy or port-forwarding setup that handles public
certificate issuance.

### Apply and restore configuration

```bash
./bin/davctl config status
./bin/davctl config validate
./bin/davctl config apply
./bin/davctl revision list
./bin/davctl revision restore REVISION_ID
```

User, share, ACL, and TLS workflows normally trigger the appropriate apply
path. The explicit apply commands are useful after imports or operational
changes.

Export a safe backup without overwriting an existing file:

```bash
./bin/davctl config export --output davdeck-backup.yaml
```

Import is transactional and changes desired state only:

```bash
./bin/davctl config import davdeck-backup.yaml
./bin/davctl config validate
./bin/davctl config apply
```

Exports do not contain plaintext passwords, the Management API token, TLS
private keys, or DNS credentials. Newly imported users may need a password set
with `davctl user passwd`.

### Manage the Linux system service

Service commands are forwarded to the Linux systemd adapter owned by `davd`:

```bash
./bin/davctl service status
./bin/davctl service install
./bin/davctl service start
./bin/davctl service stop
./bin/davctl service uninstall
```

Administrator privileges may be required. Do not run the GUI permanently as
root or Administrator. Windows and macOS desktop builds do not currently
provide native service installation. Their close button hides the GUI in the
tray or menu bar; on macOS it also hides the Dock icon. Choose Exit there to
stop the GUI-owned daemon.

## 6. Platform notes

### macOS ARM64

The native GUI is the primary desktop validation target. Preview applications
are unsigned and may require explicit user approval in Privacy & Security.
Use a custom certificate or internal HTTPS when the server is local-only.

### Windows x64

The daemon, CLI, and GUI are release targets, but GUI behavior and Windows
reparse-point/junction confinement remain manual validation work. Before using
Windows for sensitive data, test the actual share paths on the intended Windows
version. The close button hides the GUI in the notification-area tray; choose
Exit from the tray menu to stop it.

### Linux x64 and ARM64

Use the headless archive over SSH. No Flutter or desktop session is required.
Keep the daemon's data, config, and runtime directories on suitable local
storage and use the Linux systemd service adapter only after reviewing the
required privileges.

## 7. Data, backup, and upgrade safety

The SQLite database is the authoritative application state. Before upgrades:

1. Stop the daemon cleanly.
2. Back up the data directory, config directory, and any custom TLS material.
3. Keep a copy of the release archive and its checksum.
4. Start the new daemon and run `davctl doctor` and `davctl config status`.
5. Confirm WebDAV reads and writes with a non-production test client.

Removing a user, share, service registration, or application metadata does not
delete user files. Physical data deletion must be performed separately and
deliberately by the administrator.

## 8. Troubleshooting

### `DAEMON_DISCOVERY_FAILED`

Confirm that `davd` is running, that `davctl` uses the same platform profile,
and that the endpoint file and token file belong to the same instance. For a
portable instance, pass `--endpoint` and `--token-file` explicitly.

### `PRIVILEGE_REQUIRED`

The requested service or protected filesystem operation needs administrator
privileges. Re-run the narrowly scoped operation through the platform's normal
elevation flow; do not run the whole GUI as administrator/root.

### Configuration apply fails

Run:

```bash
./bin/davctl config validate
./bin/davctl doctor
./bin/davctl logs --level ERROR
```

Check that share paths are absolute and readable, listener ports are free, and
custom certificate files are readable. Do not edit the generated Caddy JSON by
hand; correct the DavDeck state and apply again.

### A WebDAV client rejects HTTPS

Check the hostname, certificate chain, client trust store, and the configured
HTTPS port. Internal certificates require trust installation on each client.
For custom certificates, confirm that the certificate covers the hostname and
that the private key matches the certificate.

## 9. Security reporting

Do not publish unpatched vulnerability details in a public issue. Read
[`SECURITY.md`](../SECURITY.md) for the current reporting status. A private
repository reporting channel must be configured before treating the public
repository as ready for regular security disclosures.
