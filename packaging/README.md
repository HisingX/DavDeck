# Packaging

`scripts/package_release.sh` builds the Go daemon/client, bundles the exact
pinned Caddy runtime, records version metadata, creates a deterministic archive,
and writes a sidecar SHA-256 checksum.

Linux archives now have explicit Server and Desktop flavors. macOS and Windows
retain their existing target names; their manifest flavor is `desktop`.

Linux Server archives use this root layout:

```text
DavDeck-<version>-<target>/
  bin/davd[.exe]
  bin/davctl[.exe]
  libexec/caddy[.exe]
  install.sh               # Linux Server only
  uninstall.sh             # Linux Server only
  systemd/                 # Linux Server only
  manifest.json
  README.md
  README.zh-CN.md
  LICENSE
  NOTICE
  THIRD_PARTY_NOTICES.md
  third_party/
  SECURITY.md
```

Linux Server targets are `linux-amd64-server` and `linux-arm64-server`; the
Linux x64 Desktop target is `linux-amd64-desktop`. For example:

```text
make release-package VERSION=1.0.0 TARGET=linux-amd64-server
make release-package VERSION=1.0.0 TARGET=linux-amd64-desktop
```

Release CI supplies the target-native Flutter bundle for the desktop targets;
Linux ARM64 remains Server-only. The legacy `linux-amd64` and `linux-arm64`
names remain accepted for local compatibility, but new release automation must
use an explicit flavor.

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
