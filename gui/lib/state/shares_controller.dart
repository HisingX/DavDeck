import 'package:davdeck/api/daemon_api.dart';
import 'package:flutter/foundation.dart';

class SharesController extends ChangeNotifier {
  SharesController(this.api);
  final ShareApi api;
  bool loading = true;
  bool busy = false;
  Object? error;
  List<ManagedShare> shares = const [];
  Future<void> refresh() async {
    loading = true;
    error = null;
    notifyListeners();
    try {
      shares = await api.listShares();
    } catch (caught) {
      error = caught;
    }
    loading = false;
    notifyListeners();
  }

  Future<bool> create(String name, String slug, String path) =>
      _mutate(() => api.createShare(name, slug, path));
  Future<bool> update(
    ManagedShare share, {
    String? name,
    String? slug,
    String? path,
    bool? enabled,
  }) => _mutate(
    () => api.updateShare(
      share.id,
      name: name,
      slug: slug,
      path: path,
      enabled: enabled,
    ),
  );
  Future<bool> delete(ManagedShare share) =>
      _mutate(() => api.deleteShare(share.id));
  Future<List<ManagedPermission>> permissions(ManagedShare share) =>
      api.listPermissions(share.id);
  Future<bool> setPermission(
    ManagedShare share,
    ManagedPermission entry,
    String permission,
  ) async {
    busy = true;
    error = null;
    notifyListeners();
    try {
      await api.setPermission(share.id, entry.userId, permission);
      return true;
    } catch (caught) {
      error = caught;
      return false;
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  Future<bool> _mutate(Future<Object?> Function() operation) async {
    busy = true;
    error = null;
    notifyListeners();
    try {
      await operation();
      shares = await api.listShares();
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
