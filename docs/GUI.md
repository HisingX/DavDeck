# Desktop GUI Design

## 1. Product role

The Flutter GUI is a native management client for `davd`. It is not the WebDAV server and must not own backend business logic.

No WebView/browser UI is used.

## 2. Supported targets

Primary:

- macOS ARM64
- Windows x64

Secondary:

- Linux x64 desktop where maintained

Linux headless does not require the GUI.

## 3. Information architecture

Recommended primary navigation:

- Dashboard
- Users
- Shares
- HTTPS
- Logs
- Diagnostics
- About
- Revisions
- Settings

## 4. Dashboard

Show:

- daemon state
- Caddy/WebDAV health
- primary WebDAV URL
- HTTPS status
- user count
- share count
- desired vs active config state
- uptime

Actions:

- Start
- Stop
- Restart
- Apply pending configuration
- Copy WebDAV URL
- Open diagnostics/logs
- Apply pending configuration and show the resulting revision

In the desktop GUI's portable mode, opening DavDeck does not automatically
start the Caddy/WebDAV runtime. When the daemon and database are ready but the
runtime is stopped, the Dashboard shows a neutral “Service not started” state,
explains that the service is not yet available, and provides a direct start
action. This state is distinct from a failed or unreachable service; the
sidebar uses the same status summary and provides the start hint as well.

Runtime controls follow the active Caddy state. Start is available only when
the runtime is stopped or has failed and is available for retry; Stop and
Restart are available when the runtime is running or degraded. All runtime
controls are disabled while a start/stop transition is in progress or while
the state is unknown.

The Dashboard must distinguish configured ports from active, reachable
endpoints. It should not present the reserved HTTPS port as an available URL
when HTTPS is unconfigured, pending, or failed. Endpoint URLs use the active
TLS hostname rather than a hard-coded `localhost` value. Saving, disabling, or
applying TLS must refresh the shared dashboard state; pending configuration and
endpoint failures must also be reflected in the global/sidebar status.

## 5. Users

List fields:

- username
- enabled/disabled
- number of accessible shares

Actions:

- add
- change password
- enable/disable
- delete

Never display password hash.

Password forms clear sensitive input after success/failure handling.

## 6. Shares

List fields:

- name
- path
- URL/path slug
- enabled state
- permitted user count

Detail includes per-user permission editor.

Deleting a Share must clearly say physical files are preserved.

## 7. Permission UX

MVP values:

- No access
- Read only
- Read & write

A matrix UI may be added when user/share counts justify it, but a per-share list is acceptable for MVP.

## 8. HTTPS wizard

Step 1: choose mode.

- Automatic public HTTPS
- Internal/LAN HTTPS
- Custom certificate

Then show only fields relevant to the selected mode.

The HTTPS page provides an explicit “Disable HTTPS” action. Disabling removes
the desired TLS profile and requires Apply before the runtime returns to
HTTP-only mode.

HTTPS actions distinguish unsaved form changes from saved-but-pending changes.
Save is the primary action only while the form differs from the saved desired
profile. When a configuration is waiting to be applied, Apply configuration is
the primary action and Save is secondary; Apply remains disabled until any new
form edits are saved. With no changes, Save is disabled and no Apply action is
shown. The page restores pending-apply state from the shared daemon status.

Preflight checks should be presented as actionable statuses, not raw Caddy errors where a safer explanation is possible.

For automatic public HTTPS, the page also shows the effective Caddy storage
directory, the public certificate file path, the current issuance phase, and the
certificate expiry time when available. ACME issuance is asynchronous, so Apply
returns when the runtime accepts the configuration; the page then polls while
the certificate is being issued. A failed or expired certificate provides a
direct link to Logs. Only public certificate metadata is shown; private-key
contents are not read or displayed.

When an automatic certificate is already issued or expired, the page provides a
“Renew certificate” action. After confirmation, DavDeck reuses the saved
challenge and DNS provider, asks the active Caddy TLS automation policy to
force a new certificate, and shows the renewal as an issuing operation. Renewal
does not change the TLS profile or create a configuration revision. While it is
running, “Cancel certificate renewal” is available; canceling preserves the
existing certificate and HTTPS configuration. A failed renewal remains visible
with a retryable failure state; wildcard renewal is currently excluded because
the force-renewal operation requires a concrete certificate name.

## 9. Desktop window lifecycle

The desktop GUI uses portable mode and does not install or manage a native
system service. The dashboard controls the DavDeck-owned Caddy/WebDAV runtime;
Linux Server installation is a headless `install.sh` workflow, while daily
system-service operations remain available through `davctl`.

On Windows, closing the window hides DavDeck in the notification-area tray. On
macOS, closing the window hides the Dock icon while leaving DavDeck running in
the menu bar. The tray or status-bar menu provides:

- Show DavDeck
- Exit DavDeck

Only Exit quits the GUI and its portable daemon. A fallback Dock click also
restores the macOS window if the Dock icon is still present. Do not conflate
“GUI window hidden” with “server stopped”.

## 10. Logs

Provide:

- level filter
- component filter
- newest-first paged records with timestamp, level, component, message, and
  expandable safe structured fields
- manual refresh and bounded 30-second auto-refresh with pause control
- copy of the sanitized JSON view
- export of the sanitized JSON view to a temporary file
- explicit loading, empty, unavailable, and refresh-error states

Do not expose secrets even in debug mode.

## 11. Diagnostics

Show checks with statuses:

- pass
- warning
- fail
- not applicable

Each result should contain:

- short title
- safe explanation
- remediation hint when known
- optional technical detail

Logs failure states provide a direct link back to Diagnostics.
Known stable error codes show a localized remediation hint; raw stack traces,
private keys, bearer tokens, and unrestricted filesystem paths remain hidden.

## 12. Revisions and configuration state

Show desired and active revision numbers, pending/dirty state, validation and
apply status, creation time, and the safe configuration hash. A complete
revision may be restored only after an explicit confirmation; restore brings
back users (including enabled/disabled state), shares, permissions, server/TLS
settings, and the generated Caddy runtime configuration together. Older
runtime-only revisions show why safe restore is unavailable instead of
pretending that restoring Caddy JSON also restores application state. An
unreferenced revision may be deleted after confirmation. Both operations are
routed through `davd`. Active and desired revisions must show why deletion is
unavailable. Starting, stopping, or restarting the server does not create a
revision. Raw generated Caddy JSON is not displayed.

## 13. Backup and upgrade safety

Settings must provide a visible data-safety notice: normal application upgrades
and uninstall operations preserve the DavDeck database and configuration by
default, while shared-directory files are never removed by configuration
operations. A future native installer/uninstaller must expose any application
data deletion as a separate, explicit choice.

The Settings page provides configuration export and import actions using the
safe versioned YAML format. Export uses a native save dialog. Import requires
confirmation, merges desired state transactionally, preserves physical files,
and explains that newly imported users need a new password and that the
configuration remains pending until Apply. Passwords, management tokens, TLS
private keys, and other secrets must not be included in the backup.

## 14. Advanced mode

Optional later advanced section may show:

- generated Caddy JSON
- runtime versions
- revision list
- raw sanitized logs

Do not expose advanced internals by default.

## 15. Architecture

Recommended Flutter layers:

```text
views/widgets
-> state/view models
-> repositories/facades
-> management API client
```

Widgets should not make raw HTTP requests directly.

## 16. Localization

Support at least:

- English
- Simplified Chinese

The current GUI follows the operating-system language when it is English or
Simplified Chinese. Other locales fall back to English. The About page also
shows the project address, license, and current language support.

Backend error code is stable; GUI maps it to localized text.

Do not hardcode user-facing strings across widgets.

## 17. Accessibility and desktop behavior

Use standard native-feeling desktop interaction patterns:

- keyboard navigation where practical
- clear focus states
- accessible labels
- copyable URLs/errors
- predictable confirmation dialogs for metadata-destructive actions

## 18. Error UX

Prefer:

- concise localized summary
- safe technical detail expandable/copyable
- suggested remediation

Never show raw stack traces or secrets.
