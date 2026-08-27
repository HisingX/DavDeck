import 'package:davdeck/api/daemon_api.dart';
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

Widget tlsTestApp(TlsController controller) => MaterialApp(
  supportedLocales: const [Locale('en'), Locale('zh', 'CN')],
  localizationsDelegates: GlobalMaterialLocalizations.delegates,
  home: TlsPage(controller: controller),
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
    await tester.pumpWidget(tlsTestApp(controller));
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
    await tester.tap(find.text('Save HTTPS settings'));
    await tester.pumpAndSettle();
    expect(api.profile?.mode, 'custom');
    expect(api.profile?.privateKeyPath, '/key.pem');
    expect(find.text('Apply configuration'), findsOneWidget);
    await tester.tap(find.text('Run preflight'));
    await tester.pumpAndSettle();
    expect(find.text('Preflight passed'), findsOneWidget);
    expect(find.text('Certificate and private key match'), findsOneWidget);
    await tester.tap(find.text('Apply configuration'));
    await tester.pumpAndSettle();
    expect(api.applied, isTrue);
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

      await tester.tap(find.text('Save HTTPS settings'));
      await tester.pumpAndSettle();

      expect(find.text('Configure a secure connection'), findsOneWidget);
      expect(find.textContaining('TLS_CONFIGURATION_ERROR'), findsOneWidget);
      expect(find.textContaining('hostname is required'), findsOneWidget);
      expect(find.text('Retry'), findsOneWidget);
    },
  );
}
