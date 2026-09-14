import 'dart:async';

import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/permissions/permission_models.dart';
import 'package:davdeck/state/shares_controller.dart';
import 'package:davdeck/state/users_controller.dart';
import 'package:flutter_test/flutter_test.dart';

const _shareOne = ManagedShare(
  id: 'share-1',
  name: 'Documents',
  slug: 'documents',
  path: '/srv/documents',
  enabled: true,
);

const _shareTwo = ManagedShare(
  id: 'share-2',
  name: 'Archive',
  slug: 'archive',
  path: '/srv/archive',
  enabled: true,
);

const _userOne = ManagedUser(id: 'user-1', username: 'Alice', enabled: true);
const _userTwo = ManagedUser(id: 'user-2', username: 'Bob', enabled: true);
const _permissionSummary = ManagedUserPermission(
  shareId: 'share-1',
  shareName: 'Documents',
  shareSlug: 'documents',
  shareEnabled: true,
  permission: 'READ',
);

class _BlockingShareApi implements ShareApi {
  final pendingUpdates = <Completer<ManagedShare>>[];
  final pendingPermissions = <Completer<ManagedPermission>>[];
  int listedAuthorizedUserCount = 0;

  @override
  Future<List<ManagedShare>> listShares() async => [
    _shareOne.copyWith(authorizedUserCount: listedAuthorizedUserCount),
    _shareTwo,
  ];

  @override
  Future<ManagedShare> createShare(String name, String slug, String path) =>
      throw UnimplementedError();

  @override
  Future<ManagedShare> updateShare(
    String id, {
    String? name,
    String? slug,
    String? path,
    bool? enabled,
  }) {
    final completer = Completer<ManagedShare>();
    pendingUpdates.add(completer);
    return completer.future;
  }

  @override
  Future<void> deleteShare(String id) => throw UnimplementedError();

  @override
  Future<List<ManagedPermission>> listPermissions(String shareId) =>
      throw UnimplementedError();

  @override
  Future<ManagedPermission> setPermission(
    String shareId,
    String userId,
    String permission,
  ) {
    final completer = Completer<ManagedPermission>();
    pendingPermissions.add(completer);
    return completer.future;
  }
}

class _BlockingUserApi implements UserApi, UserPermissionsApi {
  final pendingEnabled = <Completer<ManagedUser>>[];
  final pendingPermissions = <Completer<ManagedPermission>>[];

  @override
  Future<List<ManagedUser>> listUsers() async => const [_userOne, _userTwo];

  @override
  Future<ManagedUser> createUser(String username, String password) =>
      throw UnimplementedError();

  @override
  Future<ManagedUser> setUserEnabled(String id, bool enabled) {
    final completer = Completer<ManagedUser>();
    pendingEnabled.add(completer);
    return completer.future;
  }

  @override
  Future<void> changeUserPassword(String id, String password) =>
      throw UnimplementedError();

  @override
  Future<void> deleteUser(String id) => throw UnimplementedError();

  @override
  Future<List<ManagedUserPermission>> listUserPermissions(String userId) =>
      throw UnimplementedError();

  @override
  Future<ManagedPermission> setUserPermission(
    String shareId,
    String userId,
    String permission,
  ) {
    final completer = Completer<ManagedPermission>();
    pendingPermissions.add(completer);
    return completer.future;
  }
}

void main() {
  test('share row mutations do not lock unrelated rows', () async {
    final api = _BlockingShareApi();
    final controller = SharesController(api)
      ..shares = [_shareOne.copyWith(authorizedUserCount: 3), _shareTwo];
    addTearDown(controller.dispose);

    final first = controller.update(_shareOne, enabled: false);
    final second = controller.update(_shareTwo, enabled: false);
    expect(controller.busy, isFalse);
    expect(controller.busyShareIds, containsAll(['share-1', 'share-2']));

    api.pendingUpdates[0].complete(_shareOne.copyWith(enabled: false));
    expect(await first, isTrue);
    expect(controller.isShareBusy('share-1'), isFalse);
    expect(controller.isShareBusy('share-2'), isTrue);
    expect(controller.shares.first.authorizedUserCount, 3);

    api.pendingUpdates[1].complete(_shareTwo.copyWith(enabled: false));
    expect(await second, isTrue);
    expect(controller.busyShareIds, isEmpty);
  });

  test('share permission saves are last-write-wins per row', () async {
    final api = _BlockingShareApi();
    final controller = SharesController(api)
      // Deliberately start with a stale value. A successful permission save
      // must replace it with the server-side aggregate.
      ..shares = [_shareOne.copyWith(authorizedUserCount: 7)];
    addTearDown(controller.dispose);
    const entry = ManagedPermission(
      shareId: 'share-1',
      userId: 'user-1',
      username: 'Alice',
      permission: 'NONE',
    );

    final first = controller.setPermission(_shareOne, entry, 'READ');
    final second = controller.setPermission(
      _shareOne,
      entry.copyWith(permission: 'READ'),
      'READ_WRITE',
    );
    expect(
      controller.permissionSaveState('share-1', 'user-1'),
      PermissionSaveState.saving,
    );

    // The list endpoint is the source of truth for the aggregate count.
    api.listedAuthorizedUserCount = 1;
    api.pendingPermissions[1].complete(
      entry.copyWith(permission: 'READ_WRITE'),
    );
    expect(await second, isTrue);
    api.pendingPermissions[0].complete(entry.copyWith(permission: 'READ'));
    expect(await first, isTrue);

    expect(
      controller.confirmedPermission('share-1', 'user-1', 'NONE'),
      'READ_WRITE',
    );
    expect(
      controller.permissionSaveState('share-1', 'user-1'),
      PermissionSaveState.saved,
    );
    expect(controller.shares.single.authorizedUserCount, 1);
  });

  test('user row mutations do not lock unrelated rows', () async {
    final api = _BlockingUserApi();
    final controller = UsersController(api)
      ..users = [
        _userOne.copyWith(
          authorizedShareCount: 1,
          permissions: const [_permissionSummary],
        ),
        _userTwo,
      ];
    addTearDown(controller.dispose);

    final first = controller.setEnabled(_userOne, false);
    final second = controller.setEnabled(_userTwo, false);
    expect(controller.busy, isFalse);
    expect(controller.busyUserIds, containsAll(['user-1', 'user-2']));

    api.pendingEnabled[0].complete(_userOne.copyWith(enabled: false));
    expect(await first, isTrue);
    expect(controller.isUserBusy('user-1'), isFalse);
    expect(controller.isUserBusy('user-2'), isTrue);
    expect(controller.users.first.authorizedShareCount, 1);

    api.pendingEnabled[1].complete(_userTwo.copyWith(enabled: false));
    expect(await second, isTrue);
    expect(controller.busyUserIds, isEmpty);
  });

  test('user permission saves are last-write-wins per row', () async {
    final api = _BlockingUserApi();
    final controller = UsersController(api)..users = const [_userOne];
    addTearDown(controller.dispose);
    const entry = ManagedUserPermission(
      shareId: 'share-1',
      shareName: 'Documents',
      shareSlug: 'documents',
      shareEnabled: true,
      permission: 'NONE',
    );

    final first = controller.setPermission(_userOne, entry, 'READ');
    final second = controller.setPermission(
      _userOne,
      entry.copyWith(permission: 'READ'),
      'READ_WRITE',
    );
    expect(
      controller.permissionSaveState('share-1', 'user-1'),
      PermissionSaveState.saving,
    );

    api.pendingPermissions[1].complete(
      ManagedPermission(
        shareId: 'share-1',
        userId: 'user-1',
        username: 'Alice',
        permission: 'READ_WRITE',
      ),
    );
    expect(await second, isTrue);
    api.pendingPermissions[0].complete(
      ManagedPermission(
        shareId: 'share-1',
        userId: 'user-1',
        username: 'Alice',
        permission: 'READ',
      ),
    );
    expect(await first, isTrue);

    expect(
      controller.confirmedPermission('share-1', 'user-1', 'NONE'),
      'READ_WRITE',
    );
    expect(
      controller.permissionSaveState('share-1', 'user-1'),
      PermissionSaveState.saved,
    );
    expect(controller.users.single.authorizedShareCount, 1);
  });
}
