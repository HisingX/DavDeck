import 'package:davdeck/api/daemon_api.dart';
import 'package:flutter/foundation.dart';

class TlsController extends ChangeNotifier {
  TlsController(this.api, this.configurationApi);
  final TlsApi api;
  final ConfigurationApi configurationApi;

  bool loading = true;
  bool busy = false;
  Object? error;
  ManagedTlsProfile? profile;
  ManagedTlsCheckResult? checkResult;
  bool pendingApply = false;

  Future<void> refresh() async {
    loading = true;
    error = null;
    notifyListeners();
    try {
      profile = await api.getTls();
    } catch (caught) {
      error = caught;
    } finally {
      loading = false;
      notifyListeners();
    }
  }

  Future<bool> configure({
    required String mode,
    required String hostname,
    String certificatePath = '',
    String privateKeyPath = '',
  }) async {
    busy = true;
    error = null;
    checkResult = null;
    notifyListeners();
    try {
      profile = await api.updateTls(
        mode: mode,
        hostname: hostname,
        certificatePath: certificatePath,
        privateKeyPath: privateKeyPath,
      );
      pendingApply = true;
      return true;
    } catch (caught) {
      error = caught;
      return false;
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  Future<bool> check() async {
    busy = true;
    error = null;
    checkResult = null;
    notifyListeners();
    try {
      checkResult = await api.checkTls();
      return true;
    } catch (caught) {
      error = caught;
      return false;
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  Future<bool> disable() async {
    busy = true;
    error = null;
    checkResult = null;
    notifyListeners();
    try {
      await api.disableTls();
      profile = null;
      pendingApply = true;
      return true;
    } catch (caught) {
      error = caught;
      return false;
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  Future<bool> apply() async {
    busy = true;
    error = null;
    notifyListeners();
    try {
      await configurationApi.applyConfiguration();
      pendingApply = false;
      return true;
    } catch (caught) {
      error = caught;
      return false;
    } finally {
      busy = false;
      notifyListeners();
    }
  }
}
