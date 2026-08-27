# Packaging

`scripts/package_release.sh` builds the Go daemon/client, bundles the exact
pinned Caddy runtime, records version metadata, creates a deterministic archive,
and writes a sidecar SHA-256 checksum.

All archives use this common root layout:

```text
DavDeck-<version>-<target>/
  bin/davd[.exe]
  bin/davctl[.exe]
  libexec/caddy[.exe]
  DavDeck.exe              # Windows desktop target
  flutter_windows.dll      # Windows desktop target
  data/                     # Windows Flutter runtime data
  desktop/                  # macOS/Linux desktop bundle targets
  manifest.json
  README.md
  README.zh-CN.md
  LICENSE
  NOTICE
  THIRD_PARTY_NOTICES.md
  third_party/
  SECURITY.md
```

Run `make release-package VERSION=0.1.0-rc.1 TARGET=linux-amd64`. Supported
targets are `darwin-arm64`, `windows-amd64`, `linux-amd64`, and
`linux-arm64`. Release CI supplies the target-native Flutter bundle for the
three desktop targets; Linux ARM64 remains headless.

For a directly runnable macOS application bundle, run:

```text
make macos-app VERSION=1.0.0 OUTPUT_DIR=dist
```

This creates `dist/DavDeck.app` with the native GUI and the pinned `davd`,
`davctl`, and Caddy binaries under `Contents/Resources/DavDeck/bin`. On first
launch the App starts its bundled daemon; all persistent settings remain in the
user's Application Support directory rather than inside the App bundle.

Artifacts are currently unsigned release-candidate archives. Signing,
notarization, native installers, and publication are intentionally separate
release-operator steps until credentials and platform policies are configured.
