# DavDeck Linux Server

This archive is the headless Linux Server flavor. It includes `davd`, `davctl`,
and the pinned Caddy runtime; Go, Flutter, and a separately installed Caddy are
not required.

Install the system service from the extracted directory:

```bash
sudo ./install.sh
davctl
```

The installer checks the host architecture, installs the programs under
`/opt/davdeck`, creates the systemd service, starts it, and performs a local
management API smoke check. The service uses `/var/lib/davdeck` for data,
`/etc/davdeck` for configuration, and `/run/davdeck` for its temporary
endpoint. Management remains loopback-only.

To remove the programs while preserving configuration and data:

```bash
sudo ./uninstall.sh
```

Verify the archive checksum and review `manifest.json` before installation.
