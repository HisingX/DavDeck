import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/permissions/permission_models.dart';
import 'package:flutter/foundation.dart';

class UsersController extends ChangeNotifier {
  UsersController(this.api);

  final UserApi api;
  bool loading = true;
  Object? loadError;
  Object? actionError;
  List<ManagedUser> users = const [];
  final Set<String> busyUserIds = <String>{};
  bool _pageBusy = false;

  /// Page-level mutations (create) still expose a busy state for the header.
  /// User-specific operations use [busyUserIds] so unrelated rows stay usable.
  bool get busy => _pageBusy;
  bool get pageBusy => _pageBusy;
  bool isUserBusy(String userId) => busyUserIds.contains(userId);

  UserPermissionsApi? get permissionsApi =>
      api is UserPermissionsApi ? api as UserPermissionsApi : null;

  final Map<String, PermissionSaveState> permissionStates =
      <String, PermissionSaveState>{};
  final Map<String, String> _confirmedPermissions = <String, String>{};
  final Map<String, int> _permissionVersions = <String, int>{};

  Future<void> refresh() async {
    loading = true;
    loadError = null;
    notifyListeners();
    try {
      users = await api.listUsers();
    } catch (caught) {
      loadError = caught;
    }
    loading = false;
    notifyListeners();
  }

  Future<List<ManagedUserPermission>> userPermissions(ManagedUser user) async {
    final permissionApi = permissionsApi;
    if (permissionApi == null) {
      throw UnsupportedError('User permission API is unavailable');
    }
    final entries = await permissionApi.listUserPermissions(user.id);
    _updateUserPermissionSummary(user.id, entries);
    notifyListeners();
    return entries;
  }

  Future<bool> create(String username, String password) =>
      _pageMutate(() async {
        await api.createUser(username, password);
        users = await api.listUsers();
      });

  Future<bool> setEnabled(ManagedUser user, bool enabled) =>
      _userMutate(user.id, () async {
        final updated = await api.setUserEnabled(user.id, enabled);
        _replaceUser(updated, preserveSummary: true);
      });

  Future<bool> changePassword(ManagedUser user, String password) =>
      _userMutate(user.id, () => api.changeUserPassword(user.id, password));

  Future<bool> delete(ManagedUser user) => _userMutate(user.id, () async {
    await api.deleteUser(user.id);
    users = users.where((value) => value.id != user.id).toList(growable: false);
  });

  Future<bool> setPermission(
    ManagedUser user,
    ManagedUserPermission entry,
    String permission,
  ) async {
    final permissionApi = permissionsApi;
    if (permissionApi == null) return false;
    final key = permissionKey(entry.shareId, user.id);
    final version = (_permissionVersions[key] ?? 0) + 1;
    _permissionVersions[key] = version;
    _confirmedPermissions.putIfAbsent(key, () => entry.permission);
    permissionStates[key] = PermissionSaveState.saving;
    actionError = null;
    notifyListeners();
    try {
      await permissionApi.setUserPermission(entry.shareId, user.id, permission);
      if (_permissionVersions[key] == version) {
        _confirmedPermissions[key] = permission;
        permissionStates[key] = PermissionSaveState.saved;
        _updateUserPermissionSummaryEntry(user.id, entry, permission);
        notifyListeners();
      }
      return true;
    } catch (caught) {
      if (_permissionVersions[key] == version) {
        actionError = caught;
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

  void _replaceUser(ManagedUser updated, {bool preserveSummary = false}) {
    final current = users.cast<ManagedUser?>().firstWhere(
      (value) => value?.id == updated.id,
      orElse: () => null,
    );
    // PATCH /users/{id} intentionally returns the compact user shape. Keep
    // the already loaded permission summary while a status-only mutation is
    // in flight so a successful toggle does not blank the card chips.
    if (preserveSummary &&
        current != null &&
        current.permissionSummaryAvailable &&
        updated.authorizedShareCount == 0 &&
        updated.permissions.isEmpty) {
      updated = updated.copyWith(
        authorizedShareCount: current.authorizedShareCount,
        permissions: current.permissions,
        permissionSummaryAvailable: true,
      );
    }
    users = users
        .map((value) => value.id == updated.id ? updated : value)
        .toList(growable: false);
  }

  void _updateUserPermissionSummary(
    String userId,
    List<ManagedUserPermission> entries,
  ) {
    final authorized = entries
        .where((entry) => entry.permission != 'NONE')
        .toList(growable: false);
    final user = users.cast<ManagedUser?>().firstWhere(
      (value) => value?.id == userId,
      orElse: () => null,
    );
    if (user == null) return;
    _replaceUser(
      user.copyWith(
        authorizedShareCount: authorized.length,
        permissions: authorized,
        permissionSummaryAvailable: true,
      ),
    );
  }

  void _updateUserPermissionSummaryEntry(
    String userId,
    ManagedUserPermission entry,
    String permission,
  ) {
    final user = users.cast<ManagedUser?>().firstWhere(
      (value) => value?.id == userId,
      orElse: () => null,
    );
    if (user == null) return;
    final next = user.permissions
        .where((value) => value.shareId != entry.shareId)
        .toList();
    if (permission != 'NONE') {
      next.add(entry.copyWith(permission: permission));
    }
    _replaceUser(
      user.copyWith(
        authorizedShareCount: next.length,
        permissions: next,
        permissionSummaryAvailable: true,
      ),
    );
  }

  Future<bool> _userMutate(
    String userId,
    Future<void> Function() operation,
  ) async {
    busyUserIds.add(userId);
    actionError = null;
    notifyListeners();
    try {
      await operation();
      return true;
    } catch (caught) {
      actionError = caught;
      return false;
    } finally {
      busyUserIds.remove(userId);
      notifyListeners();
    }
  }

  Future<bool> _pageMutate(Future<void> Function() operation) async {
    _pageBusy = true;
    actionError = null;
    notifyListeners();
    try {
      await operation();
      return true;
    } catch (caught) {
      actionError = caught;
      return false;
    } finally {
      _pageBusy = false;
      notifyListeners();
    }
  }
}
