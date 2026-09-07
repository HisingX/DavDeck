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
  Future<ManagedTlsProfile> renewTls() async {
    if (profile == null) throw StateError('TLS is not configured');
    profile = ManagedTlsProfile(
      id: profile!.id,
      mode: profile!.mode,
      hostname: profile!.hostname,
      certificatePath: profile!.certificatePath,
      privateKeyPath: profile!.privateKeyPath,
      challenge: profile!.challenge,
      dnsProviderId: profile!.dnsProviderId,
      certificateStatus: const ManagedCertificateStatus(
        state: 'ISSUING',
        storagePath: '/Users/test/Library/Application Support/Caddy',
        message: 'Caddy is renewing the certificate',
        renewal: true,
      ),
    );
    return profile!;
  }

  @override
  Future<ManagedTlsProfile> cancelTlsRenewal() async {
    if (profile == null) throw StateError('TLS is not configured');
    profile = ManagedTlsProfile(
      id: profile!.id,
      mode: profile!.mode,
      hostname: profile!.hostname,
      certificatePath: profile!.certificatePath,
      privateKeyPath: profile!.privateKeyPath,
      challenge: profile!.challenge,
      dnsProviderId: profile!.dnsProviderId,
      certificateStatus: const ManagedCertificateStatus(
        state: 'READY',
        storagePath: '/Users/test/Library/Application Support/Caddy',
        message: 'Certificate is ready',
      ),
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

class FakeDnsProviderApi implements DnsProviderApi {
  List<ManagedDnsProvider> providers = const [];
  Map<String, String>? savedSecret;

  @override
  Future<List<ManagedDnsProvider>> listDnsProviders() async => providers;

  @override
  Future<ManagedDnsProvider> saveDnsProvider({
    String? id,
    required String name,
    required String provider,
    List<String> allowedZones = const [],
    Map<String, String>? secret,
  }) async {
    savedSecret = secret;
    final result = ManagedDnsProvider(
      id: id ?? 'dns-1',
      name: name,
      provider: provider,
      allowedZones: allowedZones,
      secretConfigured: secret != null || providers.isNotEmpty,
    );
    providers = [result];
    return result;
  }

  @override
  Future<void> deleteDnsProvider(String id) async {
    providers = providers.where((provider) => provider.id != id).toList();
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

  testWidgets('automatic TLS shows certificate issuance status and storage', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1100, 1000));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final api = FakeTlsApi()
      ..profile = ManagedTlsProfile(
        id: 'tls-1',
        mode: 'automatic',
        hostname: 'dav.example.com',
        certificatePath: '',
        privateKeyPath: '',
        certificateStatus: const ManagedCertificateStatus(
          state: 'ISSUING',
          storagePath: '/Users/test/Library/Application Support/Caddy',
          certificatePath:
              '/Users/test/Library/Application Support/Caddy/certificate.crt',
          message: 'Caddy is requesting the certificate',
        ),
      );
    final controller = TlsController(api, api);
    controller.profile = api.profile;
    controller.loading = false;
    addTearDown(controller.dispose);

    await tester.pumpWidget(tlsTestApp(controller));
    await tester.pump();

    expect(find.text('Issuing'), findsNWidgets(2));
    expect(find.textContaining('requesting or renewing'), findsOneWidget);
    expect(
      find.text('/Users/test/Library/Application Support/Caddy'),
      findsOneWidget,
    );
    expect(find.textContaining('certificate.crt'), findsOneWidget);
    expect(find.byType(LinearProgressIndicator), findsOneWidget);
  });

  testWidgets('automatic TLS can renew an issued certificate', (tester) async {
    await tester.binding.setSurfaceSize(const Size(1100, 1000));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final api = FakeTlsApi()
      ..profile = const ManagedTlsProfile(
        id: 'tls-1',
        mode: 'automatic',
        hostname: 'dav.example.com',
        certificatePath: '',
        privateKeyPath: '',
        certificateStatus: ManagedCertificateStatus(
          state: 'READY',
          storagePath: '/Users/test/Library/Application Support/Caddy',
          message: 'ready',
        ),
      );
    final controller = TlsController(api, api)
      ..profile = api.profile
      ..loading = false;

    await tester.pumpWidget(tlsTestApp(controller));
    await tester.pump();
    expect(find.text('Renew certificate'), findsOneWidget);
    await tester.tap(find.text('Renew certificate'));
    await tester.pumpAndSettle();
    expect(
      find.textContaining('Renew the current public certificate'),
      findsOneWidget,
    );
    await tester.tap(find.text('Renew certificate').last);
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text('Issuing'), findsNWidgets(2));
    expect(find.textContaining('renewing the certificate'), findsOneWidget);
    controller.dispose();
  });

  testWidgets('automatic TLS can cancel an ongoing certificate renewal', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1100, 1000));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final api = FakeTlsApi()
      ..profile = const ManagedTlsProfile(
        id: 'tls-1',
        mode: 'automatic',
        hostname: 'dav.example.com',
        certificatePath: '',
        privateKeyPath: '',
        certificateStatus: ManagedCertificateStatus(
          state: 'ISSUING',
          storagePath: '/Users/test/Library/Application Support/Caddy',
          message: 'renewing',
          renewal: true,
        ),
      );
    final controller = TlsController(api, api)
      ..profile = api.profile
      ..loading = false;
    addTearDown(controller.dispose);

    await tester.pumpWidget(tlsTestApp(controller));
    await tester.pump();
    expect(find.text('Cancel certificate renewal'), findsOneWidget);
    await tester.tap(find.text('Cancel certificate renewal'));
    await tester.pump(const Duration(milliseconds: 300));
    expect(
      find.textContaining('The existing certificate and HTTPS configuration'),
      findsOneWidget,
    );
    await tester.tap(find.text('Cancel certificate renewal').last);
    await tester.pumpAndSettle();

    expect(api.profile?.certificateStatus?.state, 'READY');
    expect(api.profile?.certificateStatus?.renewal, isFalse);
    expect(find.text('Renew certificate'), findsOneWidget);
  });

  testWidgets('switching certificate modes preserves the DNS-01 strategy', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1100, 1000));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final api = FakeTlsApi()
      ..profile = const ManagedTlsProfile(
        id: 'tls-1',
        mode: 'automatic',
        hostname: 'dav.example.com',
        certificatePath: '',
        privateKeyPath: '',
        challenge: 'dns',
        dnsProviderId: 'dns-1',
      );
    final dnsApi = FakeDnsProviderApi()
      ..providers = const [
        ManagedDnsProvider(
          id: 'dns-1',
          name: 'DNSPOD',
          provider: 'dnspod',
          allowedZones: ['example.com'],
          secretConfigured: true,
        ),
      ];
    final controller = TlsController(api, api, dnsProviderApi: dnsApi);
    await controller.refresh();
    addTearDown(controller.dispose);

    await tester.pumpWidget(tlsTestApp(controller));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey<String>('challenge-dns')), findsOneWidget);
    expect(find.text('DNSPOD'), findsOneWidget);

    await tester.tap(find.text('Internal'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Automatic'));
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey<String>('challenge-dns')), findsOneWidget);
    expect(find.text('DNSPOD'), findsOneWidget);
  });

  testWidgets('automatic TLS can cancel an ongoing certificate request', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1100, 1000));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final api = FakeTlsApi()
      ..profile = const ManagedTlsProfile(
        id: 'tls-1',
        mode: 'automatic',
        hostname: 'dav.example.com',
        certificatePath: '',
        privateKeyPath: '',
        certificateStatus: ManagedCertificateStatus(
          state: 'ISSUING',
          storagePath: '/Users/test/Library/Application Support/Caddy',
          message: 'issuing',
        ),
      );
    final controller = TlsController(api, api)
      ..profile = api.profile
      ..loading = false;
    addTearDown(controller.dispose);

    await tester.pumpWidget(tlsTestApp(controller));
    await tester.pump();
    await tester.tap(find.text('Cancel certificate request'));
    await tester.pump(const Duration(milliseconds: 300));
    await tester.tap(find.text('Cancel certificate request').last);
    await tester.pumpAndSettle();

    expect(api.profile, isNull);
    expect(api.applied, isTrue);
    expect(find.text('Not configured'), findsOneWidget);
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

  testWidgets('TLS DNS-01 page opens the DNS provider manager', (tester) async {
    await tester.binding.setSurfaceSize(const Size(1100, 1000));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final api = FakeTlsApi();
    final dnsApi = FakeDnsProviderApi();
    final controller = TlsController(api, api, dnsProviderApi: dnsApi);
    await controller.refresh();
    addTearDown(controller.dispose);

    await tester.pumpWidget(tlsTestApp(controller));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Automatic'));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey<String>('challenge-auto')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('DNS-01').last);
    await tester.pumpAndSettle();

    expect(find.text('Manage DNS providers'), findsOneWidget);
    await tester.tap(find.text('Manage DNS providers'));
    await tester.pumpAndSettle();
    expect(
      find.textContaining('No DNS provider credential is configured.').last,
      findsOneWidget,
    );
    expect(
      find.widgetWithText(FilledButton, 'Add DNS provider'),
      findsOneWidget,
    );

    await tester.tap(find.widgetWithText(FilledButton, 'Add DNS provider'));
    await tester.pumpAndSettle();
    expect(find.text('Add DNS provider'), findsNWidgets(2));
    expect(find.text('Access credential'), findsOneWidget);
  });

  testWidgets(
    'DNS provider manager saves credentials without displaying them',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(1100, 1000));
      addTearDown(() => tester.binding.setSurfaceSize(null));
      final api = FakeTlsApi();
      final dnsApi = FakeDnsProviderApi();
      final controller = TlsController(api, api, dnsProviderApi: dnsApi);
      await controller.refresh();
      addTearDown(controller.dispose);

      await tester.pumpWidget(tlsTestApp(controller));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Automatic'));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey<String>('challenge-auto')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('DNS-01').last);
      await tester.pumpAndSettle();
      await tester.tap(find.text('Manage DNS providers'));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(FilledButton, 'Add DNS provider'));
      await tester.pumpAndSettle();

      await tester.enterText(
        find.widgetWithText(TextFormField, 'Configuration name'),
        'Production Cloudflare',
      );
      await tester.enterText(
        find.widgetWithText(TextFormField, 'API token'),
        'cf-secret-value',
      );
      await tester.tap(find.widgetWithText(FilledButton, 'Save'));
      await tester.pumpAndSettle();

      expect(dnsApi.savedSecret, {'api_token': 'cf-secret-value'});
      expect(find.text('Production Cloudflare'), findsOneWidget);
      expect(find.text('cf-secret-value'), findsNothing);
    },
  );
}
