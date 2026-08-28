# DavDeck

DavDeck is an open-source, cross-platform WebDAV server manager powered by
Caddy. It provides a native desktop application for macOS and Windows, plus a
headless daemon and CLI for Linux servers.

This repository is in the release-candidate preview phase. It is not a stable
`1.0` release. See the [GitHub Releases](https://github.com/HisingX/DavDeck/releases)
page for the exact release version and downloads. The exact version of an
archive is also recorded in its `manifest.json` and reported by `davctl
version --json`. Preview binaries are unsigned and should be used only after
reviewing the platform limitations and release notes.

## AI-assisted development disclosure

DavDeck was developed with substantial assistance from AI coding tools. AI was
used for design exploration, implementation, refactoring, documentation, and
test development. Human maintainers remain responsible for code review,
security decisions, dependency and license review, testing, and release
decisions.

## Features

- Manage WebDAV users, shares, and per-share `NONE`, `READ`, and `READ_WRITE`
  permissions.
- Run Caddy through a generated and validated runtime configuration.
- Use a local-only authenticated Management API shared by the GUI and CLI.
- Configure automatic, internal, or custom-certificate HTTPS.
- Manage daemon-owned server state, revisions, diagnostics, logs, and native
  service adapters.
- Import and export safe, versioned YAML configuration without exporting
  passwords, tokens, or private keys.
- Run Linux headless without Flutter or a desktop session.

DavDeck is not a Caddyfile editor. Users manage application state; DavDeck
compiles and operates Caddy for them.

## Preview targets

| Target | Current preview status |
| --- | --- |
| macOS ARM64 | Native GUI smoke-tested; unsigned binaries |
| Windows x64 | Build target available; GUI validation is deferred |
| Linux x64 | Headless daemon/CLI and HTTPS smoke-tested on native hardware |
| Linux ARM64 | Headless daemon/CLI and HTTPS smoke-tested on native hardware |

Windows GUI validation, installer polish, code signing, notarization, and
Linux service boot-persistence validation are intentionally deferred from this
preview. See [Known Limitations](docs/KNOWN_LIMITATIONS.md).

## Quick start from source

Requirements are pinned in the project documentation. The core requires Go;
the desktop client additionally requires Flutter.

```bash
make core-build caddy-build
./core/davd \
  --caddy-binary ./core/bin/caddy \
  --data-dir ./data --config-dir ./config --runtime-dir ./run
```

In another terminal, use the CLI through the daemon's loopback Management API:

```bash
./core/davctl version --json
./core/davctl status
./core/davctl doctor
```

For a packaged installation, use the target archive's `bin/davd` and
`bin/davctl` binaries. The complete workflow, including GUI setup, service
management, HTTPS, backups, and CLI automation is in the
[User Guide](docs/USER_GUIDE.md), with a [Chinese version](docs/USER_GUIDE.zh-CN.md).

## Documentation

- [User Guide](docs/USER_GUIDE.md) · [用户手册](docs/USER_GUIDE.zh-CN.md)
- [Known Limitations](docs/KNOWN_LIMITATIONS.md)
- [Project specification](docs/PROJECT_SPEC.md)
- [Architecture](docs/ARCHITECTURE.md)
- [CLI reference](docs/CLI.md)
- [Platform notes](docs/PLATFORM.md)
- [Security design](docs/SECURITY.md) · [Security policy](SECURITY.md)
- [Release process](docs/RELEASE.md)
- [Changelog](CHANGELOG.md)

Internal development workflow notes and local validation records are not part
of the public user documentation.

## Development

```bash
make check
make caddy-tooling-test
make caddy-module-test
make release-packaging-test
```

Run the applicable native platform checks before making a release claim. See
the testing and release documentation for prerequisites and limitations.

## License

DavDeck is licensed under the [Apache License 2.0](LICENSE). Third-party
components retain their own licenses and attribution requirements; see
[NOTICE](NOTICE) and the relevant upstream project metadata.
