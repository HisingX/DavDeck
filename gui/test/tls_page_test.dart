import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/state/status_controller.dart';
import 'package:davdeck/state/tls_controller.dart';
import 'package:davdeck/tls/tls_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';

class FakeTlsApi implements TlsApi, ConfigurationApi {
  ManagedTlsProfile? profile;
  Object? getFailure;
  Object? updateFailure;
  var applied = false;

  @override
  Future<ManagedTlsProfile?> getTls() async {
    if (getFailure != null) throw getFailure!;
    return profile;
  }

  @override
  Future<ManagedTlsProfile> updateTls({
    required String mode,
    required String hostname,
    String certificatePath = '',
    String privateKeyPath = '',
  }) async {
    if (updateFailure != null) throw updateFailure!;
    profile = ManagedTlsProfile(
      id: 'tls-1',
      mode: mode,
      hostname: hostname,
      certificatePath: certificatePath,
      privateKeyPath: privateKeyPath,
    );
    return profile!;
  }

  @override
  Future<void> disableTls() async {
    profile = null;
  }

  @override
  Future<ManagedTlsCheckResult> checkTls() async => const ManagedTlsCheckResult(
    ready: true,
    checks: [
      ManagedTlsCheck(
        name: 'certificate_pair',
        ok: true,
        message: 'Certificate and private key match',
      ),
    ],
  );

  @override
  Future<void> applyConfiguration() async {
    applied = true;
  }
}

class FakeStatusApi implements DaemonApi {
  FakeStatusApi({this.pendingChanges = false});

  var statusCalls = 0;
  final bool pendingChanges;

  @override
  Future<DaemonStatus> status() async {
    statusCalls++;
    return DaemonStatus(
      name: 'DavDeck',
      version: 'test',
      daemon: 'RUNNING',
      database: 'READY',
      schemaVersion: 1,
      pendingChanges: pendingChanges,
    );
  }
}

Widget tlsTestApp(TlsController controller, {StatusController? status}) =>
    MaterialApp(
      supportedLocales: const [Locale('en'), Locale('zh', 'CN')],
      localizationsDelegates: GlobalMaterialLocalizations.delegates,
      home: TlsPage(controller: controller, status: status),
    );

void main() {
  testWidgets('TLS wizard explains trust and shows custom certificate fields', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1100, 900));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final api = FakeTlsApi();
    final controller = TlsController(api, api);
    await controller.refresh();
    addTearDown(controller.dispose);
    await tester.pumpWidget(tlsTestApp(controller));
    await tester.pumpAndSettle();
    expect(
      find.ancestor(
        of: find.text('Save HTTPS settings'),
        matching: find.byType(FilledButton),
      ),
      findsNothing,
    );
    expect(
      find.ancestor(
        of: find.text('Save HTTPS settings'),
        matching: find.byType(OutlinedButton),
      ),
      findsOneWidget,
    );
    expect(find.text('Automatic'), findsOneWidget);
    expect(find.text('Internal'), findsOneWidget);
    expect(find.text('Custom'), findsOneWidget);
    expect(find.byType(ChoiceChip), findsNWidgets(3));
    expect(find.byType(SegmentedButton<String>), findsNothing);
    expect(find.textContaining('internal root certificate'), findsOneWidget);
    await tester.tap(find.text('Custom'));
    await tester.pumpAndSettle();
    expect(find.text('Certificate absolute path'), findsOneWidget);
    expect(find.text('Private-key absolute path'), findsOneWidget);
  });

  testWidgets('TLS wizard saves, preflights, and explicitly applies', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1100, 1000));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final api = FakeTlsApi();
    final controller = TlsController(api, api);
    await controller.refresh();
    addTearDown(controller.dispose);
    final statusApi = FakeStatusApi();
    final status = StatusController(statusApi);
    await status.refresh();
    addTearDown(status.dispose);
    await tester.pumpWidget(tlsTestApp(controller, status: status));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Custom'));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.widgetWithText(TextField, 'Hostname'),
      'dav.example.com',
    );
    await tester.enterText(
      find.widgetWithText(TextField, 'Certificate absolute path'),
      '/cert.pem',
    );
    await tester.enterText(
      find.widgetWithText(TextField, 'Private-key absolute path'),
      '/key.pem',
    );
    expect(
      find.ancestor(
        of: find.text('Save HTTPS settings'),
        matching: find.byType(FilledButton),
      ),
      findsOneWidget,
    );
    await tester.tap(find.text('Save HTTPS settings'));
    await tester.pumpAndSettle();
    expect(api.profile?.mode, 'custom');
    expect(api.profile?.privateKeyPath, '/key.pem');
    expect(find.text('Apply configuration'), findsOneWidget);
    expect(
      find.ancestor(
        of: find.text('Apply configuration'),
        matching: find.byType(FilledButton),
      ),
      findsOneWidget,
    );
    expect(
      find.ancestor(
        of: find.text('Save HTTPS settings'),
        matching: find.byType(OutlinedButton),
      ),
      findsOneWidget,
    );
    await tester.tap(find.text('Run preflight'));
    await tester.pumpAndSettle();
    expect(find.text('Preflight passed'), findsOneWidget);
    expect(find.text('Certificate and private key match'), findsOneWidget);
    await tester.tap(find.text('Apply configuration'));
    await tester.pumpAndSettle();
    expect(api.applied, isTrue);
    expect(statusApi.statusCalls, 3);
    expect(find.text('Apply configuration'), findsNothing);
  });

  testWidgets(
    'TLS save failure keeps the form and shows the actionable error',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(1100, 900));
      addTearDown(() => tester.binding.setSurfaceSize(null));
      final api = FakeTlsApi()
        ..updateFailure = const DaemonApiException(
          'TLS_CONFIGURATION_ERROR',
          'hostname is required',
        );
      final controller = TlsController(api, api);
      await controller.refresh();
      addTearDown(controller.dispose);
      await tester.pumpWidget(tlsTestApp(controller));
      await tester.pumpAndSettle();

      await tester.enterText(
        find.widgetWithText(TextField, 'Hostname'),
        'invalid.example.com',
      );
      await tester.pump();
      expect(find.text('invalid.example.com'), findsOneWidget);
      expect(
        find.ancestor(
          of: find.text('Save HTTPS settings'),
          matching: find.byType(FilledButton),
        ),
        findsOneWidget,
      );
      await tester.tap(find.text('Save HTTPS settings'));
      await tester.pumpAndSettle();

      expect(find.text('Configure a secure connection'), findsOneWidget);
      expect(find.textContaining('TLS_CONFIGURATION_ERROR'), findsOneWidget);
      expect(find.textContaining('hostname is required'), findsOneWidget);
      expect(find.text('Retry'), findsOneWidget);
    },
  );

  testWidgets('TLS wizard can disable HTTPS', (tester) async {
    await tester.binding.setSurfaceSize(const Size(1100, 900));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final api = FakeTlsApi()
      ..profile = const ManagedTlsProfile(
        id: 'tls-1',
        mode: 'internal',
        hostname: 'dav.local',
        certificatePath: '',
        privateKeyPath: '',
      );
    final controller = TlsController(api, api);
    await controller.refresh();
    addTearDown(controller.dispose);
    final statusApi = FakeStatusApi();
    final status = StatusController(statusApi);
    await status.refresh();
    addTearDown(status.dispose);
    await tester.pumpWidget(tlsTestApp(controller, status: status));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Disable HTTPS'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Disable HTTPS').last);
    await tester.pumpAndSettle();

    expect(api.profile, isNull);
    expect(controller.pendingApply, isTrue);
    expect(statusApi.statusCalls, 2);
    expect(find.text('Not configured'), findsOneWidget);
    expect(
      find.ancestor(
        of: find.text('Apply configuration'),
        matching: find.byType(FilledButton),
      ),
      findsOneWidget,
    );
  });

  testWidgets('TLS page restores pending apply state from shared status', (
    tester,
  ) async {
    final api = FakeTlsApi()
      ..profile = const ManagedTlsProfile(
        id: 'tls-1',
        mode: 'internal',
        hostname: 'dav.local',
        certificatePath: '',
        privateKeyPath: '',
      );
    final controller = TlsController(api, api);
    await controller.refresh();
    addTearDown(controller.dispose);
    final status = StatusController(FakeStatusApi(pendingChanges: true));
    await status.refresh();
    addTearDown(status.dispose);

    await tester.pumpWidget(tlsTestApp(controller, status: status));
    await tester.pumpAndSettle();

    expect(controller.pendingApply, isFalse);
    expect(find.text('Apply configuration'), findsOneWidget);
    expect(
      find.ancestor(
        of: find.text('Apply configuration'),
        matching: find.byType(FilledButton),
      ),
      findsOneWidget,
    );
  });

  testWidgets('TLS page requires saving new edits before applying', (
    tester,
  ) async {
    final api = FakeTlsApi()
      ..profile = const ManagedTlsProfile(
        id: 'tls-1',
        mode: 'internal',
        hostname: 'dav.local',
        certificatePath: '',
        privateKeyPath: '',
      );
    final controller = TlsController(api, api);
    await controller.refresh();
    controller.pendingApply = true;
    addTearDown(controller.dispose);

    await tester.pumpWidget(tlsTestApp(controller));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.widgetWithText(TextField, 'Hostname'),
      'dav.changed.local',
    );
    await tester.pump();

    final applyButton = find.ancestor(
      of: find.text('Apply configuration'),
      matching: find.byType(OutlinedButton),
    );
    final saveButton = find.ancestor(
      of: find.text('Save HTTPS settings'),
      matching: find.byType(FilledButton),
    );

    expect(tester.widget<OutlinedButton>(applyButton).onPressed, isNull);
    expect(tester.widget<FilledButton>(saveButton).onPressed, isNotNull);
  });
}
