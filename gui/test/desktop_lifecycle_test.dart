import 'package:davdeck/desktop/desktop_lifecycle.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:tray_manager/tray_manager.dart' as tray;

void main() {
  test('window close hides the desktop window instead of quitting', () async {
    final calls = <String>[];
    final lifecycle = DesktopLifecycle(
      enabled: true,
      hideWindow: () async => calls.add('hide'),
      destroyTray: () async => calls.add('destroy-tray'),
      setPreventClose: (preventClose) async =>
          calls.add('prevent-close:$preventClose'),
      destroyWindow: () async => calls.add('destroy-window'),
      setSkipTaskbar: (skipTaskbar) async =>
          calls.add('skip-taskbar:$skipTaskbar'),
      isMacOS: true,
    );

    lifecycle.onWindowClose();
    await Future<void>.delayed(Duration.zero);

    expect(calls, ['hide', 'skip-taskbar:true']);
  });

  test('showing a hidden macOS window restores its Dock icon', () async {
    final calls = <String>[];
    final lifecycle = DesktopLifecycle(
      enabled: true,
      showWindow: () async => calls.add('show'),
      focusWindow: () async => calls.add('focus'),
      setSkipTaskbar: (skipTaskbar) async =>
          calls.add('skip-taskbar:$skipTaskbar'),
      isMacOS: true,
    );

    await lifecycle.showWindow();

    expect(calls, ['skip-taskbar:false', 'show', 'focus']);
  });

  test('tray Exit destroys the tray and window', () async {
    final calls = <String>[];
    final lifecycle = DesktopLifecycle(
      enabled: true,
      destroyTray: () async => calls.add('destroy-tray'),
      setPreventClose: (preventClose) async =>
          calls.add('prevent-close:$preventClose'),
      destroyWindow: () async => calls.add('destroy-window'),
      isMacOS: true,
    );

    lifecycle.onTrayMenuItemClick(
      tray.MenuItem(key: 'exit_app', label: 'Exit DavDeck'),
    );
    await Future<void>.delayed(Duration.zero);

    expect(calls, ['destroy-tray', 'prevent-close:false', 'destroy-window']);
  });

  test('a second close after Exit does not hide or quit again', () async {
    final calls = <String>[];
    final lifecycle = DesktopLifecycle(
      enabled: true,
      hideWindow: () async => calls.add('hide'),
      destroyTray: () async => calls.add('destroy-tray'),
      setPreventClose: (preventClose) async =>
          calls.add('prevent-close:$preventClose'),
      destroyWindow: () async => calls.add('destroy-window'),
      isMacOS: true,
    );

    await lifecycle.quit();
    lifecycle.onWindowClose();
    await Future<void>.delayed(Duration.zero);

    expect(calls, ['destroy-tray', 'prevent-close:false', 'destroy-window']);
  });
}
