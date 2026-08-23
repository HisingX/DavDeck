import 'package:davdeck/api/daemon_api.dart';
import 'package:flutter/foundation.dart';

class UsersController extends ChangeNotifier {
  UsersController(this.api);
  final UserApi api;
  bool loading = true;
  bool busy = false;
  Object? loadError;
  Object? actionError;
  List<ManagedUser> users = const [];

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

  Future<bool> create(String username, String password) =>
      _mutate(() => api.createUser(username, password));
  Future<bool> setEnabled(ManagedUser user, bool enabled) =>
      _mutate(() => api.setUserEnabled(user.id, enabled));
  Future<bool> changePassword(ManagedUser user, String password) =>
      _mutate(() => api.changeUserPassword(user.id, password));
  Future<bool> delete(ManagedUser user) =>
      _mutate(() => api.deleteUser(user.id));

  Future<bool> _mutate(Future<Object?> Function() operation) async {
    busy = true;
    actionError = null;
    notifyListeners();
    try {
      await operation();
      users = await api.listUsers();
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
