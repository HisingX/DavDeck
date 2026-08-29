# macOS packaging

The macOS ARM64 release-candidate archive contains the native `DavDeck.app`
bundle plus headless management binaries and the pinned Caddy runtime. The
release packager mirrors the runtime into the app bundle so launching
`DavDeck.app` from the archive also starts its bundled daemon. It is an
portable GUI runtime: the app gracefully shuts down the daemon it started when
the app exits. A separately installed launchd service remains independent.
It is an unsigned tar archive, not yet a DMG. The initial `1.0.0` distribution
does not use a trusted Developer ID signature or notarization, so Gatekeeper may
show a warning. Users should verify the published SHA-256 checksum and
`manifest.json` before opening it.

`make macos-app VERSION=<version>` creates a standalone `DavDeck.app` for
local distribution. The bundle contains `davd`, `davctl`, and the pinned Caddy
runtime. Its native startup code launches `davd` with the bundled Caddy binary
and waits for the loopback management API to become available. Persistent
database/configuration data is deliberately stored outside the bundle at
`~/Library/Application Support/DavDeck/`, so replacing the App during an
upgrade does not replace user configuration.
