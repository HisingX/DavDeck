# DavDeck Linux Desktop

This archive is the Linux x64 Desktop flavor. Keep the archive layout intact
and run the launcher from its root directory:

```bash
./davdeck
```

The GUI includes the matching `davd` and Caddy binaries. If no local daemon is
already reachable, it starts its bundled portable daemon and shuts down only
that daemon when the GUI exits. An existing systemd daemon is treated as an
external daemon and is left running.

Verify the archive checksum and review `manifest.json` before running it.
