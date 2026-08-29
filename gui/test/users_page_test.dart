import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

class FakeManagementApi implements ManagementApi {
  final users = <ManagedUser>[
    const ManagedUser(id: 'user-1', username: 'Alice', enabled: true),
  ];
  String? submittedPassword;

  @override
  Future<ManagedServerStatus> serverStatus() async =>
      const ManagedServerStatus(caddy: 'RUNNING');
  @override
  Future<void> startServer() async {}
  @override
  Future<void> stopServer() async {}
  @override
  Future<void> restartServer() async {}
  @override
  Future<ManagedServerSettings> serverSettings() async =>
      const ManagedServerSettings(httpPort: 8080, httpsPort: 8443);
  @override
  Future<ManagedServerSettings> updateServerPorts(
    int httpPort,
    int httpsPort,
  ) async => ManagedServerSettings(httpPort: httpPort, httpsPort: httpsPort);

  @override
  Future<ManagedServerEndpoints> serverEndpoints() async =>
      const ManagedServerEndpoints(
        http: ManagedServerEndpoint(
          protocol: 'HTTP',
          url: 'http://localhost:8080/dav/',
          port: 8080,
          state: 'RUNNING',
          configured: true,
          active: true,
          copyable: true,
        ),
        https: ManagedServerEndpoint(
          protocol: 'HTTPS',
          url: '',
          port: 8443,
          state: 'NOT_CONFIGURED',
          configured: false,
          active: false,
          copyable: false,
        ),
      );

  @override
  Future<DaemonStatus> status() async => const DaemonStatus(
    name: 'DavDeck',
    version: 'test',
    daemon: 'RUNNING',
    database: 'READY',
    schemaVersion: 4,
  );

  @override
  Future<ManagedLogPage> logs({
    int limit = 100,
    int? cursor,
    DateTime? since,
    String? level,
    String? component,
  }) async => const ManagedLogPage();

  @override
  Future<List<ManagedUser>> listUsers() async => List.of(users);
  @override
  Future<ManagedUser> createUser(String username, String password) async {
    submittedPassword = password;
    final user = ManagedUser(
      id: 'user-${users.length + 1}',
      username: username,
      enabled: true,
    );
    users.add(user);
    return user;
  }

  @override
  Future<ManagedUser> setUserEnabled(String id, bool enabled) async {
    final index = users.indexWhere((user) => user.id == id);
    final updated = ManagedUser(
      id: id,
      username: users[index].username,
      enabled: enabled,
    );
    users[index] = updated;
    return updated;
  }

  @override
  Future<void> changeUserPassword(String id, String password) async {
    submittedPassword = password;
  }

  @override
  Future<void> deleteUser(String id) async {
    users.removeWhere((user) => user.id == id);
  }

  @override
  Future<List<ManagedShare>> listShares() async => const [];
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
  }) => throw UnimplementedError();
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
  ) => throw UnimplementedError();
  @override
  Future<ManagedTlsProfile?> getTls() async => null;
  @override
  Future<ManagedTlsProfile> updateTls({
    required String mode,
    required String hostname,
    String certificatePath = '',
    String privateKeyPath = '',
  }) => throw UnimplementedError();
  @override
  Future<void> disableTls() => throw UnimplementedError();
  @override
  Future<ManagedTlsCheckResult> checkTls() => throw UnimplementedError();
  @override
  Future<void> applyConfiguration() => throw UnimplementedError();
  @override
  Future<DiagnosticReport> runDiagnostics() => throw UnimplementedError();
}

void main() {
  testWidgets('create user validates the password before calling the API', (
    tester,
  ) async {
    final api = FakeManagementApi();
    await tester.pumpWidget(DavDeckApp(api: api));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Users'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Add user'));
    await tester.pumpAndSettle();
    final fields = find.byType(TextField);
    await tester.enterText(fields.first, 'Bob');
    await tester.tap(find.text('Create'));
    await tester.pumpAndSettle();

    expect(
      find.text('Password must contain 8 to 72 UTF-8 bytes.'),
      findsOneWidget,
    );
    expect(api.users, hasLength(1));
  });

  testWidgets(
    'users page lists and creates users without displaying password',
    (tester) async {
      final api = FakeManagementApi();
      await tester.pumpWidget(DavDeckApp(api: api));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Users'));
      await tester.pumpAndSettle();
      expect(find.text('Alice'), findsOneWidget);
      await tester.tap(find.text('Add user'));
      await tester.pumpAndSettle();
      final fields = find.byType(TextField);
      await tester.enterText(fields.first, 'Bob');
      await tester.enterText(fields.last, 'private password');
      await tester.tap(find.text('Create'));
      await tester.pumpAndSettle();
      expect(find.text('Bob'), findsOneWidget);
      expect(find.text('private password'), findsNothing);
      expect(api.submittedPassword, 'private password');
    },
  );

  testWidgets('user enable switch calls API', (tester) async {
    final api = FakeManagementApi();
    await tester.pumpWidget(DavDeckApp(api: api));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Users'));
    await tester.pumpAndSettle();
    await tester.tap(find.byType(Switch));
    await tester.pumpAndSettle();
    expect(api.users.single.enabled, isFalse);
    expect(find.text('Disabled'), findsOneWidget);
  });
}
