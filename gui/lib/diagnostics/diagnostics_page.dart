import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/state/diagnostics_controller.dart';
import 'package:flutter/material.dart';

class DiagnosticsPage extends StatelessWidget {
  const DiagnosticsPage({super.key, required this.controller});
  final DiagnosticsController controller;

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return Scaffold(
      appBar: AppBar(
        title: Text(strings.diagnostics),
        actions: [
          Padding(
            padding: const EdgeInsets.only(right: 16),
            child: FilledButton.icon(
              onPressed: controller.running ? null : controller.run,
              icon: const Icon(Icons.health_and_safety_outlined),
              label: Text(strings.runDiagnostics),
            ),
          ),
        ],
      ),
      body: AnimatedBuilder(
        animation: controller,
        builder: (context, _) {
          if (controller.running && controller.report == null) {
            return Center(child: Text(strings.runningDiagnostics));
          }
          if (controller.error != null && controller.report == null) {
            return Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(strings.diagnosticsUnavailable),
                  const SizedBox(height: 12),
                  FilledButton(
                    onPressed: controller.run,
                    child: Text(strings.retry),
                  ),
                ],
              ),
            );
          }
          final report = controller.report;
          if (report == null) {
            return Center(child: Text(strings.diagnosticsNotRun));
          }
          return Stack(
            children: [
              ListView(
                padding: const EdgeInsets.all(24),
                children: [
                  _SummaryCard(report: report),
                  const SizedBox(height: 12),
                  Row(
                    children: [
                      const Icon(Icons.privacy_tip_outlined, size: 18),
                      const SizedBox(width: 8),
                      Expanded(child: Text(strings.sanitizedReportNotice)),
                    ],
                  ),
                  const SizedBox(height: 16),
                  ...report.results.map(
                    (result) => Card(
                      child: ListTile(
                        leading: Icon(
                          _icon(result.status),
                          color: _color(context, result.status),
                        ),
                        title: Text(result.title),
                        subtitle: Text(_details(strings, result)),
                        isThreeLine: result.code.isNotEmpty,
                        trailing: Text(result.status),
                      ),
                    ),
                  ),
                ],
              ),
              if (controller.running) const LinearProgressIndicator(),
            ],
          );
        },
      ),
    );
  }

  static IconData _icon(String status) => switch (status) {
    'PASS' => Icons.check_circle,
    'WARN' => Icons.warning_amber,
    'SKIP' => Icons.skip_next,
    _ => Icons.error,
  };

  static Color _color(BuildContext context, String status) => switch (status) {
    'PASS' => Colors.green,
    'WARN' => Colors.orange,
    'SKIP' => Colors.blueGrey,
    _ => Theme.of(context).colorScheme.error,
  };

  static String _details(AppStrings strings, DiagnosticResult result) {
    final parts = <String>[
      if (result.code.isNotEmpty) result.code,
      result.message,
    ];
    final hint = strings.diagnosticRemediation(result.code);
    if (hint.isNotEmpty) parts.add('${strings.suggestedAction}: $hint');
    return parts.join('\n');
  }
}

class _SummaryCard extends StatelessWidget {
  const _SummaryCard({required this.report});
  final DiagnosticReport report;

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Row(
          children: [
            Icon(
              DiagnosticsPage._icon(report.overall),
              size: 40,
              color: DiagnosticsPage._color(context, report.overall),
            ),
            const SizedBox(width: 16),
            Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  strings.diagnosticOverall(report.overall),
                  style: Theme.of(context).textTheme.titleLarge,
                ),
                Text('${strings.generatedAt}: ${report.generatedAt}'),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
