# ADR-0001: Use Flutter for the Native Desktop GUI

Status: Accepted

## Context

DavDeck needs a cross-platform desktop GUI for macOS, Windows, and optionally Linux desktop. The project explicitly avoids WebView-based application shells.

## Decision

Use Flutter Desktop for the GUI.

The GUI is a management client only and communicates with `davd` through the local Management API.

## Alternatives considered

- Electron: rejected because it embeds a web runtime and conflicts with product constraints.
- Tauri/Wails: rejected for the same WebView-based UI concern.
- Fyne: attractive because it is Go-native, but Flutter is preferred for richer cross-platform desktop UX and separation from backend runtime.
- Qt: mature, but heavier toolchain/licensing/deployment considerations for this project.

## Consequences

- Two primary languages/toolchains: Go and Dart/Flutter.
- Flutter desktop builds should run on target OS CI runners.
- GUI business logic must not duplicate backend logic.
