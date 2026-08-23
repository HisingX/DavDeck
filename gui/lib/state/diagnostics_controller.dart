import 'package:davdeck/api/daemon_api.dart';
import 'package:flutter/foundation.dart';

class DiagnosticsController extends ChangeNotifier {
  DiagnosticsController(this.api);
  final DiagnosticsApi api;

  bool running = false;
  Object? error;
  DiagnosticReport? report;

  Future<void> run() async {
    running = true;
    error = null;
    notifyListeners();
    try {
      report = await api.runDiagnostics();
    } catch (caught) {
      error = caught;
    } finally {
      running = false;
      notifyListeners();
    }
  }
}
