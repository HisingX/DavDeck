import 'dart:async';
import 'dart:io';

import 'package:tray_manager/tray_manager.dart' as tray;
import 'package:window_manager/window_manager.dart';

const _macOSIconAsset = 'assets/branding/davdeck_tray.png';
const _windowsIconAsset = 'windows/runner/resources/app_icon.ico';

/// Owns desktop-only window and tray behavior.
///
/// Linux intentionally does not install a tray listener. Linux headless
/// operation remains the supported service mode and the Linux GUI stays a
/// regular windowed client for now.
class DesktopLifecycle with tray.TrayListener, WindowListener {
  DesktopLifecycle({
    bool? enabled,
    Future<void> Function()? hideWindow,
    Future<void> Function()? showWindow,
    Future<void> Function()? focusWindow,
    Future<void> Function()? destroyTray,
    Future<void> Function(bool preventClose)? setPreventClose,
    Future<void> Function()? destroyWindow,
  }) : _enabled = enabled ?? (Platform.isWindows || Platform.isMacOS),
       _hideWindowOverride = hideWindow,
       _showWindowOverride = showWindow,
       _focusWindowOverride = focusWindow,
       _destroyTrayOverride = destroyTray,
       _setPreventCloseOverride = setPreventClose,
       _destroyWindowOverride = destroyWindow;

  final bool _enabled;
  final Future<void> Function()? _hideWindowOverride;
  final Future<void> Function()? _showWindowOverride;
  final Future<void> Function()? _focusWindowOverride;
  final Future<void> Function()? _destroyTrayOverride;
  final Future<void> Function(bool preventClose)? _setPreventCloseOverride;
  final Future<void> Function()? _destroyWindowOverride;
  bool _initialized = false;
  bool _quitting = false;

  bool get enabled => _enabled;

  Future<void> initialize() async {
    if (!_enabled || _initialized) return;

    await windowManager.ensureInitialized();
    windowManager.addListener(this);
    tray.trayManager.addListener(this);
    try {
      await windowManager.setPreventClose(true);
      await tray.trayManager.setIcon(
        Platform.isMacOS ? _macOSIconAsset : _windowsIconAsset,
        isTemplate: Platform.isMacOS,
      );
      await tray.trayManager.setToolTip('DavDeck');
      await tray.trayManager.setContextMenu(
        tray.Menu(
          items: [
            tray.MenuItem(
              key: 'show_window',
              label: _isChinese ? '显示 DavDeck' : 'Show DavDeck',
            ),
            tray.MenuItem.separator(),
            tray.MenuItem(
              key: 'exit_app',
              label: _isChinese ? '退出 DavDeck' : 'Exit DavDeck',
            ),
          ],
        ),
      );
      _initialized = true;
    } catch (_) {
      tray.trayManager.removeListener(this);
      windowManager.removeListener(this);
      rethrow;
    }
  }

  bool get _isChinese => Platform.localeName.toLowerCase().startsWith('zh');

  Future<void> closeToTray() async {
    if (!_enabled || _quitting) return;
    await (_hideWindowOverride?.call() ?? windowManager.hide());
  }

  Future<void> showWindow() async {
    if (!_enabled || _quitting) return;
    await (_showWindowOverride?.call() ?? windowManager.show());
    await (_focusWindowOverride?.call() ?? windowManager.focus());
  }

  Future<void> quit() async {
    if (!_enabled || _quitting) return;
    _quitting = true;
    await (_destroyTrayOverride?.call() ?? tray.trayManager.destroy());
    await (_setPreventCloseOverride?.call(false) ??
        windowManager.setPreventClose(false));
    await (_destroyWindowOverride?.call() ?? windowManager.destroy());
  }

  @override
  void onWindowClose() {
    unawaited(closeToTray());
  }

  @override
  void onTrayIconMouseDown() {
    if (Platform.isMacOS) {
      unawaited(tray.trayManager.popUpContextMenu());
    } else {
      unawaited(showWindow());
    }
  }

  @override
  void onTrayIconRightMouseDown() {
    if (Platform.isWindows) {
      unawaited(tray.trayManager.popUpContextMenu());
    }
  }

  @override
  void onTrayMenuItemClick(tray.MenuItem menuItem) {
    switch (menuItem.key) {
      case 'show_window':
        unawaited(showWindow());
      case 'exit_app':
        unawaited(quit());
    }
  }

  void dispose() {
    if (!_initialized) return;
    windowManager.removeListener(this);
    tray.trayManager.removeListener(this);
    _initialized = false;
  }
}
