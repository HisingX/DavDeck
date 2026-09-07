import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/permissions/permission_models.dart';
import 'package:flutter/foundation.dart';

class SharesController extends ChangeNotifier {
  SharesController(this.api);

  final ShareApi api;
  bool loading = true;
  Object? error;
  List<ManagedShare> shares = const [];
  final Set<String> busyShareIds = <String>{};
  bool _pageBusy = false;

  bool get busy => _pageBusy;
  bool get pageBusy => _pageBusy;
  bool isShareBusy(String shareId) => busyShareIds.contains(shareId);

  final Map<String, PermissionSaveState> permissionStates =
      <String, PermissionSaveState>{};
  final Map<String, String> _confirmedPermissions = <String, String>{};
  final Map<String, int> _permissionVersions = <String, int>{};

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
      _pageMutate(() async {
        await api.createShare(name, slug, path);
        shares = await api.listShares();
      });

  Future<bool> update(
    ManagedShare share, {
    String? name,
    String? slug,
    String? path,
    bool? enabled,
  }) => _shareMutate(share.id, () async {
    final updated = await api.updateShare(
      share.id,
      name: name,
      slug: slug,
      path: path,
      enabled: enabled,
    );
    _replaceShare(updated, preserveAuthorizedCount: true);
  });

  Future<bool> delete(ManagedShare share) => _shareMutate(share.id, () async {
    await api.deleteShare(share.id);
    shares = shares
        .where((value) => value.id != share.id)
        .toList(growable: false);
  });

  Future<List<ManagedPermission>> permissions(ManagedShare share) async {
    final entries = await api.listPermissions(share.id);
    for (final entry in entries) {
      _confirmedPermissions[permissionKey(share.id, entry.userId)] =
          entry.permission;
    }
    // The permission endpoint returns the rows for the dialog, while the
    // share list is the authoritative source for the aggregated user count.
    // Refresh it as the dialog opens so a change made on the Users page is
    // reflected immediately when the Share page is revisited.
    try {
      final refreshed = await _refreshShareSummary(share.id);
      if (refreshed) notifyListeners();
    } catch (_) {
      // Loading the dialog must remain possible even if the summary refresh
      // races with a daemon restart or a transient connection failure.
    }
    return entries;
  }

  Future<bool> setPermission(
    ManagedShare share,
    ManagedPermission entry,
    String permission,
  ) async {
    final key = permissionKey(share.id, entry.userId);
    final version = (_permissionVersions[key] ?? 0) + 1;
    _permissionVersions[key] = version;
    // The dialog loaded this entry from the daemon. Use that value as the
    // baseline instead of a value cached by an earlier page or dialog.
    _confirmedPermissions[key] = entry.permission;
    permissionStates[key] = PermissionSaveState.saving;
    error = null;
    notifyListeners();
    try {
      await api.setPermission(share.id, entry.userId, permission);
      if (_permissionVersions[key] == version) {
        _confirmedPermissions[key] = permission;
        permissionStates[key] = PermissionSaveState.saved;
        // Do not derive the aggregate from the cached row. The cache may be
        // stale after the same user was edited in User management. Reloading
        // the list makes the displayed count server-authoritative.
        try {
          await _refreshShareSummary(share.id);
        } catch (_) {
          // The permission itself was saved. A later page activation or
          // dialog open will retry the summary refresh.
        }
        notifyListeners();
      }
      return true;
    } catch (caught) {
      if (_permissionVersions[key] == version) {
        error = caught;
        permissionStates[key] = PermissionSaveState.error;
        notifyListeners();
      }
      return false;
    }
  }

  PermissionSaveState permissionSaveState(String shareId, String userId) =>
      permissionStates[permissionKey(shareId, userId)] ??
      PermissionSaveState.idle;

  String confirmedPermission(String shareId, String userId, String fallback) =>
      _confirmedPermissions[permissionKey(shareId, userId)] ?? fallback;

  void _replaceShare(
    ManagedShare updated, {
    bool preserveAuthorizedCount = false,
  }) {
    final current = shares.cast<ManagedShare?>().firstWhere(
      (value) => value?.id == updated.id,
      orElse: () => null,
    );
    // Share status/name updates return the base share response. Preserve the
    // count loaded by the list endpoint until the next list refresh.
    if (preserveAuthorizedCount &&
        current != null &&
        current.authorizedUserCount > 0 &&
        updated.authorizedUserCount == 0) {
      updated = updated.copyWith(
        authorizedUserCount: current.authorizedUserCount,
      );
    }
    shares = shares
        .map((value) => value.id == updated.id ? updated : value)
        .toList(growable: false);
  }

  Future<bool> _refreshShareSummary(String shareId) async {
    final latestShares = await api.listShares();
    final updated = latestShares.cast<ManagedShare?>().firstWhere(
      (value) => value?.id == shareId,
      orElse: () => null,
    );
    if (updated == null) return false;
    _replaceShare(updated);
    return true;
  }

  Future<bool> _shareMutate(
    String shareId,
    Future<void> Function() operation,
  ) async {
    busyShareIds.add(shareId);
    error = null;
    notifyListeners();
    try {
      await operation();
      return true;
    } catch (caught) {
      error = caught;
      return false;
    } finally {
      busyShareIds.remove(shareId);
      notifyListeners();
    }
  }

  Future<bool> _pageMutate(Future<void> Function() operation) async {
    _pageBusy = true;
    error = null;
    notifyListeners();
    try {
      await operation();
      return true;
    } catch (caught) {
      error = caught;
      return false;
    } finally {
      _pageBusy = false;
      notifyListeners();
    }
  }
}
