# Local Development Guide

## 1. Primary environment

Primary developer host is macOS Apple Silicon.

The project should be structured so most development and debugging happens locally on macOS while CI provides native Linux/Windows coverage.

## 2. Toolchains

Expected categories:

- current pinned Go toolchain
- current pinned Flutter stable toolchain
- Caddy/xcaddy build toolchain as defined by repository scripts
- SQLite runtime/library as chosen by Go implementation
- Git

The Phase 0 baseline is Go 1.26.5 and Flutter 3.44.4. CI pins both versions;
`core/go.mod` and `gui/pubspec.lock` pin language/package requirements.

## 3. Local development loop

Typical flow:

```text
edit core
-> gofmt
-> go vet/test
-> start davd in development mode
-> exercise davctl
-> build/run pinned Caddy runtime
-> run WebDAV integration tests
-> run Flutter macOS GUI
```

## 4. Development runtime directories

Development mode should support explicit temporary/local directories so tests and developer runs do not touch production user data.

Example logical overrides:

- data dir
- config dir
- runtime dir
- Caddy binary path
- management port/token path

Do not hardcode developer home paths into source.

## 5. Test Caddy

Repository scripts provide a repeatable command to build the exact pinned Caddy + caddy-webdav runtime for local tests.

The test runtime must match release dependency pins whenever integration behavior matters.

The pinned runtime is now built with `make caddy-build` and verified with
`make caddy-verify`. The default output is ignored at `core/bin/caddy`; pass an
explicit path to `scripts/build_caddy.sh` when producing a temporary artifact.

## 6. Running daemon and CLI

Repository development commands:

```text
make check
make core-test
make core-build
make gui-build-macos
make smoke
```

`make smoke` starts `davd` with temporary data/config/runtime directories and
queries it with `davctl status`. It does not touch production user data.

## 7. Flutter workflow

Primary local GUI target:

- macOS ARM64

Run Flutter analyze/tests before GUI task completion. Native Windows/Linux desktop builds should be delegated to CI rather than forced through unsupported cross-build paths from macOS.

## 8. Debugging philosophy

Prefer observable structured errors and diagnostics over manually inspecting generated configuration.

Development mode may expose additional debug information, but secrets must still be redacted.

## 9. Local data safety

Tests must use temporary share roots. Never point automated destructive WebDAV tests at a developer's real Documents/Photos directories.

## 10. Adding a feature

Before coding:

1. identify domain model change
2. identify DB migration need
3. identify API change
4. identify CLI/GUI exposure
5. identify Caddy/compiler impact
6. identify platform impact
7. identify security impact
8. write/update tests

Follow `CONTRIBUTING.md` and the relevant public technical documentation for
repository rules.
