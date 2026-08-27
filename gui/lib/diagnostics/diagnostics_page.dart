import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/state/diagnostics_controller.dart';
import 'package:davdeck/widgets/app_ui.dart';
import 'package:flutter/material.dart';

class DiagnosticsPage extends StatelessWidget {
  const DiagnosticsPage({super.key, required this.controller});

  final DiagnosticsController controller;

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return Scaffold(
      body: AnimatedBuilder(
        animation: controller,
        builder: (context, _) {
          if (controller.running && controller.report == null) {
            return Center(child: Text(strings.runningDiagnostics));
          }
          if (controller.error != null && controller.report == null) {
            return _ErrorState(strings: strings, onRetry: controller.run);
          }
          final report = controller.report;
          if (report == null) {
            return _NotRunState(strings: strings, onRun: controller.run);
          }
          return Stack(
            children: [
              SingleChildScrollView(
                padding: appPagePadding(context),
                child: Center(
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(maxWidth: 1120),
                    child: _DiagnosticsContent(
                      report: report,
                      strings: strings,
                      running: controller.running,
                      onRun: controller.run,
                    ),
                  ),
                ),
              ),
              if (controller.running)
                const Positioned(
                  top: 0,
                  left: 0,
                  right: 0,
                  child: LinearProgressIndicator(minHeight: 2),
                ),
            ],
          );
        },
      ),
    );
  }

  static IconData iconFor(String status) => switch (status.toUpperCase()) {
    'PASS' => Icons.check,
    'WARN' => Icons.priority_high,
    'SKIP' => Icons.remove,
    _ => Icons.close,
  };

  static Color colorFor(BuildContext context, String status) =>
      appStatusColor(context, status);

  static String details(AppStrings strings, DiagnosticResult result) {
    final parts = <String>[
      if (result.code.isNotEmpty) result.code,
      result.message,
    ];
    final hint = strings.diagnosticRemediation(result.code);
    if (hint.isNotEmpty) parts.add('${strings.suggestedAction}: $hint');
    return parts.join('\n');
  }
}

class _DiagnosticsContent extends StatelessWidget {
  const _DiagnosticsContent({
    required this.report,
    required this.strings,
    required this.running,
    required this.onRun,
  });

  final DiagnosticReport report;
  final AppStrings strings;
  final bool running;
  final VoidCallback onRun;

  @override
  Widget build(BuildContext context) {
    final warnings = report.results
        .where((item) => item.status == 'WARN')
        .length;
    final passed = report.results.where((item) => item.status == 'PASS').length;
    final theme = Theme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        AppPageHeader(
          title: strings.diagnostics,
          subtitle: strings.diagnosticsSubtitle,
          actions: FilledButton.icon(
            onPressed: running ? null : onRun,
            icon: const Icon(Icons.health_and_safety_outlined),
            label: Text(strings.runDiagnostics),
          ),
        ),
        const SizedBox(height: 24),
        _SummaryCard(
          report: report,
          warnings: warnings,
          passed: passed,
          strings: strings,
        ),
        const SizedBox(height: 14),
        AppNotice(
          icon: Icons.info_outline,
          text: strings.sanitizedReportNotice,
          color: theme.colorScheme.primaryContainer.withValues(alpha: 0.32),
          textColor: theme.colorScheme.onPrimaryContainer,
        ),
        const SizedBox(height: 18),
        ...report.results.map(
          (result) => Padding(
            padding: const EdgeInsets.only(bottom: 12),
            child: _DiagnosticCard(result: result, strings: strings),
          ),
        ),
      ],
    );
  }
}

class _SummaryCard extends StatelessWidget {
  const _SummaryCard({
    required this.report,
    required this.warnings,
    required this.passed,
    required this.strings,
  });

  final DiagnosticReport report;
  final int warnings;
  final int passed;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final color = DiagnosticsPage.colorFor(context, report.overall);
    return AppSurface(
      color: color.withValues(alpha: 0.055),
      padding: const EdgeInsets.all(24),
      child: LayoutBuilder(
        builder: (context, constraints) {
          final compact = constraints.maxWidth < 780;
          final icon = Container(
            width: 72,
            height: 72,
            decoration: BoxDecoration(
              color: theme.colorScheme.surface,
              shape: BoxShape.circle,
              border: Border.all(
                color: color.withValues(alpha: 0.32),
                width: 2,
              ),
            ),
            child: Icon(
              DiagnosticsPage.iconFor(report.overall),
              color: color,
              size: 34,
            ),
          );
          final title = Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                strings.diagnosticOverall(report.overall),
                style: theme.textTheme.titleLarge?.copyWith(
                  fontWeight: FontWeight.w700,
                ),
              ),
              const SizedBox(height: 7),
              Text('${strings.generatedAt}: ${report.generatedAt}'),
            ],
          );
          final counts = Wrap(
            spacing: 18,
            runSpacing: 8,
            children: [
              _SummaryCount(
                icon: Icons.warning_amber_rounded,
                value: warnings,
                label: strings.diagnosticWarnings,
                color: const Color(0xffb87800),
              ),
              _SummaryCount(
                icon: Icons.check_circle,
                value: passed,
                label: strings.diagnosticPassed,
                color: theme.colorScheme.primary,
              ),
            ],
          );
          if (compact) {
            return Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(children: [icon, const SizedBox(width: 16), title]),
                const SizedBox(height: 18),
                counts,
              ],
            );
          }
          return Row(
            children: [
              icon,
              const SizedBox(width: 18),
              title,
              const Spacer(),
              counts,
            ],
          );
        },
      ),
    );
  }
}

class _SummaryCount extends StatelessWidget {
  const _SummaryCount({
    required this.icon,
    required this.value,
    required this.label,
    required this.color,
  });

  final IconData icon;
  final int value;
  final String label;
  final Color color;

  @override
  Widget build(BuildContext context) => Row(
    mainAxisSize: MainAxisSize.min,
    children: [
      Icon(icon, size: 20, color: color),
      const SizedBox(width: 7),
      Text('$value $label'),
    ],
  );
}

class _DiagnosticCard extends StatelessWidget {
  const _DiagnosticCard({required this.result, required this.strings});

  final DiagnosticResult result;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final color = DiagnosticsPage.colorFor(context, result.status);
    return AppSurface(
      padding: const EdgeInsets.fromLTRB(18, 16, 18, 16),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 48,
            height: 48,
            decoration: BoxDecoration(
              color: color.withValues(alpha: 0.10),
              shape: BoxShape.circle,
            ),
            child: Icon(DiagnosticsPage.iconFor(result.status), color: color),
          ),
          const SizedBox(width: 16),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  result.title,
                  style: theme.textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.w700,
                  ),
                ),
                const SizedBox(height: 6),
                Text(
                  DiagnosticsPage.details(strings, result),
                  style: theme.textTheme.bodyMedium?.copyWith(height: 1.45),
                ),
              ],
            ),
          ),
          const SizedBox(width: 12),
          AppStatusPill(label: result.status, color: color),
        ],
      ),
    );
  }
}

class _NotRunState extends StatelessWidget {
  const _NotRunState({required this.strings, required this.onRun});

  final AppStrings strings;
  final VoidCallback onRun;

  @override
  Widget build(BuildContext context) => Center(
    child: Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(
          Icons.health_and_safety_outlined,
          size: 42,
          color: Theme.of(context).colorScheme.onSurfaceVariant,
        ),
        const SizedBox(height: 12),
        Text(strings.diagnosticsNotRun),
        const SizedBox(height: 12),
        FilledButton.icon(
          onPressed: onRun,
          icon: const Icon(Icons.play_arrow_outlined),
          label: Text(strings.runDiagnostics),
        ),
      ],
    ),
  );
}

class _ErrorState extends StatelessWidget {
  const _ErrorState({required this.strings, required this.onRetry});

  final AppStrings strings;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) => Center(
    child: Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(strings.diagnosticsUnavailable),
        const SizedBox(height: 12),
        FilledButton(onPressed: onRetry, child: Text(strings.retry)),
      ],
    ),
  );
}
