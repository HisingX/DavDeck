# Platform Support and OS Integration

## 1. Development strategy

The primary local development environment is macOS ARM64. Cross-platform correctness is achieved through strict platform abstraction plus native CI jobs on macOS, Linux, and Windows.

Developers should not need three physical machines for daily work.

## 2. Support tiers

### Tier 1

- macOS ARM64: daemon, CLI, GUI
- Windows x64: daemon, CLI, GUI
- Linux x64: daemon, CLI; GUI supported where practical
- Linux ARM64: daemon, CLI

### Later / best effort

- Windows ARM64
- macOS Intel
- Linux ARM64 GUI
- other Unix platforms

## 3. Platform boundary

OS-specific code belongs under platform packages/files.

Examples:

```text
platform/
  paths_darwin.go
  paths_linux.go
  paths_windows.go
  service_darwin.go
  service_linux.go
  service_windows.go
  privilege_darwin.go
  privilege_linux.go
  privilege_windows.go
```

Use build constraints where appropriate.

Do not spread `runtime.GOOS` checks through application/domain code.

## 4. Standard path abstraction

Expose logical locations rather than hardcoding paths:

- DataDir
- ConfigDir
- RuntimeDir
- LogDir
- BundledBinaryDir
- ManagementTokenPath

Tests should be able to inject temporary directories.

The exact OS-specific locations should follow native conventions and be documented in packaging code.

## 5. macOS

### Service manager

Desktop GUI service installation is intentionally deferred. The macOS GUI runs
DavDeck in portable mode and remains available from the menu bar after its
window is closed. Native launchd support can be added after the Linux headless
service flow is stable.

### Primary development workflow

- Go unit/integration tests locally
- Flutter macOS native run/build
- real local Caddy/WebDAV smoke tests
- Finder or command-line WebDAV client checks as supplemental manual testing

### Filesystem/privacy considerations

macOS privacy/TCC can affect access to Desktop, Documents, Downloads, external drives, or other protected locations.

Diagnostics should convert low-level permission errors into clear guidance without attempting broad permission changes automatically.

The GUI should not permanently run as root.

## 6. Linux

### Headless guarantee

No desktop environment is required.

Server installation should only require runtime components such as:

- `davd`
- `davctl`
- bundled/custom Caddy runtime

### Service manager

Use systemd for supported mainstream Linux distributions.

This is the only supported native system-service integration in the current
milestone. Manage it through `davctl`/`davd`; no desktop session is required.

Linux Server release archives use the following installed layout:

```text
/opt/davdeck/bin/davd
/opt/davdeck/bin/davctl
/opt/davdeck/libexec/caddy
/usr/local/bin/davctl -> /opt/davdeck/bin/davctl
/etc/davdeck/
/var/lib/davdeck/
/run/davdeck/
```

The archive's `install.sh` creates the systemd unit and enables it with
`systemctl enable --now`. `RuntimeDirectory=davdeck` recreates `/run/davdeck`
after reboot. The CLI discovers the installed endpoint/token automatically;
the system service and desktop XDG paths remain separate. Do not assume these
paths in domain/application code; resolve them through platform configuration.

### Permissions

Avoid automatically recursively chmod/chown on user shares. Report missing permissions and let the user choose a safe remediation.

### Linux ARM64

Treat ARM64 headless as important for home servers, NAS-like devices, SBCs, and ARM VPS environments.

## 7. Windows

### Service manager

Windows Service Control Manager integration is intentionally not exposed in the
current desktop milestone. The Windows GUI runs portable mode, stays resident
in the notification-area tray after its window is closed, and exits only from
the tray menu. SCM support remains deferred until service startup and recovery
are validated end to end.

### Path cases

Test:

- drive-letter paths
- spaces
- Unicode
- long-ish paths
- path case behavior
- UNC/network paths if/when support is claimed

MVP may explicitly mark UNC/network-share support as limited until integration tests exist.

### Secrets and ACLs

Management token and sensitive local files should be protected using Windows ACLs appropriate to the service/user identity.

### GUI/runtime process model

The GUI is not the server. In portable mode, the native runner owns only the
bundled daemon process it started. Closing the window hides the GUI and keeps
that daemon and its Caddy child running; choosing Exit from the tray menu
gracefully shuts them down.

## 8. Privilege elevation

Abstract privileged operations.

Examples:

- install/uninstall the Linux system service
- write protected service configuration
- bind privileged ports depending on runtime model
- local certificate trust installation if later automated

Elevation should be scoped to the operation, not the whole GUI session.

## 9. Build strategy

### Go core/CLI

Use native CI jobs as the primary runtime-confidence path. Cross-compilation is useful as an additional build check but is not proof of runtime correctness.

### Flutter desktop

Build on target OS CI runners:

- macOS runner -> macOS build
- Windows runner -> Windows build
- Linux runner -> Linux build

Do not make macOS cross-compile the Windows Flutter desktop app.

### Caddy

Build the pinned Caddy + caddy-webdav combination in reproducible CI jobs for each target architecture. Prefer target-native builds where practical.

## 10. Platform test levels

### Every PR

- Go unit tests on all three OS families where possible
- Go build for supported targets
- Flutter analyze/test
- target desktop build where practical
- Caddy config/compiler tests

### Release candidate

- manual macOS ARM64 smoke test
- Windows x64 smoke test
- Linux x64 headless smoke test
- Linux ARM64 smoke test via physical/VM/remote runner when available

### Later self-hosted coverage

Self-hosted Windows/Linux runners can be introduced when testing system restart, real service installation, or hardware-specific behavior becomes important.
