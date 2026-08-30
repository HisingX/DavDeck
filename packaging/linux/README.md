# Linux packaging

Linux release archives are split into explicit flavors:

- `linux-amd64-server`: headless systemd installation with `install.sh`;
- `linux-arm64-server`: headless systemd installation with `install.sh`;
- `linux-amd64-desktop`: runnable GUI launcher at the archive root.

The Server installer uses `/opt/davdeck` for programs, `/var/lib/davdeck` for
data, `/etc/davdeck` for configuration, and `/run/davdeck` for the temporary
management endpoint. It preserves data and configuration on uninstall. Native
`.deb`/`.rpm` packages remain post-1.0 packaging work.
