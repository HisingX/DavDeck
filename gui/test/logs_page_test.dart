import 'dart:io';

import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/logs/logs_page.dart';
import 'package:davdeck/state/logs_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';

class PageLogsApi implements LogsApi {
  final calls = <({String? level, String? component})>[];
  bool fail = false;
  bool empty = false;

  @override
  Future<ManagedLogPage> logs({
    int limit = 100,
    int? cursor,
    DateTime? since,
    String? level,
    String? component,
  }) async {
    calls.add((level: level, component: component));
    if (fail) throw const DaemonApiException('LOGS_UNAVAILABLE', 'offline');
    if (empty) return const ManagedLogPage();
    return ManagedLogPage(
      records: [
        ManagedLogRecord(
          id: 2,
          timestamp: DateTime.utc(2026, 8, 23, 1, 2, 3),
          level: 'ERROR',
          component: 'caddy',
          message: 'safe failure',
          fields: const {'error_code': 'CADDY_START_FAILED'},
        ),
        ManagedLogRecord(
          id: 1,
          timestamp: DateTime.utc(2026, 8, 23, 1, 2, 2),
          level: 'INFO',
          component: 'daemon',
          message: 'started',
        ),
      ],
    );
  }
}

Widget logsTestApp(LogsPage page) => MaterialApp(
  supportedLocales: const [Locale('en'), Locale('zh', 'CN')],
  localizationsDelegates: GlobalMaterialLocalizations.delegates,
  home: page,
);

void main() {
  test('temporary export writes the already sanitized text', () async {
    final path = await exportLogsToTemporaryFile('[]');
    addTearDown(() => File(path).delete());
    expect(await File(path).readAsString(), '[]');
  });

  testWidgets('logs page shows safe records, filters, copy, and export', (
    tester,
  ) async {
    final api = PageLogsApi();
    final controller = LogsController(api);
    addTearDown(controller.dispose);
    await controller.refresh();
    final copied = <String>[];
    final exported = <String>[];

    await tester.pumpWidget(
      logsTestApp(
        LogsPage(
          controller: controller,
          copyAction: (content) async => copied.add(content),
          exportAction: (content) async {
            exported.add(content);
            return '/tmp/davdeck-logs.json';
          },
        ),
      ),
    );
    expect(find.text('safe failure'), findsOneWidget);
    expect(find.text('started'), findsOneWidget);
    expect(find.text('Structured fields'), findsOneWidget);

    await tester.tap(find.byTooltip('Copy logs'));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Export logs'));
    await tester.pumpAndSettle();
    expect(copied.single, contains('safe failure'));
    expect(exported.single, contains('CADDY_START_FAILED'));
    expect(copied.single, isNot(contains('password')));
    expect(exported.single, isNot(contains('token')));

    await tester.tap(find.text('All levels'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('ERROR').last);
    await tester.pumpAndSettle();
    final field = find.byType(TextField);
    await tester.enterText(field, ' caddy ');
    await tester.tap(find.text('Apply filter'));
    await tester.pumpAndSettle();
    expect(api.calls.last, (level: 'ERROR', component: 'caddy'));

    await tester.tap(find.text('Caddy'));
    await tester.pumpAndSettle();
    expect(find.text('started'), findsNothing);

    await tester.tap(find.text('Clear'));
    await tester.pumpAndSettle();
    expect(find.text('No matching logs.'), findsOneWidget);
  });

  testWidgets('logs page renders empty and unavailable states', (tester) async {
    final api = PageLogsApi()..empty = true;
    final controller = LogsController(api);
    addTearDown(controller.dispose);
    await controller.refresh();
    await tester.pumpWidget(logsTestApp(LogsPage(controller: controller)));
    expect(find.text('No recent logs.'), findsOneWidget);

    api.empty = false;
    api.fail = true;
    await controller.refresh();
    await tester.pumpAndSettle();
    expect(find.text('Unable to load logs.'), findsOneWidget);
    expect(find.text('Retry'), findsOneWidget);

    var openedDiagnostics = false;
    await tester.pumpWidget(
      logsTestApp(
        LogsPage(
          controller: controller,
          onOpenDiagnostics: () => openedDiagnostics = true,
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('Open diagnostics'));
    expect(openedDiagnostics, isTrue);
  });
}
