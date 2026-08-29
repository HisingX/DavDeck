import 'dart:convert';
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
  late final ScrollController logScrollController;

  final Set<int> _knownRecordIds = <int>{};
  int? _selectedRecordId;
  int _pendingNewLogs = 0;
  String _sourceFilter = '';
  int _detailTab = 0;
  bool _userPausedFollow = false;

  LogsController get controller => widget.controller;

  @override
  void initState() {
    super.initState();
    componentController = TextEditingController(
      text: controller.componentFilter,
    );
    queryController = TextEditingController();
    logScrollController = ScrollController()..addListener(_handleScroll);
    controller.addListener(_handleControllerChange);
  }

  @override
  void dispose() {
    controller.removeListener(_handleControllerChange);
    logScrollController
      ..removeListener(_handleScroll)
      ..dispose();
    componentController.dispose();
    queryController.dispose();
    super.dispose();
  }

  void _handleScroll() {
    if (!controller.autoRefreshEnabled || !logScrollController.hasClients) {
      return;
    }
    _userPausedFollow = logScrollController.position.pixels > 24;
  }

  void _handleControllerChange() {
    if (!mounted) return;
    final ids = controller.records.map((record) => record.id).toSet();
    if (_knownRecordIds.isNotEmpty) {
      final newCount = ids.difference(_knownRecordIds).length;
      if (newCount > 0) {
        if (controller.autoRefreshEnabled && !_userPausedFollow) {
          _pendingNewLogs = 0;
          WidgetsBinding.instance.addPostFrameCallback((_) {
            if (!mounted || !logScrollController.hasClients) return;
            logScrollController.jumpTo(
              logScrollController.position.minScrollExtent,
            );
          });
        } else {
          _pendingNewLogs += newCount;
        }
      }
    }
    _knownRecordIds
      ..clear()
      ..addAll(ids);
    if (_selectedRecordId == null || !ids.contains(_selectedRecordId)) {
      _selectedRecordId = ids.isEmpty ? null : ids.first;
    }
    setState(() {});
  }

  List<ManagedLogRecord> _visibleRecords() {
    final query = queryController.text.trim().toLowerCase();
    return controller.records
        .where((record) {
          if (_sourceFilter == 'caddy' &&
              record.component.toLowerCase() != 'caddy') {
            return false;
          }
          if (_sourceFilter == 'davdeck' &&
              record.component.toLowerCase() == 'caddy') {
            return false;
          }
          if (query.isEmpty) return true;
          final searchable = [
            record.message,
            record.component,
            const JsonEncoder().convert(record.fields),
          ].join(' ').toLowerCase();
          return searchable.contains(query);
        })
        .toList(growable: false);
  }

  ManagedLogRecord? _selectedRecord(List<ManagedLogRecord> records) {
    if (records.isEmpty) return null;
    for (final record in records) {
      if (record.id == _selectedRecordId) return record;
    }
    return records.first;
  }

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return Scaffold(
      body: AnimatedBuilder(
        animation: controller,
        builder: (context, _) => LayoutBuilder(
          builder: (context, constraints) {
            final pageInset = appPageInset(context);
            final topInset = MediaQuery.sizeOf(context).height < 700
                ? 20.0
                : pageInset;
            final bottomInset = 24.0;
            final maxWidth = (constraints.maxWidth - pageInset * 2).clamp(
              0.0,
              1400.0,
            );
            final availableHeight =
                (constraints.maxHeight - topInset - bottomInset).clamp(
                  0.0,
                  double.infinity,
                );
            return Padding(
              padding: EdgeInsets.fromLTRB(
                pageInset,
                topInset,
                pageInset,
                bottomInset,
              ),
              child: Center(
                child: SizedBox(
                  width: maxWidth,
                  height: availableHeight,
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      AppPageHeader(
                        title: strings.logs,
                        subtitle: strings.logsSubtitle,
                      ),
                      const SizedBox(height: 18),
                      _FilterBar(
                        controller: controller,
                        componentController: componentController,
                        queryController: queryController,
                        sourceFilter: _sourceFilter,
                        onApplyComponent: _applyComponentFilter,
                        onQueryChanged: (_) => setState(() {}),
                        onSourceChanged: (value) => setState(() {
                          _sourceFilter = value;
                        }),
                        onFollowChanged: _setFollow,
                        onClear: _clearView,
                        onCopy: () => _copy(context),
                        onExport: () => _export(context),
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
                      const SizedBox(height: 14),
                      Expanded(child: _buildContent(context, strings)),
                    ],
                  ),
                ),
              ),
            );
          },
        ),
      ),
    );
  }

  Widget _buildContent(BuildContext context, AppStrings strings) {
    final records = _visibleRecords();
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
        child: Text(
          queryController.text.trim().isEmpty && _sourceFilter.isEmpty
              ? strings.noLogs
              : strings.noMatchingLogs,
        ),
      );
    }

    final selected = _selectedRecord(records);
    return LayoutBuilder(
      builder: (context, constraints) {
        final detailHeight = constraints.maxHeight < 560 ? 190.0 : 230.0;
        return Column(
          children: [
            Expanded(
              child: _LogConsole(
                records: records,
                selectedRecordId: selected?.id,
                pendingNewLogs: _pendingNewLogs,
                scrollController: logScrollController,
                onSelect: (record) {
                  setState(() {
                    _selectedRecordId = record.id;
                    _detailTab = 0;
                  });
                },
                onJumpToLatest: _jumpToLatest,
                onLoadMore: controller.loadMore,
                loadingMore: controller.loadingMore,
                hasMore:
                    queryController.text.trim().isEmpty &&
                    _sourceFilter.isEmpty &&
                    controller.hasMore,
                strings: strings,
              ),
            ),
            if (selected != null) ...[
              const SizedBox(height: 12),
              SizedBox(
                height: detailHeight,
                child: _LogDetails(
                  record: selected,
                  activeTab: _detailTab,
                  json: controller.recordJson(selected),
                  onTabChanged: (tab) => setState(() => _detailTab = tab),
                  onCopyJson: () =>
                      _copyContent(context, controller.recordJson(selected)),
                  strings: strings,
                ),
              ),
            ],
          ],
        );
      },
    );
  }

  void _setFollow(bool enabled) {
    controller.setAutoRefresh(enabled);
    if (enabled) {
      _userPausedFollow = false;
      _pendingNewLogs = 0;
      _jumpToLatest();
    }
  }

  void _jumpToLatest() {
    _userPausedFollow = false;
    _pendingNewLogs = 0;
    if (!logScrollController.hasClients) {
      setState(() {});
      return;
    }
    logScrollController.animateTo(
      logScrollController.position.minScrollExtent,
      duration: const Duration(milliseconds: 180),
      curve: Curves.easeOut,
    );
    setState(() {});
  }

  void _clearView() {
    controller.clearView();
    _knownRecordIds.clear();
    _selectedRecordId = null;
    _pendingNewLogs = 0;
  }

  Future<void> _applyComponentFilter() async {
    await controller.setComponentFilter(componentController.text);
  }

  Future<void> _copy(BuildContext context) =>
      _copyContent(context, controller.exportText(), showSuccess: true);

  Future<void> _copyContent(
    BuildContext context,
    String content, {
    bool showSuccess = false,
  }) async {
    final strings = AppStrings.of(context);
    final action =
        widget.copyAction ??
        (value) => Clipboard.setData(ClipboardData(text: value));
    try {
      await action(content);
      if (!context.mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(showSuccess ? strings.logsCopied : strings.jsonCopied),
        ),
      );
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
    required this.sourceFilter,
    required this.onApplyComponent,
    required this.onQueryChanged,
    required this.onSourceChanged,
    required this.onFollowChanged,
    required this.onClear,
    required this.onCopy,
    required this.onExport,
    required this.strings,
  });

  final LogsController controller;
  final TextEditingController componentController;
  final TextEditingController queryController;
  final String sourceFilter;
  final VoidCallback onApplyComponent;
  final ValueChanged<String> onQueryChanged;
  final ValueChanged<String> onSourceChanged;
  final ValueChanged<bool> onFollowChanged;
  final VoidCallback onClear;
  final VoidCallback onCopy;
  final VoidCallback onExport;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) {
    final components = <String>{
      ...controller.records.map((record) => record.component),
      if (controller.componentFilter.isNotEmpty) controller.componentFilter,
    }.toList()..sort();
    return Wrap(
      spacing: 10,
      runSpacing: 10,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        AppSearchField(
          width: 300,
          controller: queryController,
          hintText: strings.searchLogsHint,
          clearTooltip: strings.clearSearch,
          onChanged: onQueryChanged,
        ),
        _LogDropdown(
          width: 132,
          value: controller.levelFilter,
          label: strings.allLevels,
          items: [
            ('', strings.allLevels),
            for (final level in ['DEBUG', 'INFO', 'WARN', 'ERROR'])
              (level, level),
          ],
          onChanged: controller.setLevelFilter,
        ),
        SizedBox(
          width: 164,
          child: TextField(
            controller: componentController,
            onSubmitted: (_) => onApplyComponent(),
            decoration: InputDecoration(
              hintText: strings.allComponents,
              suffixIcon: PopupMenuButton<String>(
                tooltip: strings.allComponents,
                icon: const Icon(Icons.keyboard_arrow_down),
                onSelected: (value) {
                  componentController.text = value;
                  onApplyComponent();
                },
                itemBuilder: (context) => [
                  PopupMenuItem(value: '', child: Text(strings.allComponents)),
                  for (final component in components)
                    PopupMenuItem(value: component, child: Text(component)),
                ],
              ),
            ),
          ),
        ),
        TextButton(
          onPressed: onApplyComponent,
          child: Text(strings.applyFilter),
        ),
        _SourceFilter(
          value: sourceFilter,
          onChanged: onSourceChanged,
          strings: strings,
        ),
        Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(strings.followLogs),
            Switch(
              value: controller.autoRefreshEnabled,
              onChanged: onFollowChanged,
            ),
          ],
        ),
        IconButton(
          onPressed: controller.refreshing ? null : controller.refresh,
          tooltip: strings.refreshLogs,
          icon: const Icon(Icons.refresh),
        ),
        IconButton(
          onPressed: onCopy,
          tooltip: strings.copyLogs,
          icon: const Icon(Icons.copy_all_outlined),
        ),
        Tooltip(
          message: strings.clearLogs,
          child: OutlinedButton.icon(
            onPressed: onClear,
            icon: const Icon(Icons.delete_outline),
            label: Text(strings.clearLogs),
          ),
        ),
        Tooltip(
          message: strings.exportLogs,
          child: OutlinedButton.icon(
            onPressed: onExport,
            icon: const Icon(Icons.file_download_outlined),
            label: Text(strings.exportLogs),
          ),
        ),
      ],
    );
  }
}

class _LogDropdown extends StatelessWidget {
  const _LogDropdown({
    required this.width,
    required this.value,
    required this.label,
    required this.items,
    required this.onChanged,
  });

  final double width;
  final String value;
  final String label;
  final List<(String, String)> items;
  final ValueChanged<String> onChanged;

  @override
  Widget build(BuildContext context) => Container(
    width: width,
    height: 48,
    padding: const EdgeInsets.symmetric(horizontal: 12),
    decoration: BoxDecoration(
      color: Theme.of(context).colorScheme.surface,
      border: Border.all(color: Theme.of(context).colorScheme.outline),
      borderRadius: BorderRadius.circular(12),
    ),
    child: DropdownButtonHideUnderline(
      child: DropdownButton<String>(
        value: items.any((item) => item.$1 == value) ? value : items.first.$1,
        isExpanded: true,
        icon: const Icon(Icons.keyboard_arrow_down),
        hint: Text(label),
        items: [
          for (final item in items)
            DropdownMenuItem(value: item.$1, child: Text(item.$2)),
        ],
        onChanged: (next) {
          if (next != null) onChanged(next);
        },
      ),
    ),
  );
}

class _SourceFilter extends StatelessWidget {
  const _SourceFilter({
    required this.value,
    required this.onChanged,
    required this.strings,
  });

  final String value;
  final ValueChanged<String> onChanged;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final options = [
      ('', strings.allSources),
      ('davdeck', strings.davDeck),
      ('caddy', strings.caddy),
    ];
    return Container(
      height: 48,
      padding: const EdgeInsets.all(3),
      decoration: BoxDecoration(
        color: scheme.surface,
        border: Border.all(color: scheme.outline),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          for (final option in options)
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 1),
              child: _SourceOption(
                label: option.$2,
                selected: value == option.$1,
                onTap: () => onChanged(option.$1),
              ),
            ),
        ],
      ),
    );
  }
}

class _SourceOption extends StatelessWidget {
  const _SourceOption({
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Material(
      color: selected ? scheme.primaryContainer : Colors.transparent,
      borderRadius: BorderRadius.circular(9),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(9),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
          child: Text(
            label,
            style: TextStyle(
              color: selected
                  ? scheme.onPrimaryContainer
                  : scheme.onSurfaceVariant,
              fontWeight: selected ? FontWeight.w600 : FontWeight.w500,
            ),
          ),
        ),
      ),
    );
  }
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

class _LogConsole extends StatelessWidget {
  const _LogConsole({
    required this.records,
    required this.selectedRecordId,
    required this.pendingNewLogs,
    required this.scrollController,
    required this.onSelect,
    required this.onJumpToLatest,
    required this.onLoadMore,
    required this.loadingMore,
    required this.hasMore,
    required this.strings,
  });

  final List<ManagedLogRecord> records;
  final int? selectedRecordId;
  final int pendingNewLogs;
  final ScrollController scrollController;
  final ValueChanged<ManagedLogRecord> onSelect;
  final VoidCallback onJumpToLatest;
  final Future<void> Function() onLoadMore;
  final bool loadingMore;
  final bool hasMore;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) => AppSurface(
    padding: EdgeInsets.zero,
    shadow: false,
    child: Column(
      children: [
        _LogTableHeader(strings: strings),
        Expanded(
          child: Stack(
            children: [
              ListView.builder(
                controller: scrollController,
                padding: const EdgeInsets.only(bottom: 52),
                itemCount: records.length + (hasMore ? 1 : 0),
                itemBuilder: (context, index) {
                  if (index == records.length) {
                    return Padding(
                      padding: const EdgeInsets.symmetric(vertical: 12),
                      child: Center(
                        child: TextButton(
                          onPressed: loadingMore ? null : onLoadMore,
                          child: Text(
                            loadingMore
                                ? strings.logsLoading
                                : strings.loadMoreLogs,
                          ),
                        ),
                      ),
                    );
                  }
                  final record = records[index];
                  return _LogRow(
                    record: record,
                    selected: record.id == selectedRecordId,
                    onTap: () => onSelect(record),
                  );
                },
              ),
              if (pendingNewLogs > 0)
                Positioned(
                  left: 0,
                  right: 0,
                  bottom: 10,
                  child: Center(
                    child: Material(
                      color: Theme.of(context).colorScheme.primaryContainer,
                      borderRadius: BorderRadius.circular(99),
                      child: InkWell(
                        onTap: onJumpToLatest,
                        borderRadius: BorderRadius.circular(99),
                        child: Padding(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 16,
                            vertical: 8,
                          ),
                          child: Text(
                            strings.newLogs(pendingNewLogs),
                            style: TextStyle(
                              color: Theme.of(
                                context,
                              ).colorScheme.onPrimaryContainer,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
            ],
          ),
        ),
      ],
    ),
  );
}

class _LogTableHeader extends StatelessWidget {
  const _LogTableHeader({required this.strings});

  final AppStrings strings;

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 12),
    decoration: BoxDecoration(
      color: Theme.of(context).colorScheme.surfaceContainerLow,
      border: Border(
        bottom: BorderSide(color: Theme.of(context).colorScheme.outlineVariant),
      ),
    ),
    child: Row(
      children: [
        _LogHeaderCell(strings.logTime, flex: 2),
        _LogHeaderCell(strings.logLevel, flex: 1),
        _LogHeaderCell(strings.logComponent, flex: 1),
        _LogHeaderCell(strings.logMessage, flex: 4),
        _LogHeaderCell(strings.status, flex: 1),
        _LogHeaderCell(strings.logDuration, flex: 1),
      ],
    ),
  );
}

class _LogHeaderCell extends StatelessWidget {
  const _LogHeaderCell(this.label, {required this.flex});

  final String label;
  final int flex;

  @override
  Widget build(BuildContext context) => Expanded(
    flex: flex,
    child: Text(
      label,
      overflow: TextOverflow.ellipsis,
      style: TextStyle(
        color: Theme.of(context).colorScheme.onSurfaceVariant,
        fontSize: 12,
        fontWeight: FontWeight.w600,
      ),
    ),
  );
}

class _LogRow extends StatelessWidget {
  const _LogRow({
    required this.record,
    required this.selected,
    required this.onTap,
  });

  final ManagedLogRecord record;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final levelColor = appStatusColor(context, record.level);
    return Material(
      color: selected
          ? theme.colorScheme.primaryContainer.withValues(alpha: 0.62)
          : theme.colorScheme.surface,
      child: InkWell(
        onTap: onTap,
        child: Container(
          constraints: const BoxConstraints(minHeight: 36),
          padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 7),
          decoration: BoxDecoration(
            border: Border(
              left: BorderSide(
                color: selected
                    ? theme.colorScheme.primary
                    : Colors.transparent,
                width: 3,
              ),
              bottom: BorderSide(color: theme.colorScheme.outlineVariant),
            ),
          ),
          child: Row(
            children: [
              _LogCell(
                flex: 2,
                child: Text(
                  _formatLogTime(record.timestamp),
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontFamily: 'monospace', fontSize: 13),
                ),
              ),
              _LogCell(
                flex: 1,
                child: Row(
                  children: [
                    Container(
                      width: 7,
                      height: 7,
                      decoration: BoxDecoration(
                        color: levelColor,
                        shape: BoxShape.circle,
                      ),
                    ),
                    const SizedBox(width: 7),
                    Expanded(
                      child: Text(
                        record.level,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(color: levelColor, fontSize: 13),
                      ),
                    ),
                  ],
                ),
              ),
              _LogCell(
                flex: 1,
                child: Text(
                  _sourceComponent(record.component),
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontFamily: 'monospace', fontSize: 13),
                ),
              ),
              _LogCell(
                flex: 4,
                child: Text(
                  record.message,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontFamily: 'monospace', fontSize: 13),
                ),
              ),
              _LogCell(
                flex: 1,
                child: Text(
                  _recordStatus(record),
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontFamily: 'monospace', fontSize: 13),
                ),
              ),
              _LogCell(
                flex: 1,
                child: Text(
                  _recordDuration(record),
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontFamily: 'monospace', fontSize: 13),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _LogCell extends StatelessWidget {
  const _LogCell({required this.flex, required this.child});

  final int flex;
  final Widget child;

  @override
  Widget build(BuildContext context) => Expanded(
    flex: flex,
    child: Padding(padding: const EdgeInsets.only(right: 12), child: child),
  );
}

class _LogDetails extends StatelessWidget {
  const _LogDetails({
    required this.record,
    required this.activeTab,
    required this.json,
    required this.onTabChanged,
    required this.onCopyJson,
    required this.strings,
  });

  final ManagedLogRecord record;
  final int activeTab;
  final String json;
  final ValueChanged<int> onTabChanged;
  final VoidCallback onCopyJson;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) => AppSurface(
    padding: EdgeInsets.zero,
    shadow: false,
    child: Column(
      children: [
        SizedBox(
          height: 42,
          child: Row(
            children: [
              _DetailTab(
                label: strings.logSummary,
                selected: activeTab == 0,
                onTap: () => onTabChanged(0),
              ),
              _DetailTab(
                label: strings.logDetails,
                selected: activeTab == 1,
                onTap: () => onTabChanged(1),
              ),
              const Spacer(),
              IconButton(
                onPressed: onCopyJson,
                tooltip: strings.copyJson,
                icon: const Icon(Icons.copy_outlined, size: 17),
              ),
              const SizedBox(width: 6),
            ],
          ),
        ),
        Divider(height: 1, color: Theme.of(context).colorScheme.outlineVariant),
        Expanded(
          child: LayoutBuilder(
            builder: (context, constraints) {
              final jsonPane = _JsonPane(json: json, strings: strings);
              final fieldsPane = activeTab == 0
                  ? _SummaryPane(record: record, strings: strings)
                  : _FieldsPane(record: record, strings: strings);
              if (constraints.maxWidth < 820) {
                return Column(
                  children: [
                    Expanded(child: fieldsPane),
                    Divider(
                      height: 1,
                      color: Theme.of(context).colorScheme.outlineVariant,
                    ),
                    Expanded(child: jsonPane),
                  ],
                );
              }
              return Row(
                children: [
                  Expanded(child: fieldsPane),
                  VerticalDivider(
                    width: 1,
                    color: Theme.of(context).colorScheme.outlineVariant,
                  ),
                  Expanded(child: jsonPane),
                ],
              );
            },
          ),
        ),
      ],
    ),
  );
}

class _DetailTab extends StatelessWidget {
  const _DetailTab({
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onTap,
        child: Container(
          height: 42,
          padding: const EdgeInsets.symmetric(horizontal: 18),
          decoration: BoxDecoration(
            border: Border(
              bottom: BorderSide(
                color: selected ? scheme.primary : Colors.transparent,
                width: 2,
              ),
            ),
          ),
          alignment: Alignment.center,
          child: Text(
            label,
            style: TextStyle(
              color: selected ? scheme.primary : scheme.onSurfaceVariant,
              fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
            ),
          ),
        ),
      ),
    );
  }
}

class _SummaryPane extends StatelessWidget {
  const _SummaryPane({required this.record, required this.strings});

  final ManagedLogRecord record;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) {
    final entries = <(String, String)>[
      (strings.logTime, record.timestamp.toLocal().toIso8601String()),
      (strings.logComponent, _sourceComponent(record.component)),
      (strings.logLevel, record.level),
      (strings.logMessage, record.message),
      ...record.fields.entries.map((entry) => (entry.key, '${entry.value}')),
    ];
    return SingleChildScrollView(
      padding: const EdgeInsets.fromLTRB(18, 10, 18, 14),
      child: Wrap(
        spacing: 24,
        runSpacing: 7,
        children: [
          for (final entry in entries)
            SizedBox(
              width: 285,
              child: _DetailPair(label: entry.$1, value: entry.$2),
            ),
        ],
      ),
    );
  }
}

class _FieldsPane extends StatelessWidget {
  const _FieldsPane({required this.record, required this.strings});

  final ManagedLogRecord record;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) {
    if (record.fields.isEmpty) {
      return Center(child: Text(strings.noStructuredFields));
    }
    return ListView(
      padding: const EdgeInsets.fromLTRB(18, 8, 18, 12),
      children: [
        for (final entry in record.fields.entries)
          _DetailPair(label: entry.key, value: '${entry.value}'),
      ],
    );
  }
}

class _DetailPair extends StatelessWidget {
  const _DetailPair({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.symmetric(vertical: 3),
    child: Text.rich(
      TextSpan(
        children: [
          TextSpan(
            text: '$label  ',
            style: TextStyle(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
              fontSize: 12,
            ),
          ),
          TextSpan(text: value),
        ],
      ),
      maxLines: 2,
      overflow: TextOverflow.ellipsis,
      style: const TextStyle(fontSize: 13),
    ),
  );
}

class _JsonPane extends StatelessWidget {
  const _JsonPane({required this.json, required this.strings});

  final String json;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.fromLTRB(18, 10, 18, 10),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          strings.originalLog,
          style: TextStyle(
            color: Theme.of(context).colorScheme.onSurfaceVariant,
            fontSize: 12,
            fontWeight: FontWeight.w600,
          ),
        ),
        const SizedBox(height: 6),
        Expanded(
          child: SingleChildScrollView(
            child: SelectableText(
              json,
              style: const TextStyle(
                fontFamily: 'monospace',
                fontSize: 12,
                height: 1.35,
              ),
            ),
          ),
        ),
      ],
    ),
  );
}

String _formatLogTime(DateTime value) {
  final local = value.toLocal();
  String two(int number) => number.toString().padLeft(2, '0');
  String three(int number) => number.toString().padLeft(3, '0');
  return '${two(local.hour)}:${two(local.minute)}:${two(local.second)}.${three(local.millisecond)}';
}

String _sourceComponent(String component) =>
    component.toLowerCase() == 'daemon' ? 'davd' : component;

String _recordStatus(ManagedLogRecord record) {
  final value =
      record.fields['status'] ??
      record.fields['status_code'] ??
      record.fields['statusCode'];
  return value == null ? '-' : '$value';
}

String _recordDuration(ManagedLogRecord record) {
  final value =
      record.fields['duration_ms'] ??
      record.fields['durationMs'] ??
      record.fields['duration'];
  if (value == null) return '-';
  return value is num
      ? '${value.toStringAsFixed(value % 1 == 0 ? 0 : 1)}ms'
      : '$value';
}
