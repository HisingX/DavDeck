# Build, Packaging, and Release

## 1. Release principles

Releases must be reproducible, versioned, test-backed, and transparent about bundled Caddy/WebDAV versions.

Do not build release artifacts from unpinned dependencies.

## 1.1 Public source publication

The development repository contains private workflow and acceptance records
that are not part of the public source release. Do not push the development
branch directly to a public repository. Create a fresh sanitized public
repository, or a filtered clone with a new initial history, and verify that it
contains only the intended product and public technical documentation.

At minimum, exclude:

- `AGENTS.md`
- `docs/AI_WORKFLOW.md`
- `docs/tasks/`
- `docs/NEXT_PHASE_PLAN.md`
- `docs/ACCEPTANCE_1_0.md`
- `local/` and any untracked validation records

Before publication, search the sanitized tree for local hostnames, SSH aliases,
tokens, private paths, generated runtime files, and other credentials. Keep
the AI-assisted development disclosure in the public README, but do not publish
private AI task instructions or local development records.

The current snapshot helper is:

```bash
scripts/prepare_public_source.sh /tmp/DavDeck-public-source
```

The output directory must not already exist and must be outside the development
repository. Review the generated snapshot, initialize a new Git repository in
it, and configure the public remote only after the manual license and secret
review is complete.

## 2. Versioning

Use semantic versioning unless the project later adopts another documented scheme.

During early development:

- `0.x.y` may contain breaking changes
- config/API format changes still require explicit migration notes

The README and user guides intentionally do not pin an individual release
candidate number. The exact release version is supplied by the release tag or
manual workflow input, recorded in `CHANGELOG.md`, and embedded in each
archive's `manifest.json` and `davctl version --json` output. Updating a README
or user guide is therefore not required for every RC or patch release unless
the user-facing behavior or release status changes.

## 3. Version metadata

Each release should expose:

- DavDeck version
- Git commit
- build date if reproducible-build policy allows
- Go version
- Flutter version
- Caddy version
- caddy-webdav version/commit
- target OS/architecture

`davctl version` and diagnostics should report this information without secrets.

Release builds inject these values through Go linker flags from pinned source
configuration. `davctl version --json` works without daemon discovery. Every
archive also contains `manifest.json` with the same release/target dependency
metadata and an explicit unsigned state.

## 4. Target artifacts

Initial targets:

### Desktop

- macOS ARM64 package (DMG or appropriate signed app package)
- Windows x64 installer/package
- Linux x64 desktop package where maintained

### Server/headless

- Linux x64 archive/package
- Linux ARM64 archive/package

### CLI

Standalone binaries may be published where useful.

## 5. Caddy build

CI builds a pinned custom Caddy containing the required WebDAV module.

Store version pins in source-controlled build configuration rather than downloading `latest`.

Before release, verify module presence and run the WebDAV integration suite against the exact produced binary.

## 6. CI stages

Recommended pipeline:

1. lint/format
2. unit tests
3. integration tests
4. native platform builds
5. package creation
6. smoke tests where possible
7. checksum generation
8. signing/notarization when configured
9. release publication

The `Release Candidate` workflow implements stages 1–7 for `v*-rc.*` tags and
manual dispatch. It builds on target-native GitHub-hosted runners, with Linux
ARM64 as a headless native job, and uploads archives plus an aggregate
`SHA256SUMS` artifact. Publication, signing, and notarization are deliberately
not automatic.

## 7. macOS

Release planning should account for:

- ARM64 primary build
- application bundle
- code signing
- notarization for polished public distribution
- menu-bar close/Exit behavior
- TCC-related user guidance

Unsigned developer builds can exist before signing infrastructure is configured, but public release status must be clearly labeled.

## 8. Windows

Release planning should account for:

- x64 package
- notification-area close/Exit behavior
- code signing when infrastructure is available
- upgrade behavior
- data preservation

## 9. Linux

Start with portable tar archives if needed, then add native packages.

Long-term package targets may include:

- `.deb`
- `.rpm`
- package repository metadata

Service package should install systemd unit and preserve user configuration/data on normal uninstall according to packaging conventions.

## 10. Checksums and signatures

Every release artifact should have SHA-256 checksums.

`scripts/package_release.sh` creates one `.sha256` sidecar per archive. Archives
normalize entry ordering and timestamps using `SOURCE_DATE_EPOCH`; CI defaults
that value to the tagged commit timestamp through the packaging script.

When release signing is implemented, document verification steps.

Auto-update must not be introduced until downloaded artifacts can be authenticated/verified safely.

## 11. Upgrade behavior

Upgrades must:

- preserve SQLite/data by default
- run migrations explicitly
- keep backups/checkpoints where appropriate
- fail safely on migration errors
- not silently reset configuration

Caddy binary/module upgrade compatibility should be tested before publication.

## 12. Release checklist

- [ ] Version updated
- [ ] CHANGELOG prepared
- [ ] Dependency pins reviewed
- [ ] Caddy/WebDAV versions recorded
- [ ] Unit tests pass
- [ ] Integration tests pass
- [ ] WebDAV ACL tests pass
- [ ] Symlink/junction confinement release gate passes
- [ ] Database migration tests pass
- [ ] macOS build/smoke complete
- [ ] Windows build/smoke complete
- [ ] Linux x64 build/smoke complete
- [ ] Linux ARM64 build/smoke complete or limitation documented
- [ ] Packages contain expected files only
- [ ] Public source tree/history is sanitized and secret-scanned
- [ ] Third-party dependency license inventory and notices are complete
- [ ] Diagnostic/version output correct
- [ ] Checksums generated
- [ ] Signing/notarization complete when enabled
- [ ] Release notes include breaking changes/migrations
- [ ] SECURITY/known limitations reviewed

## 13. Package-manager roadmap

After stable manual release artifacts:

- Homebrew
- winget
- Debian/RPM distribution

Do not block MVP on package-manager publication.
