import 'package:davdeck/api/daemon_api.dart';
import 'package:flutter/foundation.dart';

class BackupController extends ChangeNotifier {
  BackupController(this.api);

  final BackupApi api;
  bool exporting = false;
  bool importing = false;
  Object? error;
  ManagedConfigImportResult? lastImport;

  Future<String?> exportConfiguration() async {
    if (exporting || importing) return null;
    exporting = true;
    error = null;
    notifyListeners();
    try {
      return await api.exportConfiguration();
    } catch (caught) {
      error = caught;
      return null;
    } finally {
      exporting = false;
      notifyListeners();
    }
  }

  Future<bool> importConfiguration(String content) async {
    if (exporting || importing) return false;
    importing = true;
    error = null;
    lastImport = null;
    notifyListeners();
    try {
      lastImport = await api.importConfiguration(content);
      return true;
    } catch (caught) {
      error = caught;
      return false;
    } finally {
      importing = false;
      notifyListeners();
    }
  }
}
