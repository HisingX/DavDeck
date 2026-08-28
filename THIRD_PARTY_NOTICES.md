# Third-Party Notices

DavDeck is licensed under Apache License 2.0. This document and the bundled
`third_party/` directory identify licenses and notices that apply to the
redistributable dependencies of the current preview.

Third-party code is not relicensed by DavDeck. Each component remains subject
to its own license terms and notices.

## Bundled components

| Component | Pinned version or source | License |
| --- | --- | --- |
| Caddy | `v2.11.4` | Apache-2.0 |
| caddy-webdav | `v0.0.0-20260127042217-fa2f366b0d75` with DavDeck changes | Apache-2.0 |
| DavDeck core dependencies | `core/go.mod` and `core/go.sum` | See `third_party/license-reports/go-core.csv` |
| Caddy/WebDAV dependency graph | `caddy/caddy-webdav/go.mod` and `go.sum` | See `third_party/license-reports/go-caddy-webdav.csv` |
| Flutter GUI | Flutter SDK `3.44.4`, `gui/pubspec.lock` | Flutter SDK license and lockfile are included under `third_party/` |

## Generated material

`scripts/generate_third_party_notices.sh` uses the fixed
`github.com/google/go-licenses/v2@v2.0.1` tool to generate:

- CSV dependency/license reports under `third_party/license-reports/`;
- license texts, copyright notices, and required source material under
  `third_party/licenses/`;
- the resolved Flutter package lockfile and local Flutter SDK license when the
  SDK is available.

The local DavDeck modules are intentionally excluded from the Go reports
because their Apache-2.0 license is the repository-root `LICENSE` file.

## Release review requirements

Before publishing a binary release, review every warning or unknown item in the
generated reports. In particular:

- inspect warnings about non-Go code and platform-specific source files;
- preserve the included source bundle for the reported MPL-2.0 dependency
  `github.com/go-sql-driver/mysql`;
- verify that the exact Caddy binary was built from the pins in
  `caddy/versions.env` and includes the intended WebDAV module;
- verify the Flutter desktop bundle's notices on each released desktop target;
- refresh `third_party/` whenever a Go module, Caddy/WebDAV pin, Flutter SDK,
  or Dart package lock changes.
