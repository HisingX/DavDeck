# Windows packaging

The Windows x64 release-candidate ZIP contains the Flutter runner files at the
archive root, `davd.exe`, `davctl.exe`, and the pinned `caddy.exe`:

```text
DavDeck.exe
flutter_windows.dll
data/
bin/davd.exe
bin/davctl.exe
libexec/caddy.exe
```

Launching `DavDeck.exe` automatically starts the bundled daemon and passes it
the bundled Caddy path. In portable mode, the GUI owns the daemon it starts
and gracefully shuts it down when the GUI exits; an installed Windows service
remains independent and is not stopped by closing the GUI. The daemon uses the
normal per-user data locations.

The ZIP is not yet an MSI or Authenticode-signed installer. The initial
`1.0.0` distribution may trigger a SmartScreen or unsigned-binary warning;
users should verify the published SHA-256 checksum and `manifest.json` first.
Windows Service installation remains an explicit management action.
