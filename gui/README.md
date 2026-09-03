# DavDeck GUI

Native Flutter management client for the local DavDeck daemon. The GUI remains
an API client and does not access SQLite or manage Caddy directly.

Run `flutter analyze`, `flutter test`, and the native `flutter build <target>`
command for the current operating system.

## Locale testing

The desktop app follows the operating-system locale by default. For repeatable
development testing, force a supported locale at launch without changing the
machine language:

```bash
flutter run -d macos --dart-define=DAVDECK_LOCALE=en
flutter run -d macos --dart-define=DAVDECK_LOCALE=zh-CN
```

Omit `DAVDECK_LOCALE` for normal system-locale behavior. The override also
localizes the macOS/Windows tray menu for that launch.
