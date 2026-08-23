# Changelog

All notable changes to this project will be documented here.

## 0.1.0-rc.1

This release candidate is an unsigned preview and is not a stable `1.0`
release.

- Provide the Go daemon, CLI, native desktop client, authenticated loopback
  Management API, SQLite state, users, shares, ACLs, TLS workflows, service
  adapters, diagnostics, logs, revisions, and safe YAML exchange.
- Pin and validate the Caddy plus caddy-webdav runtime, including runtime
  lifecycle and WebDAV authentication/ACL integration coverage.
- Validate deterministic release archives, build metadata, manifests, and
  SHA-256 checksums for the supported preview targets.
- Publish bilingual README and user-guide materials, including the AI-assisted
  development disclosure and the current platform limitations.
- Defer Windows GUI validation, signing/notarization, reboot persistence, DNS
  provider ACME integration, and stable `1.0` release approval.

## Unreleased

- Bootstrap the Phase 0 DavDeck monorepo, daemon, CLI, Flutter client, SQLite
  migration framework, authenticated loopback API, tests, and CI baseline.
- Add Task 002 pure domain primitives, enums, validation errors, and unit tests.
- Add SQLite-backed users, shares, ACLs, TLS profiles, desired/active revisions,
  and safe transactional YAML import/export.
- Add authenticated management API and `davctl` workflows, native Flutter
  users/shares/TLS/diagnostics views, and native service-manager adapters.
- Pin and verify the Caddy+caddy-webdav runtime with deterministic config,
  validate-before-apply, runtime lifecycle, and real WebDAV ACL integration.
- Add deterministic release-candidate archives, build metadata, SHA-256
  checksums, native/cross-platform CI matrices, and platform smoke tooling.
- Record 1.0 acceptance blockers, including WebDAV symlink/junction confinement,
  incomplete service/client workflows, signing/installers, and private security
  reporting.
