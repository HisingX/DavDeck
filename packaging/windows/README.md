# Windows packaging

The Windows x64 release-candidate ZIP contains the Flutter runner directory,
`davd.exe`, `davctl.exe`, and the pinned `caddy.exe`. Launching the packaged
`desktop/Release/davdeck.exe` automatically starts the bundled daemon and
passes it the bundled Caddy path. In portable mode, the GUI owns the daemon it
starts and gracefully shuts it down when the GUI exits; an installed Windows
service remains independent and is not stopped by closing the GUI. The daemon
uses the normal per-user data locations.

The ZIP is not yet an MSI or signed installer; Windows Service installation
remains an explicit management action.
