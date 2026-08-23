import 'package:davdeck/api/daemon_api.dart';
import 'package:flutter/foundation.dart';

enum ServiceLoadState { loading, ready, error }

class ServiceController extends ChangeNotifier {
  ServiceController(this.api, {this.onChanged});

  final ServiceApi api;
  final Future<void> Function()? onChanged;
  ServiceLoadState state = ServiceLoadState.loading;
  ManagedServiceStatus? service;
  Object? error;
  Object? actionError;
  bool busy = false;

  Future<void> refresh() async {
    state = ServiceLoadState.loading;
    error = null;
    notifyListeners();
    try {
      service = await api.serviceStatus();
      state = ServiceLoadState.ready;
    } catch (caught) {
      error = caught;
      state = ServiceLoadState.error;
    }
    notifyListeners();
  }

  Future<bool> install() => _mutate(api.installService);
  Future<bool> uninstall() => _mutate(api.uninstallService);
  Future<bool> start() => _mutate(api.startService);
  Future<bool> stop() => _mutate(api.stopService);

  Future<bool> _mutate(Future<void> Function() operation) async {
    if (busy) return false;
    busy = true;
    actionError = null;
    notifyListeners();
    try {
      await operation();
      await refresh();
      await onChanged?.call();
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
