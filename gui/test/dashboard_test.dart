import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

class FakeDaemonApi implements ManagementApi {
  FakeDaemonApi({
    this.failure,
    this.startFailure,
    this.statusValue,
    this.publicBasePath = '/dav',
    this.tlsHostname,
    this.endpointState = 'RUNNING',
    this.runtimeState = 'RUNNING',
  });
  final Object? failure;
  final Object? startFailure;
  final DaemonStatus? statusValue;
  final String publicBasePath;
  final String? tlsHostname;
  final String endpointState;
  final String runtimeState;
  var applied = false;
  var startCalls = 0;

  @override
  Future<ManagedServerStatus> serverStatus() async =>
      ManagedServerStatus(caddy: runtimeState, webdav: runtimeState);
  @override
  Future<void> startServer() async {
    startCalls++;
    if (startFailure != null) throw startFailure!;
  }

  @override
  Future<void> stopServer() async {}
  @override
  Future<void> restartServer() async {}
  @override
  Future<ManagedServerSettings> serverSettings() async => ManagedServerSettings(
    httpPort: 8080,
    httpsPort: 8443,
    publicBasePath: publicBasePath,
  );
  @override
  Future<ManagedServerSettings> updateServerPorts(
    int httpPort,
    int httpsPort,
  ) async => ManagedServerSettings(
    httpPort: httpPort,
    httpsPort: httpsPort,
    publicBasePath: publicBasePath,
  );

  @override
  Future<ManagedServerEndpoints>
  serverEndpoints() async => ManagedServerEndpoints(
    http: ManagedServerEndpoint(
      protocol: 'HTTP',
      url:
          '${tlsHostname == null ? 'http://localhost' : 'http://$tlsHostname'}:8080$publicBasePath/',
      port: 8080,
      state: endpointState,
      configured: true,
      active: endpointState == 'RUNNING',
      copyable: endpointState == 'RUNNING',
    ),
    https: ManagedServerEndpoint(
      protocol: 'HTTPS',
      url: tlsHostname == null
          ? ''
          : 'https://$tlsHostname:8443$publicBasePath/',
      port: 8443,
      state: tlsHostname == null ? 'NOT_CONFIGURED' : endpointState,
      configured: tlsHostname != null,
      active: tlsHostname != null && endpointState == 'RUNNING',
      copyable: tlsHostname != null && endpointState == 'RUNNING',
    ),
  );

  @override
  Future<DaemonStatus> status() async {
    if (failure != null) throw failure!;
    return statusValue ??
        DaemonStatus(
          name: 'DavDeck',
          version: 'test',
          daemon: 'RUNNING',
          database: 'READY',
          schemaVersion: 1,
          caddy: runtimeState,
          webdav: runtimeState,
        );
  }

  @override
  Future<ManagedLogPage> logs({
    int limit = 100,
    int? cursor,
    DateTime? since,
    String? level,
    String? component,
  }) async => const ManagedLogPage();

  @override
  Future<List<ManagedUser>> listUsers() async => const [];
  @override
  Future<ManagedUser> createUser(String username, String password) =>
      throw UnimplementedError();
  @override
  Future<ManagedUser> setUserEnabled(String id, bool enabled) =>
      throw UnimplementedError();
  @override
  Future<void> changeUserPassword(String id, String password) =>
      throw UnimplementedError();
  @override
  Future<void> deleteUser(String id) => throw UnimplementedError();
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
  Future<ManagedTlsProfile> renewTls() => throw UnimplementedError();
  @override
  Future<ManagedTlsProfile> cancelTlsRenewal() => throw UnimplementedError();
  @override
  Future<ManagedTlsCheckResult> checkTls() => throw UnimplementedError();
  @override
  Future<void> applyConfiguration() async {
    applied = true;
  }

  @override
  Future<DiagnosticReport> runDiagnostics() => throw UnimplementedError();
}

void main() {
  testWidgets('dashboard displays daemon status', (tester) async {
    await tester.binding.setSurfaceSize(const Size(1100, 900));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.pumpWidget(DavDeckApp(api: FakeDaemonApi()));
    await tester.pumpAndSettle();
    expect(find.text('DavDeck'), findsWidgets);
    expect(find.text('Daemon: RUNNING'), findsOneWidget);
    expect(find.text('Database: READY'), findsOneWidget);
    expect(find.text('http://localhost:8080/dav/'), findsOneWidget);
    expect(find.text('Not configured'), findsOneWidget);
  });

  testWidgets('dashboard renders wide panels without layout errors', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1800, 900));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.pumpWidget(DavDeckApp(api: FakeDaemonApi()));
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(find.text('Runtime control'), findsOneWidget);
    expect(find.text('Access endpoints'), findsOneWidget);
  });

  testWidgets('dashboard localizes connection failure', (tester) async {
    await tester.pumpWidget(
      DavDeckApp(
        api: FakeDaemonApi(failure: Exception('offline')),
        locale: const Locale('zh', 'CN'),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('无法连接到本机 DavDeck 守护进程。'), findsOneWidget);
    expect(find.text('重试'), findsOneWidget);
  });

  testWidgets('dashboard uses the configured WebDAV base path', (tester) async {
    await tester.binding.setSurfaceSize(const Size(1100, 900));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.pumpWidget(
      DavDeckApp(api: FakeDaemonApi(publicBasePath: '/files')),
    );
    await tester.pumpAndSettle();
    expect(find.text('http://localhost:8080/files/'), findsOneWidget);
    expect(find.text('Not configured'), findsOneWidget);
  });

  testWidgets('dashboard uses the configured TLS hostname', (tester) async {
    await tester.binding.setSurfaceSize(const Size(1100, 900));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.pumpWidget(
      DavDeckApp(api: FakeDaemonApi(tlsHostname: 'dav.local')),
    );
    await tester.pumpAndSettle();
    expect(find.text('http://dav.local:8080/dav/'), findsOneWidget);
    expect(find.text('https://dav.local:8443/dav/'), findsOneWidget);
    expect(find.text('http://localhost:8080/dav/'), findsNothing);
  });

  testWidgets('dashboard disables start while the runtime is running', (
    tester,
  ) async {
    await tester.pumpWidget(DavDeckApp(api: FakeDaemonApi()));
    await tester.pumpAndSettle();

    final startButton = find.ancestor(
      of: find.text('Start'),
      matching: find.byType(FilledButton),
    );
    final stopButton = find.ancestor(
      of: find.text('Stop'),
      matching: find.byType(OutlinedButton),
    );
    final restartButton = find.ancestor(
      of: find.text('Restart'),
      matching: find.byType(OutlinedButton),
    );

    expect(startButton, findsOneWidget);
    expect(tester.widget<FilledButton>(startButton).onPressed, isNull);
    expect(tester.widget<OutlinedButton>(stopButton).onPressed, isNotNull);
    expect(tester.widget<OutlinedButton>(restartButton).onPressed, isNotNull);
  });

  testWidgets('dashboard enables only start when the runtime is stopped', (
    tester,
  ) async {
    await tester.pumpWidget(
      DavDeckApp(api: FakeDaemonApi(runtimeState: 'STOPPED')),
    );
    await tester.pumpAndSettle();

    final startButton = find.ancestor(
      of: find.text('Start'),
      matching: find.byType(FilledButton),
    );
    final stopButton = find.ancestor(
      of: find.text('Stop'),
      matching: find.byType(OutlinedButton),
    );
    final restartButton = find.ancestor(
      of: find.text('Restart'),
      matching: find.byType(OutlinedButton),
    );

    expect(tester.widget<FilledButton>(startButton).onPressed, isNotNull);
    expect(tester.widget<OutlinedButton>(stopButton).onPressed, isNull);
    expect(tester.widget<OutlinedButton>(restartButton).onPressed, isNull);
  });

  testWidgets('dashboard explains a stopped runtime and offers direct start', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1100, 900));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final api = FakeDaemonApi(
      runtimeState: 'STOPPED',
      endpointState: 'STOPPED',
    );
    await tester.pumpWidget(DavDeckApp(api: api));
    await tester.pumpAndSettle();

    expect(find.text('Service not started'), findsNWidgets(2));
    expect(
      find.text(
        'The daemon is ready, but Caddy and WebDAV are stopped. Select “Start WebDAV service” to enable access.',
      ),
      findsOneWidget,
    );
    expect(
      find.text('Select Start to enable the WebDAV service'),
      findsOneWidget,
    );

    await tester.tap(find.text('Start WebDAV service'));
    await tester.pumpAndSettle();
    expect(api.startCalls, 1);
  });

  testWidgets('dashboard disables runtime controls during startup', (
    tester,
  ) async {
    await tester.pumpWidget(
      DavDeckApp(api: FakeDaemonApi(runtimeState: 'STARTING')),
    );
    await tester.pumpAndSettle();

    final startButton = find.ancestor(
      of: find.text('Start'),
      matching: find.byType(FilledButton),
    );
    final stopButton = find.ancestor(
      of: find.text('Stop'),
      matching: find.byType(OutlinedButton),
    );
    final restartButton = find.ancestor(
      of: find.text('Restart'),
      matching: find.byType(OutlinedButton),
    );

    expect(tester.widget<FilledButton>(startButton).onPressed, isNull);
    expect(tester.widget<OutlinedButton>(stopButton).onPressed, isNull);
    expect(tester.widget<OutlinedButton>(restartButton).onPressed, isNull);
  });

  testWidgets('dashboard marks an unreachable endpoint as needing attention', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1100, 900));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.pumpWidget(
      DavDeckApp(
        api: FakeDaemonApi(tlsHostname: 'dav.local', endpointState: 'DEGRADED'),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Needs attention'), findsOneWidget);
    expect(find.text('System needs attention'), findsOneWidget);
    expect(find.textContaining('Unavailable'), findsNWidgets(2));
  });

  testWidgets('dashboard displays Caddy control failures', (tester) async {
    await tester.pumpWidget(
      DavDeckApp(
        api: FakeDaemonApi(
          runtimeState: 'STOPPED',
          startFailure: const DaemonApiException(
            'CADDY_NOT_FOUND',
            'Unable to start Caddy',
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('Start'));
    await tester.pumpAndSettle();
    expect(find.textContaining('Caddy action failed'), findsOneWidget);
    expect(find.textContaining('CADDY_NOT_FOUND'), findsOneWidget);
  });

  testWidgets('dashboard renders failed and unknown component states', (
    tester,
  ) async {
    await tester.pumpWidget(
      DavDeckApp(
        api: FakeDaemonApi(
          statusValue: const DaemonStatus(
            name: 'DavDeck',
            version: 'test',
            daemon: 'RUNNING',
            database: 'READY',
            schemaVersion: 1,
            caddy: 'FAILED',
            webdav: 'UNKNOWN',
            lastErrorCode: 'CADDY_START_FAILED',
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('Caddy: FAILED'), findsOneWidget);
    expect(find.text('WebDAV: UNKNOWN'), findsOneWidget);
    expect(find.textContaining('Service:'), findsNothing);
    expect(find.text('Last error: CADDY_START_FAILED'), findsOneWidget);
  });

  testWidgets('dashboard applies pending configuration through the API', (
    tester,
  ) async {
    final api = FakeDaemonApi(
      statusValue: const DaemonStatus(
        name: 'DavDeck',
        version: 'test',
        daemon: 'RUNNING',
        database: 'READY',
        schemaVersion: 1,
        pendingChanges: true,
      ),
    );
    await tester.pumpWidget(DavDeckApp(api: api));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Apply configuration'));
    await tester.pumpAndSettle();
    expect(api.applied, isTrue);
    expect(find.text('System needs attention'), findsOneWidget);
  });
}
