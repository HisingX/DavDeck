import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/state/users_controller.dart';
import 'package:flutter_test/flutter_test.dart';

class RecordingUserApi implements UserApi {
  final users = <ManagedUser>[
    const ManagedUser(id: '1', username: 'Alice', enabled: true),
  ];
  String? changedPasswordFor;
  String? deletedUser;
  bool failCreate = false;
  @override
  Future<List<ManagedUser>> listUsers() async => List.of(users);
  @override
  Future<ManagedUser> createUser(String username, String password) async {
    if (failCreate) {
      throw const DaemonApiException('USER_ALREADY_EXISTS', 'duplicate');
    }
    final user = ManagedUser(id: '2', username: username, enabled: true);
    users.add(user);
    return user;
  }

  @override
  Future<ManagedUser> setUserEnabled(String id, bool enabled) async =>
      throw UnimplementedError();
  @override
  Future<void> changeUserPassword(String id, String password) async {
    changedPasswordFor = id;
  }

  @override
  Future<void> deleteUser(String id) async {
    deletedUser = id;
    users.removeWhere((user) => user.id == id);
  }
}

void main() {
  test('users controller changes password and deletes through API', () async {
    final api = RecordingUserApi();
    final controller = UsersController(api);
    await controller.refresh();
    final user = controller.users.single;
    expect(
      await controller.changePassword(user, 'new private password'),
      isTrue,
    );
    expect(api.changedPasswordFor, user.id);
    expect(await controller.delete(user), isTrue);
    expect(api.deletedUser, user.id);
    expect(controller.users, isEmpty);
    controller.dispose();
  });

  test(
    'a create failure does not become a user-list loading failure',
    () async {
      final api = RecordingUserApi()..failCreate = true;
      final controller = UsersController(api);
      await controller.refresh();

      expect(await controller.create('Alice', 'valid password'), isFalse);
      expect(controller.loadError, isNull);
      expect(controller.actionError, isA<DaemonApiException>());
      expect(controller.users, hasLength(1));
      controller.dispose();
    },
  );
}
