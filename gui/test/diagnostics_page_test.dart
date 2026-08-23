import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/diagnostics/diagnostics_page.dart';
import 'package:davdeck/state/diagnostics_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';

class FakeDiagnosticsApi implements DiagnosticsApi {
  @override
  Future<DiagnosticReport> runDiagnostics() async => const DiagnosticReport(
    generatedAt: '2026-08-20T01:02:03Z',
    overall: 'WARN',
    sanitized: true,
    results: [
      DiagnosticResult(
        id: 'database',
        title: 'Database',
        status: 'PASS',
        code: '',
        message: 'SQLite is ready',
      ),
      DiagnosticResult(
        id: 'tls',
        title: 'TLS',
        status: 'WARN',
        code: 'TLS_CONFIGURATION_ERROR',
        message: 'TLS is not configured',
      ),
    ],
  );
}

void main() {
  testWidgets('diagnostics page runs and explains sanitized results', (
    tester,
  ) async {
    final controller = DiagnosticsController(FakeDiagnosticsApi());
    addTearDown(controller.dispose);
    await tester.pumpWidget(
      MaterialApp(
        supportedLocales: const [Locale('en'), Locale('zh', 'CN')],
        localizationsDelegates: GlobalMaterialLocalizations.delegates,
        home: DiagnosticsPage(controller: controller),
      ),
    );
    expect(find.text('Diagnostics have not been run yet.'), findsOneWidget);
    await tester.tap(find.text('Run diagnostics'));
    await tester.pumpAndSettle();
    expect(find.text('Overall status: WARN'), findsOneWidget);
    expect(find.text('SQLite is ready'), findsOneWidget);
    expect(find.textContaining('management tokens'), findsOneWidget);
    expect(find.textContaining('TLS_CONFIGURATION_ERROR'), findsOneWidget);
    expect(
      find.textContaining(
        'Run HTTPS preflight and correct the certificate settings.',
      ),
      findsOneWidget,
    );
  });
}
