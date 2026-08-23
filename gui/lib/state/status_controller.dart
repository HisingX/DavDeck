import 'package:davdeck/api/daemon_api.dart';
import 'package:flutter/foundation.dart';

enum LoadState { loading, ready, error }

class StatusController extends ChangeNotifier {
  StatusController(
    this.api, [
    this.server,
    this.settings,
    this.configuration,
    this.revisions,
  ]);

  final DaemonApi api;
  final ServerApi? server;
  final ServerSettingsApi? settings;
  final ConfigurationApi? configuration;
  final RevisionApi? revisions;
  LoadState state = LoadState.loading;
  DaemonStatus? status;
  Object? error;
  ManagedServerStatus? runtime;
  ManagedServerSettings? serverSettings;
  Object? actionError;
  ManagedRevision? applyResult;
  bool busy = false;

  Future<void> refresh() async {
    state = LoadState.loading;
    error = null;
    notifyListeners();
    try {
      status = await api.status();
      runtime = server == null ? null : await server!.serverStatus();
      serverSettings = settings == null
          ? null
          : await settings!.serverSettings();
      state = LoadState.ready;
    } catch (caught) {
      error = caught;
      state = LoadState.error;
    }
    notifyListeners();
  }

  Future<void> control(Future<void> Function() action) async {
    if (server == null) return;
    busy = true;
    actionError = null;
    notifyListeners();
    try {
      await action();
      await refresh();
    } catch (caught) {
      actionError = caught;
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  Future<void> updatePorts(int httpPort, int httpsPort) async {
    if (settings == null) return;
    busy = true;
    notifyListeners();
    try {
      await settings!.updateServerPorts(httpPort, httpsPort);
      await refresh();
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  Future<bool> applyPending() async {
    if (configuration == null) return false;
    busy = true;
    actionError = null;
    applyResult = null;
    notifyListeners();
    try {
      if (revisions != null) {
        applyResult = await revisions!.applyConfigurationResult();
      } else {
        await configuration!.applyConfiguration();
      }
      await refresh();
      return true;
    } catch (caught) {
      actionError = caught;
      return false;
    } finally {
      busy = false;
      notifyListeners();
    }
  }
}
