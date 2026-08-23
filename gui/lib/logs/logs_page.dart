import 'dart:io';

import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/state/logs_controller.dart';
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

  LogsController get controller => widget.controller;

  @override
  void initState() {
    super.initState();
    componentController = TextEditingController(
      text: controller.componentFilter,
    );
  }

  @override
  void dispose() {
    componentController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return Scaffold(
      appBar: AppBar(
        title: Text(strings.logs),
        actions: [
          IconButton(
            onPressed: controller.refreshing ? null : controller.refresh,
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
            icon: const Icon(Icons.file_download_outlined),
          ),
          const SizedBox(width: 8),
        ],
      ),
      body: AnimatedBuilder(
        animation: controller,
        builder: (context, _) => Column(
          children: [
            _FilterBar(
              controller: controller,
              componentController: componentController,
              onApplyComponent: _applyComponentFilter,
              strings: strings,
            ),
            if (controller.error != null && controller.records.isNotEmpty)
              _RefreshError(
                onRetry: controller.refresh,
                onOpenDiagnostics: widget.onOpenDiagnostics,
                strings: strings,
              ),
            Expanded(child: _buildContent(context, strings)),
          ],
        ),
      ),
    );
  }

  Widget _buildContent(BuildContext context, AppStrings strings) {
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
    if (controller.records.isEmpty) {
      return Center(child: Text(strings.noLogs));
    }
    final itemCount = controller.records.length + (controller.hasMore ? 1 : 0);
    return Stack(
      children: [
        ListView.builder(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
          itemCount: itemCount,
          itemBuilder: (context, index) {
            if (index == controller.records.length) {
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
            return _LogCard(
              record: controller.records[index],
              strings: strings,
            );
          },
        ),
        if (controller.refreshing) const LinearProgressIndicator(),
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
    required this.onApplyComponent,
    required this.strings,
  });

  final LogsController controller;
  final TextEditingController componentController;
  final VoidCallback onApplyComponent;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
    child: Wrap(
      spacing: 12,
      runSpacing: 8,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
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
    ),
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
  Widget build(BuildContext context) => MaterialBanner(
    content: Text(strings.logsUnavailable),
    actions: [
      if (onOpenDiagnostics != null)
        TextButton.icon(
          onPressed: onOpenDiagnostics,
          icon: const Icon(Icons.health_and_safety_outlined),
          label: Text(strings.openDiagnostics),
        ),
      TextButton(onPressed: onRetry, child: Text(strings.retry)),
    ],
  );
}

class _LogCard extends StatelessWidget {
  const _LogCard({required this.record, required this.strings});

  final ManagedLogRecord record;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) => Card(
    child: Column(
      children: [
        ListTile(
          leading: CircleAvatar(
            child: Text(record.level.isEmpty ? '?' : record.level[0]),
          ),
          title: Text(record.message),
          subtitle: Text(
            '${record.timestamp.toLocal().toIso8601String()} · ${record.component}',
          ),
          trailing: Text(record.level),
        ),
        if (record.fields.isNotEmpty)
          ExpansionTile(
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
      ],
    ),
  );
}
