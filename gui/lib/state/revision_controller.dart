import 'package:davdeck/api/daemon_api.dart';
import 'package:flutter/foundation.dart';

enum RevisionLoadState { loading, ready, error }

class RevisionController extends ChangeNotifier {
  RevisionController(this.api, {this.onRestored});

  final RevisionApi api;
  final Future<void> Function()? onRestored;
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
        !revision.stateSnapshotAvailable ||
        restoringId != null ||
        deletingId != null) {
      return false;
    }
    restoringId = revision.id;
    error = null;
    notifyListeners();
    try {
      await api.restoreRevision(revision.id);
      await onRestored?.call();
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

  /// Deletes several revisions through the existing authenticated revision
  /// endpoint. Protected revisions are filtered by the page before this
  /// method is called; each individual request still receives the daemon's
  /// normal safety checks.
  Future<bool> deleteMany(Iterable<ManagedRevision> revisions) async {
    if (restoringId != null || deletingId != null) return false;
    final ids = revisions.map((revision) => revision.id).toSet().toList();
    if (ids.isEmpty) return false;

    deletingId = ids.first;
    error = null;
    notifyListeners();
    try {
      for (final id in ids) {
        await api.deleteRevision(id);
      }
      await refresh();
      return true;
    } catch (caught) {
      await refresh();
      error = caught;
      notifyListeners();
      return false;
    } finally {
      deletingId = null;
      notifyListeners();
    }
  }
}
