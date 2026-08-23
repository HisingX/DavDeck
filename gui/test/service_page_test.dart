import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/service/service_page.dart';
import 'package:davdeck/state/service_controller.dart';
import 'package:davdeck/state/status_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';

class ServicePageApi implements ServiceApi, DaemonApi {
  ManagedServiceStatus service = const ManagedServiceStatus(
    installed: false,
    state: 'NOT_INSTALLED',
  );
  Object? failure;
  final calls = <String>[];

  @override
  Future<DaemonStatus> status() async => DaemonStatus(
    name: 'DavDeck',
    version: 'test',
    daemon: 'RUNNING',
    database: 'READY',
    schemaVersion: 4,
    caddy: 'RUNNING',
    webdav: 'RUNNING',
    service: service,
    portableDaemonOwned: true,
  );

  @override
  Future<ManagedServiceStatus> serviceStatus() async {
    calls.add('status');
    if (failure != null) throw failure!;
    return service;
  }

  @override
  Future<void> installService() => _mutate('install');

  @override
  Future<void> uninstallService() => _mutate('uninstall');

  @override
  Future<void> startService() => _mutate('start');

  @override
  Future<void> stopService() => _mutate('stop');

  Future<void> _mutate(String operation) async {
    calls.add(operation);
    if (failure != null) throw failure!;
    service = ManagedServiceStatus(
      installed: operation != 'uninstall',
      state: operation == 'start' ? 'RUNNING' : 'STOPPED',
      startsAtBoot: operation == 'install',
    );
  }
}

Widget serviceTestApp(ServicePage page) => MaterialApp(
  supportedLocales: const [Locale('en'), Locale('zh', 'CN')],
  localizationsDelegates: GlobalMaterialLocalizations.delegates,
  home: page,
);

void main() {
  testWidgets('service page shows states, portable ownership, and install', (
    tester,
  ) async {
    final api = ServicePageApi();
    final status = StatusController(api);
    final service = ServiceController(api);
    addTearDown(status.dispose);
    addTearDown(service.dispose);
    await status.refresh();
    await service.refresh();
    await tester.pumpWidget(
      serviceTestApp(ServicePage(status: status, controller: service)),
    );

    expect(find.text('Daemon state'), findsOneWidget);
    expect(find.text('Daemon stateRUNNING'), findsNothing);
    expect(
      find.textContaining('The daemon is owned by the GUI'),
      findsOneWidget,
    );
    expect(find.text('Install service'), findsOneWidget);

    await tester.tap(find.text('Install service'));
    await tester.pumpAndSettle();
    expect(
      find.textContaining('This changes DavDeck’s system service state.'),
      findsOneWidget,
    );
    await tester.tap(find.text('Install service').last);
    await tester.pumpAndSettle();
    expect(api.calls, contains('install'));
    expect(find.text('Installed'), findsNWidgets(2));
  });

  testWidgets('service page requires confirmation before stopping', (
    tester,
  ) async {
    final api = ServicePageApi()
      ..service = const ManagedServiceStatus(
        installed: true,
        state: 'RUNNING',
        startsAtBoot: true,
      );
    final status = StatusController(api);
    final service = ServiceController(api);
    addTearDown(status.dispose);
    addTearDown(service.dispose);
    await status.refresh();
    await service.refresh();
    await tester.pumpWidget(
      serviceTestApp(ServicePage(status: status, controller: service)),
    );

    await tester.tap(find.text('Stop service'));
    await tester.pumpAndSettle();
    expect(
      find.textContaining('This changes DavDeck’s system service state.'),
      findsOneWidget,
    );
    expect(api.calls, isNot(contains('stop')));
    await tester.tap(find.text('Stop service').last);
    await tester.pumpAndSettle();
    expect(api.calls, contains('stop'));
  });

  testWidgets('service page exposes unavailable state and retry', (
    tester,
  ) async {
    final api = ServicePageApi()
      ..failure = const DaemonApiException(
        'SERVICE_STATUS_FAILED',
        'unavailable',
      );
    final status = StatusController(api);
    final service = ServiceController(api);
    addTearDown(status.dispose);
    addTearDown(service.dispose);
    await service.refresh();
    await tester.pumpWidget(
      serviceTestApp(ServicePage(status: status, controller: service)),
    );
    expect(find.text('Unable to read system service status.'), findsOneWidget);
    expect(find.text('Retry'), findsOneWidget);
  });

  testWidgets('service action failures link to diagnostics', (tester) async {
    final api = ServicePageApi();
    final status = StatusController(api);
    final service = ServiceController(api);
    addTearDown(status.dispose);
    addTearDown(service.dispose);
    await status.refresh();
    await service.refresh();
    api.failure = const DaemonApiException(
      'PRIVILEGE_REQUIRED',
      'administrator privileges are required',
    );
    var openedDiagnostics = false;
    await tester.pumpWidget(
      serviceTestApp(
        ServicePage(
          status: status,
          controller: service,
          onOpenDiagnostics: () => openedDiagnostics = true,
        ),
      ),
    );
    await tester.tap(find.text('Install service'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Install service').last);
    await tester.pumpAndSettle();
    expect(find.text('Open diagnostics'), findsOneWidget);
    await tester.tap(find.text('Open diagnostics'));
    expect(openedDiagnostics, isTrue);
  });
}
