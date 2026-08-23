# ADR-0002: Use Go for `davd` and `davctl`

Status: Accepted

## Context

The project needs a cross-platform daemon, CLI, process/service integration, SQLite access, and close compatibility with the Caddy ecosystem.

## Decision

Implement the core daemon (`davd`) and CLI (`davctl`) in Go.

## Consequences

- Shared language/ecosystem with Caddy tooling.
- Straightforward cross-platform builds for core binaries.
- OS-specific behavior still requires native adapters and tests.
- Business logic belongs in `davd`, not duplicated in CLI.
