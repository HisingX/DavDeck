import 'package:davdeck/api/daemon_api.dart';
import 'package:flutter/foundation.dart';

enum RevisionLoadState { loading, ready, error }

class RevisionController extends ChangeNotifier {
  RevisionController(this.api);

  final RevisionApi api;
  RevisionLoadState state = RevisionLoadState.loading;
  List<ManagedRevision> revisions = const [];
  ManagedRevisionState? configuration;
  Object? error;
  String? restoringId;
  String? deletingId;

  Future<void> refresh() async {
    state = RevisionLoadState.loading;
    error = null;
    notifyListeners();
    try {
      final values = await Future.wait([
        api.configurationState(),
        api.listRevisions(),
      ]);
      configuration = values[0] as ManagedRevisionState;
      revisions = values[1] as List<ManagedRevision>;
      state = RevisionLoadState.ready;
    } catch (caught) {
      error = caught;
      state = RevisionLoadState.error;
    }
    notifyListeners();
  }

  Future<bool> restore(ManagedRevision revision) async {
    if (revision.validationStatus != 'VALID' ||
        restoringId != null ||
        deletingId != null) {
      return false;
    }
    restoringId = revision.id;
    error = null;
    notifyListeners();
    try {
      await api.restoreRevision(revision.id);
      await refresh();
      return true;
    } catch (caught) {
      error = caught;
      return false;
    } finally {
      restoringId = null;
      notifyListeners();
    }
  }

  Future<bool> delete(ManagedRevision revision) async {
    if (restoringId != null || deletingId != null) return false;
    deletingId = revision.id;
    error = null;
    notifyListeners();
    try {
      await api.deleteRevision(revision.id);
      await refresh();
      return true;
    } catch (caught) {
      error = caught;
      return false;
    } finally {
      deletingId = null;
      notifyListeners();
    }
  }
}
