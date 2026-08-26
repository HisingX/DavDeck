import 'dart:io';

import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/state/logs_controller.dart';
import 'package:davdeck/widgets/app_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

typedef LogsTextAction = Future<void> Function(String content);
typedef LogsExportAction = Future<String> Function(String content);

class LogsPage extends StatefulWidget {
  const LogsPage({
    super.key,
    required this.controller,
    this.copyAction,
    this.exportAction,
    this.onOpenDiagnostics,
  });

  final LogsController controller;
  final LogsTextAction? copyAction;
  final LogsExportAction? exportAction;
  final VoidCallback? onOpenDiagnostics;

  @override
  State<LogsPage> createState() => _LogsPageState();
}

class _LogsPageState extends State<LogsPage> {
  late final TextEditingController componentController;
  late final TextEditingController queryController;

  LogsController get controller => widget.controller;

  @override
  void initState() {
    super.initState();
    componentController = TextEditingController(
      text: controller.componentFilter,
    );
    queryController = TextEditingController();
  }

  @override
  void dispose() {
    componentController.dispose();
    queryController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return Scaffold(
      body: AnimatedBuilder(
        animation: controller,
        builder: (context, _) => LayoutBuilder(
          builder: (context, constraints) {
            final maxWidth = constraints.maxWidth.clamp(0.0, 1240.0);
            return Column(
              children: [
                SingleChildScrollView(
                  padding: const EdgeInsets.fromLTRB(28, 28, 28, 0),
                  child: Center(
                    child: SizedBox(
                      width: maxWidth,
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          AppPageHeader(
                            title: strings.logs,
                            subtitle: strings.logsSubtitle,
                            actions: Wrap(
                              spacing: 4,
                              children: [
                                IconButton(
                                  onPressed: controller.refreshing
                                      ? null
                                      : controller.refresh,
                                  tooltip: strings.refreshLogs,
                                  icon: const Icon(Icons.refresh),
                                ),
                                IconButton(
                                  onPressed: () => _copy(context),
                                  tooltip: strings.copyLogs,
                                  icon: const Icon(Icons.copy_all_outlined),
                                ),
                                IconButton(
                                  onPressed: () => _export(context),
                                  tooltip: strings.exportLogs,
                                  icon: const Icon(
                                    Icons.file_download_outlined,
                                  ),
                                ),
                              ],
                            ),
                          ),
                          const SizedBox(height: 20),
                          _FilterBar(
                            controller: controller,
                            componentController: componentController,
                            queryController: queryController,
                            onApplyComponent: _applyComponentFilter,
                            onQueryChanged: (_) => setState(() {}),
                            strings: strings,
                          ),
                          if (controller.error != null &&
                              controller.records.isNotEmpty)
                            Padding(
                              padding: const EdgeInsets.only(top: 12),
                              child: _RefreshError(
                                onRetry: controller.refresh,
                                onOpenDiagnostics: widget.onOpenDiagnostics,
                                strings: strings,
                              ),
                            ),
                        ],
                      ),
                    ),
                  ),
                ),
                Expanded(child: _buildContent(context, strings, maxWidth)),
              ],
            );
          },
        ),
      ),
    );
  }

  Widget _buildContent(
    BuildContext context,
    AppStrings strings,
    double maxWidth,
  ) {
    final query = queryController.text.trim().toLowerCase();
    final records = query.isEmpty
        ? controller.records
        : controller.records
              .where(
                (record) =>
                    record.message.toLowerCase().contains(query) ||
                    record.component.toLowerCase().contains(query),
              )
              .toList(growable: false);
    if (controller.state == LogsLoadState.loading &&
        controller.records.isEmpty) {
      return Center(child: Text(strings.logsLoading));
    }
    if (controller.state == LogsLoadState.error && controller.records.isEmpty) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(strings.logsUnavailable),
            const SizedBox(height: 12),
            FilledButton(
              onPressed: controller.refresh,
              child: Text(strings.retry),
            ),
            if (widget.onOpenDiagnostics != null)
              TextButton.icon(
                onPressed: widget.onOpenDiagnostics,
                icon: const Icon(Icons.health_and_safety_outlined),
                label: Text(strings.openDiagnostics),
              ),
          ],
        ),
      );
    }
    if (records.isEmpty) {
      return Center(
        child: Text(query.isEmpty ? strings.noLogs : strings.noMatchingLogs),
      );
    }

    final itemCount =
        records.length + (query.isEmpty && controller.hasMore ? 1 : 0);
    return Stack(
      children: [
        ListView.builder(
          padding: const EdgeInsets.fromLTRB(28, 12, 28, 24),
          itemCount: itemCount + 1,
          itemBuilder: (context, index) {
            if (index == records.length) {
              if (query.isNotEmpty || !controller.hasMore) {
                return _LogsFooter(count: records.length, strings: strings);
              }
              return Padding(
                padding: const EdgeInsets.symmetric(vertical: 12),
                child: Center(
                  child: FilledButton.tonal(
                    onPressed: controller.loadingMore
                        ? null
                        : controller.loadMore,
                    child: Text(
                      controller.loadingMore
                          ? strings.logsLoading
                          : strings.loadMoreLogs,
                    ),
                  ),
                ),
              );
            }
            if (index == records.length + 1) {
              return _LogsFooter(count: records.length, strings: strings);
            }
            return Padding(
              padding: const EdgeInsets.only(bottom: 10),
              child: Center(
                child: ConstrainedBox(
                  constraints: BoxConstraints(maxWidth: maxWidth),
                  child: _LogCard(record: records[index], strings: strings),
                ),
              ),
            );
          },
        ),
        if (controller.refreshing)
          const Positioned(
            top: 0,
            left: 0,
            right: 0,
            child: LinearProgressIndicator(minHeight: 2),
          ),
      ],
    );
  }

  Future<void> _applyComponentFilter() async {
    await controller.setComponentFilter(componentController.text);
  }

  Future<void> _copy(BuildContext context) async {
    final strings = AppStrings.of(context);
    final action =
        widget.copyAction ??
        (content) => Clipboard.setData(ClipboardData(text: content));
    try {
      await action(controller.exportText());
      if (!context.mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(strings.logsCopied)));
    } catch (_) {
      if (!context.mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(strings.logsExportFailed)));
    }
  }

  Future<void> _export(BuildContext context) async {
    final strings = AppStrings.of(context);
    final content = controller.exportText();
    try {
      final path = widget.exportAction == null
          ? await exportLogsToTemporaryFile(content)
          : await widget.exportAction!(content);
      if (!context.mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(strings.logsExportedTo(path))));
    } catch (_) {
      if (!context.mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(strings.logsExportFailed)));
    }
  }
}

Future<String> exportLogsToTemporaryFile(String content) async {
  final fileName =
      'davdeck-logs-${DateTime.now().toUtc().microsecondsSinceEpoch}.json';
  final file = File(
    '${Directory.systemTemp.path}${Platform.pathSeparator}$fileName',
  );
  await file.writeAsString(content, flush: true);
  return file.path;
}

class _FilterBar extends StatelessWidget {
  const _FilterBar({
    required this.controller,
    required this.componentController,
    required this.queryController,
    required this.onApplyComponent,
    required this.onQueryChanged,
    required this.strings,
  });

  final LogsController controller;
  final TextEditingController componentController;
  final TextEditingController queryController;
  final VoidCallback onApplyComponent;
  final ValueChanged<String> onQueryChanged;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) => Wrap(
    spacing: 10,
    runSpacing: 10,
    crossAxisAlignment: WrapCrossAlignment.center,
    children: [
      AppSearchField(
        width: 280,
        controller: queryController,
        hintText: strings.searchLogsHint,
        clearTooltip: strings.clearSearch,
        onChanged: onQueryChanged,
      ),
      DropdownButton<String>(
        value: controller.levelFilter,
        onChanged: (value) {
          if (value != null) controller.setLevelFilter(value);
        },
        items: [
          DropdownMenuItem(value: '', child: Text(strings.allLevels)),
          for (final level in ['DEBUG', 'INFO', 'WARN', 'ERROR'])
            DropdownMenuItem(value: level, child: Text(level)),
        ],
      ),
      SizedBox(
        width: 220,
        child: TextField(
          controller: componentController,
          onSubmitted: (_) => onApplyComponent(),
          decoration: InputDecoration(
            labelText: strings.componentFilter,
            isDense: true,
          ),
        ),
      ),
      FilledButton.tonalIcon(
        onPressed: onApplyComponent,
        icon: const Icon(Icons.filter_alt_outlined),
        label: Text(strings.applyFilter),
      ),
      Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            controller.autoRefreshEnabled
                ? strings.autoRefresh
                : strings.pauseRefresh,
          ),
          Switch(
            value: controller.autoRefreshEnabled,
            onChanged: controller.setAutoRefresh,
          ),
        ],
      ),
    ],
  );
}

class _RefreshError extends StatelessWidget {
  const _RefreshError({
    required this.onRetry,
    required this.strings,
    this.onOpenDiagnostics,
  });

  final VoidCallback onRetry;
  final AppStrings strings;
  final VoidCallback? onOpenDiagnostics;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.fromLTRB(14, 8, 8, 8),
      decoration: BoxDecoration(
        color: theme.colorScheme.errorContainer,
        borderRadius: BorderRadius.circular(13),
      ),
      child: Row(
        children: [
          Icon(Icons.error_outline, color: theme.colorScheme.onErrorContainer),
          const SizedBox(width: 10),
          Expanded(
            child: Text(
              strings.logsUnavailable,
              style: TextStyle(color: theme.colorScheme.onErrorContainer),
            ),
          ),
          if (onOpenDiagnostics != null)
            TextButton(
              onPressed: onOpenDiagnostics,
              child: Text(strings.openDiagnostics),
            ),
          TextButton(onPressed: onRetry, child: Text(strings.retry)),
        ],
      ),
    );
  }
}

class _LogCard extends StatelessWidget {
  const _LogCard({required this.record, required this.strings});

  final ManagedLogRecord record;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final color = appStatusColor(context, record.level);
    return AppSurface(
      padding: EdgeInsets.zero,
      child: Column(
        children: [
          ListTile(
            contentPadding: const EdgeInsets.fromLTRB(18, 8, 14, 8),
            leading: Container(
              width: 10,
              height: 10,
              decoration: BoxDecoration(color: color, shape: BoxShape.circle),
            ),
            title: Text(
              record.message,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: theme.textTheme.titleMedium,
            ),
            subtitle: Padding(
              padding: const EdgeInsets.only(top: 5),
              child: Text(
                '${record.timestamp.toLocal().toIso8601String()} · ${record.component}',
              ),
            ),
            trailing: AppStatusPill(label: record.level, color: color),
          ),
          if (record.fields.isNotEmpty)
            Material(
              type: MaterialType.transparency,
              child: ExpansionTile(
                title: Text(strings.logDetails),
                children: [
                  for (final entry in record.fields.entries)
                    ListTile(
                      dense: true,
                      title: Text(entry.key),
                      subtitle: Text('${entry.value}'),
                    ),
                ],
              ),
            ),
        ],
      ),
    );
  }
}

class _LogsFooter extends StatelessWidget {
  const _LogsFooter({required this.count, required this.strings});

  final int count;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(top: 4, bottom: 18),
    child: Center(
      child: Text(
        strings.logsCount(count),
        style: Theme.of(context).textTheme.bodyMedium?.copyWith(
          color: Theme.of(context).colorScheme.onSurfaceVariant,
        ),
      ),
    ),
  );
}
