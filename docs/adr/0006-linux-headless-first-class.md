# ADR-0006: Linux Headless Is a First-Class Product Target

Status: Accepted

## Context

WebDAV servers commonly run on VPSs, home servers, NAS-like machines, SBCs, and systems without a desktop environment.

## Decision

All core server features must be operable with `davd + davctl` on Linux without Flutter, X11, Wayland, a browser, or WebView.

## Consequences

- GUI-only configuration of backend features is not acceptable.
- CLI/API capability should be implemented alongside backend functionality.
- Linux x64 and Linux ARM64 server builds are high-priority release targets.
