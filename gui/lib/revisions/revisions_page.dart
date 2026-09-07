import 'dart:math' as math;

import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/state/revision_controller.dart';
import 'package:davdeck/widgets/app_ui.dart';
import 'package:flutter/material.dart';

const _defaultRevisionPageSize = 10;
const _revisionPageSizes = [10, 20, 50];
const _revisionContentMaxWidth = 1320.0;

/// Formats the daemon's UTC timestamp in the user's local timezone.
String formatRevisionCreatedAt(String value) {
  final parsed = DateTime.tryParse(value);
  if (parsed == null) return value;
  final local = parsed.toLocal();
  String twoDigits(int number) => number.toString().padLeft(2, '0');
  return '${local.year}-${twoDigits(local.month)}-${twoDigits(local.day)} '
      '${twoDigits(local.hour)}:${twoDigits(local.minute)}:${twoDigits(local.second)}';
}

class RevisionsPage extends StatefulWidget {
  const RevisionsPage({super.key, required this.controller});

  final RevisionController controller;

  @override
  State<RevisionsPage> createState() => _RevisionsPageState();
}

enum _RevisionSort { createdDescending, createdAscending }

class _RevisionsPageState extends State<RevisionsPage> {
  final _searchController = TextEditingController();
  final _selectedIds = <String>{};
  _RevisionSort _sort = _RevisionSort.createdDescending;
  int _page = 1;
  int _pageSize = _defaultRevisionPageSize;

  RevisionController get controller => widget.controller;

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  List<ManagedRevision> _filteredRevisions() {
    final query = _searchController.text.trim().toLowerCase();
    final activeRevision = controller.configuration?.activeRevision;
    final values = controller.revisions.where((revision) {
      if (query.isEmpty) return true;
      return revision.number.toString().contains(query);
    }).toList();
    values.sort((left, right) {
      final leftIsActive = left.number == activeRevision;
      final rightIsActive = right.number == activeRevision;
      if (leftIsActive != rightIsActive) return leftIsActive ? -1 : 1;
      final leftDate = DateTime.tryParse(left.createdAt);
      final rightDate = DateTime.tryParse(right.createdAt);
      final comparison = leftDate != null && rightDate != null
          ? leftDate.compareTo(rightDate)
          : left.number.compareTo(right.number);
      return _sort == _RevisionSort.createdDescending
          ? -comparison
          : comparison;
    });
    return values;
  }

  List<ManagedRevision> _visibleRevisions(List<ManagedRevision> filtered) {
    final pageCount = math.max(1, (filtered.length / _pageSize).ceil());
    final page = _page.clamp(1, pageCount);
    if (page != _page) _page = page;
    final start = (page - 1) * _pageSize;
    final end = math.min(start + _pageSize, filtered.length);
    return start >= filtered.length ? const [] : filtered.sublist(start, end);
  }

  bool _isProtected(ManagedRevision revision) {
    final state = controller.configuration;
    return revision.number == state?.activeRevision ||
        revision.number == state?.desiredRevision;
  }

  bool _isActive(ManagedRevision revision) =>
      revision.number == controller.configuration?.activeRevision;

  Future<void> _confirmRestore(
    BuildContext context,
    ManagedRevision revision,
    AppStrings strings,
  ) async {
    final confirmed = await showAppDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(strings.restoreRevision),
        content: Text(strings.confirmRestoreRevision(revision.number)),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: Text(strings.cancel),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: Text(strings.restore),
          ),
        ],
      ),
    );
    if (confirmed == true) await controller.restore(revision);
  }

  Future<void> _confirmDelete(
    BuildContext context,
    ManagedRevision revision,
    AppStrings strings,
  ) async {
    final confirmed = await showAppDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(strings.deleteRevision),
        content: Text(strings.confirmDeleteRevision(revision.number)),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: Text(strings.cancel),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: Text(strings.delete),
          ),
        ],
      ),
    );
    if (confirmed == true) {
      final deleted = await controller.delete(revision);
      if (deleted && mounted) setState(() => _selectedIds.remove(revision.id));
    }
  }

  Future<void> _confirmBatchDelete(
    BuildContext context,
    AppStrings strings,
    Iterable<ManagedRevision> selected,
  ) async {
    final deletable = selected
        .where((revision) => !_isProtected(revision))
        .toList();
    if (deletable.isEmpty) return;
    final confirmed = await showAppDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(strings.batchDelete),
        content: Text(strings.confirmBatchDeleteRevision(deletable.length)),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: Text(strings.cancel),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: Text(strings.batchDelete),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    final deleted = await controller.deleteMany(deletable);
    if (deleted && mounted) {
      setState(() {
        _selectedIds.removeAll(deletable.map((revision) => revision.id));
      });
    }
  }

  void _toggleSelection(ManagedRevision revision, bool selected) {
    setState(() {
      if (selected) {
        _selectedIds.add(revision.id);
      } else {
        _selectedIds.remove(revision.id);
      }
    });
  }

  void _toggleAll(Iterable<ManagedRevision> revisions) {
    final ids = revisions.map((revision) => revision.id).toSet();
    setState(() {
      if (ids.every(_selectedIds.contains)) {
        _selectedIds.removeAll(ids);
      } else {
        _selectedIds.addAll(ids);
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return Scaffold(
      body: AnimatedBuilder(
        animation: controller,
        builder: (context, _) {
          if (controller.state == RevisionLoadState.loading &&
              controller.revisions.isEmpty) {
            return Center(child: Text(strings.revisionsLoading));
          }
          if (controller.state == RevisionLoadState.error &&
              controller.revisions.isEmpty) {
            return _ErrorState(
              message: controller.error.toString(),
              retry: controller.refresh,
              strings: strings,
            );
          }
          final filtered = _filteredRevisions();
          final visible = _visibleRevisions(filtered);
          final selected = controller.revisions
              .where((revision) => _selectedIds.contains(revision.id))
              .toList();
          final allVisibleSelected =
              visible.isNotEmpty &&
              visible.every((revision) => _selectedIds.contains(revision.id));
          final anyVisibleSelected = visible.any(
            (revision) => _selectedIds.contains(revision.id),
          );
          return Stack(
            children: [
              SingleChildScrollView(
                padding: const EdgeInsets.fromLTRB(28, 24, 28, 28),
                child: Center(
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(
                      maxWidth: _revisionContentMaxWidth,
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        if (controller.configuration != null)
                          _ConfigurationCard(
                            state: controller.configuration!,
                            strings: strings,
                          ),
                        const SizedBox(height: 20),
                        Row(
                          crossAxisAlignment: CrossAxisAlignment.end,
                          children: [
                            Text(
                              strings.revisionHistory,
                              style: Theme.of(context).textTheme.headlineSmall
                                  ?.copyWith(fontWeight: FontWeight.w700),
                            ),
                            const Spacer(),
                            Text(
                              strings.revisionsCount(filtered.length),
                              style: Theme.of(context).textTheme.bodyMedium
                                  ?.copyWith(
                                    color: Theme.of(
                                      context,
                                    ).colorScheme.onSurfaceVariant,
                                  ),
                            ),
                          ],
                        ),
                        const SizedBox(height: 10),
                        _RevisionTable(
                          strings: strings,
                          revisions: visible,
                          selectedIds: _selectedIds,
                          selectedCount: selected.length,
                          allVisibleSelected: allVisibleSelected,
                          anyVisibleSelected: anyVisibleSelected,
                          sort: _sort,
                          page: _page,
                          pageSize: _pageSize,
                          pageCount: math.max(
                            1,
                            (filtered.length / _pageSize).ceil(),
                          ),
                          restoringId: controller.restoringId,
                          deletingId: controller.deletingId,
                          isActive: _isActive,
                          isProtected: _isProtected,
                          onToggleAll: () => _toggleAll(visible),
                          onToggle: _toggleSelection,
                          onSortChanged: (value) => setState(() {
                            _sort = value;
                            _page = 1;
                          }),
                          onSearchChanged: (_) => setState(() => _page = 1),
                          onPageChanged: (value) =>
                              setState(() => _page = value),
                          onPageSizeChanged: (value) => setState(() {
                            _pageSize = value;
                            _page = 1;
                          }),
                          onRestore: (revision) =>
                              _confirmRestore(context, revision, strings),
                          onDelete: (revision) =>
                              _confirmDelete(context, revision, strings),
                          onBatchDelete: () =>
                              _confirmBatchDelete(context, strings, selected),
                          searchController: _searchController,
                        ),
                        if (controller.error != null &&
                            controller.state == RevisionLoadState.ready) ...[
                          const SizedBox(height: 12),
                          AppNotice(
                            icon: Icons.error_outline,
                            text: controller.error.toString(),
                            color: Theme.of(context).colorScheme.errorContainer,
                            textColor: Theme.of(
                              context,
                            ).colorScheme.onErrorContainer,
                          ),
                        ],
                      ],
                    ),
                  ),
                ),
              ),
              if (controller.restoringId != null ||
                  controller.deletingId != null)
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
}

class _ConfigurationCard extends StatelessWidget {
  const _ConfigurationCard({required this.state, required this.strings});

  final ManagedRevisionState state;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final color = state.pending
        ? const Color(0xffb87800)
        : theme.colorScheme.primary;
    return AppSurface(
      padding: const EdgeInsets.symmetric(horizontal: 22, vertical: 14),
      borderRadius: 14,
      child: Row(
        children: [
          Container(
            width: 56,
            height: 56,
            decoration: BoxDecoration(color: color, shape: BoxShape.circle),
            child: Icon(
              state.pending ? Icons.pending_actions : Icons.check,
              color: Colors.white,
              size: 30,
            ),
          ),
          const SizedBox(width: 16),
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                strings.configurationState,
                style: theme.textTheme.titleLarge?.copyWith(
                  fontWeight: FontWeight.w700,
                ),
              ),
              const SizedBox(height: 4),
              Text(
                '${strings.desiredRevision}: ${state.desiredRevision ?? strings.none} · '
                '${strings.activeRevision}: ${state.activeRevision ?? strings.none}',
              ),
            ],
          ),
          const Spacer(),
          AppStatusPill(
            label: state.pending ? strings.pending : strings.applied,
            color: color,
          ),
        ],
      ),
    );
  }
}

class _RevisionTable extends StatelessWidget {
  const _RevisionTable({
    required this.strings,
    required this.revisions,
    required this.selectedIds,
    required this.selectedCount,
    required this.allVisibleSelected,
    required this.anyVisibleSelected,
    required this.sort,
    required this.page,
    required this.pageSize,
    required this.pageCount,
    required this.restoringId,
    required this.deletingId,
    required this.isActive,
    required this.isProtected,
    required this.onToggleAll,
    required this.onToggle,
    required this.onSortChanged,
    required this.onSearchChanged,
    required this.onPageChanged,
    required this.onPageSizeChanged,
    required this.onRestore,
    required this.onDelete,
    required this.onBatchDelete,
    required this.searchController,
  });

  final AppStrings strings;
  final List<ManagedRevision> revisions;
  final Set<String> selectedIds;
  final int selectedCount;
  final bool allVisibleSelected;
  final bool anyVisibleSelected;
  final _RevisionSort sort;
  final int page;
  final int pageSize;
  final int pageCount;
  final String? restoringId;
  final String? deletingId;
  final bool Function(ManagedRevision) isActive;
  final bool Function(ManagedRevision) isProtected;
  final VoidCallback onToggleAll;
  final void Function(ManagedRevision revision, bool selected) onToggle;
  final ValueChanged<_RevisionSort> onSortChanged;
  final ValueChanged<String> onSearchChanged;
  final ValueChanged<int> onPageChanged;
  final ValueChanged<int> onPageSizeChanged;
  final ValueChanged<ManagedRevision> onRestore;
  final ValueChanged<ManagedRevision> onDelete;
  final VoidCallback onBatchDelete;
  final TextEditingController searchController;

  @override
  Widget build(BuildContext context) => LayoutBuilder(
    builder: (context, constraints) {
      final compact = constraints.maxWidth < 760;
      final rows = revisions
          .map(
            (revision) => Padding(
              padding: const EdgeInsets.only(bottom: 6),
              child: _RevisionRow(
                revision: revision,
                strings: strings,
                selected: selectedIds.contains(revision.id),
                active: isActive(revision),
                compact: compact,
                restoring: restoringId == revision.id,
                deleting: deletingId == revision.id,
                canRestore:
                    revision.validationStatus == 'VALID' &&
                    revision.stateSnapshotAvailable &&
                    !isProtected(revision),
                canDelete: !isProtected(revision),
                onToggle: (value) => onToggle(revision, value),
                onRestore: () => onRestore(revision),
                onDelete: () => onDelete(revision),
              ),
            ),
          )
          .toList();
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          AppSurface(
            padding: EdgeInsets.zero,
            child: _Toolbar(
              strings: strings,
              selectedCount: selectedCount,
              allVisibleSelected: allVisibleSelected,
              partiallySelected: anyVisibleSelected && !allVisibleSelected,
              canSelectAll: revisions.isNotEmpty,
              sort: sort,
              onToggleAll: onToggleAll,
              onSortChanged: onSortChanged,
              onBatchDelete: onBatchDelete,
              searchController: searchController,
              onSearchChanged: onSearchChanged,
            ),
          ),
          const SizedBox(height: 10),
          if (revisions.isEmpty)
            AppSurface(
              padding: const EdgeInsets.symmetric(vertical: 28),
              child: Center(child: Text(strings.noMatchingRevisions)),
            )
          else
            ...rows,
          AppSurface(
            padding: EdgeInsets.zero,
            child: _TableFooter(
              strings: strings,
              selectedCount: selectedCount,
              page: page,
              pageSize: pageSize,
              pageCount: pageCount,
              onPageChanged: onPageChanged,
              onPageSizeChanged: onPageSizeChanged,
            ),
          ),
        ],
      );
    },
  );
}

class _Toolbar extends StatelessWidget {
  const _Toolbar({
    required this.strings,
    required this.selectedCount,
    required this.allVisibleSelected,
    required this.partiallySelected,
    required this.canSelectAll,
    required this.sort,
    required this.onToggleAll,
    required this.onSortChanged,
    required this.onBatchDelete,
    required this.searchController,
    required this.onSearchChanged,
  });

  final AppStrings strings;
  final int selectedCount;
  final bool allVisibleSelected;
  final bool partiallySelected;
  final bool canSelectAll;
  final _RevisionSort sort;
  final VoidCallback onToggleAll;
  final ValueChanged<_RevisionSort> onSortChanged;
  final VoidCallback onBatchDelete;
  final TextEditingController searchController;
  final ValueChanged<String> onSearchChanged;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.fromLTRB(20, 12, 20, 12),
    child: LayoutBuilder(
      builder: (context, constraints) {
        final search = AppSearchField(
          controller: searchController,
          hintText: strings.revisionSearchHint,
          clearTooltip: strings.clearSearch,
          onChanged: onSearchChanged,
        );
        final selectAll = IconButton(
          tooltip: allVisibleSelected
              ? strings.clearSelection
              : strings.selectAll,
          onPressed: canSelectAll ? onToggleAll : null,
          icon: Icon(
            allVisibleSelected
                ? Icons.deselect_outlined
                : partiallySelected
                ? Icons.indeterminate_check_box_outlined
                : Icons.select_all,
          ),
        );
        final actions = <Widget>[
          _SortButton(strings: strings, value: sort, onChanged: onSortChanged),
          if (selectedCount > 0)
            Text(
              '${strings.selectedItems} $selectedCount ${strings.revision}',
              style: Theme.of(context).textTheme.bodyMedium,
            ),
          OutlinedButton.icon(
            onPressed: selectedCount == 0 ? null : onBatchDelete,
            icon: const Icon(Icons.delete_outline),
            label: Text(strings.batchDelete),
            style: OutlinedButton.styleFrom(
              foregroundColor: Theme.of(context).colorScheme.error,
              side: BorderSide(color: Theme.of(context).colorScheme.error),
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 14),
            ),
          ),
        ];
        if (constraints.maxWidth < 1100) {
          return Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                children: [
                  selectAll,
                  const SizedBox(width: 8),
                  Expanded(child: search),
                ],
              ),
              const SizedBox(height: 10),
              Wrap(
                alignment: WrapAlignment.end,
                crossAxisAlignment: WrapCrossAlignment.center,
                spacing: 18,
                runSpacing: 8,
                children: actions,
              ),
            ],
          );
        }
        return Row(
          children: [
            selectAll,
            const SizedBox(width: 4),
            Expanded(child: search),
            const SizedBox(width: 20),
            ...actions.expand((action) sync* {
              yield action;
              if (action != actions.last) yield const SizedBox(width: 18);
            }),
          ],
        );
      },
    ),
  );
}

class _SortButton extends StatelessWidget {
  const _SortButton({
    required this.strings,
    required this.value,
    required this.onChanged,
  });

  final AppStrings strings;
  final _RevisionSort value;
  final ValueChanged<_RevisionSort> onChanged;

  @override
  Widget build(BuildContext context) => SizedBox(
    width: 280,
    child: PopupMenuButton<_RevisionSort>(
      tooltip: strings.sortByCreatedDesc,
      onSelected: onChanged,
      itemBuilder: (context) => [
        PopupMenuItem(
          value: _RevisionSort.createdDescending,
          child: Text(strings.sortByCreatedDesc),
        ),
        PopupMenuItem(
          value: _RevisionSort.createdAscending,
          child: Text(strings.sortByCreatedAsc),
        ),
      ],
      child: Container(
        height: 54,
        padding: const EdgeInsets.symmetric(horizontal: 16),
        decoration: BoxDecoration(
          border: Border.all(color: Theme.of(context).colorScheme.outline),
          borderRadius: BorderRadius.circular(12),
        ),
        child: Row(
          children: [
            const Icon(Icons.swap_vert, size: 22),
            const SizedBox(width: 9),
            Expanded(
              child: Text(
                value == _RevisionSort.createdDescending
                    ? strings.sortByCreatedDesc
                    : strings.sortByCreatedAsc,
                overflow: TextOverflow.ellipsis,
              ),
            ),
            const SizedBox(width: 12),
            const Icon(Icons.keyboard_arrow_down),
          ],
        ),
      ),
    ),
  );
}

class _RevisionRow extends StatelessWidget {
  const _RevisionRow({
    required this.revision,
    required this.strings,
    required this.selected,
    required this.active,
    required this.compact,
    required this.restoring,
    required this.deleting,
    required this.canRestore,
    required this.canDelete,
    required this.onToggle,
    required this.onRestore,
    required this.onDelete,
  });

  final ManagedRevision revision;
  final AppStrings strings;
  final bool selected;
  final bool active;
  final bool compact;
  final bool restoring;
  final bool deleting;
  final bool canRestore;
  final bool canDelete;
  final ValueChanged<bool> onToggle;
  final VoidCallback onRestore;
  final VoidCallback onDelete;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final validationColor = appStatusColor(context, revision.validationStatus);
    final rowColor = selected
        ? theme.colorScheme.primaryContainer.withValues(alpha: 0.42)
        : theme.colorScheme.surface;
    Widget details() => LayoutBuilder(
      builder: (context, constraints) => _RevisionDetails(
        revision: revision,
        strings: strings,
        active: active,
        compact: compact,
        singleLine: !compact && constraints.maxWidth >= 760,
        validationColor: validationColor,
      ),
    );
    final actions = Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        if (revision.stateSnapshotAvailable)
          TextButton.icon(
            onPressed: canRestore && !restoring && !deleting ? onRestore : null,
            icon: Icon(restoring ? Icons.hourglass_top : Icons.history),
            label: Text(restoring ? strings.restoring : strings.restore),
            style: TextButton.styleFrom(
              foregroundColor: theme.colorScheme.primary,
              padding: const EdgeInsets.symmetric(horizontal: 6),
            ),
          ),
        IconButton(
          tooltip: strings.delete,
          onPressed: canDelete && !deleting && !restoring ? onDelete : null,
          color: theme.colorScheme.error,
          icon: const Icon(Icons.delete_outline),
        ),
      ],
    );
    return Container(
      constraints: BoxConstraints(minHeight: compact ? 132 : 80),
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
      decoration: BoxDecoration(
        color: rowColor,
        border: Border.all(color: theme.colorScheme.outlineVariant),
        borderRadius: BorderRadius.circular(12),
      ),
      child: compact
          ? Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Row(
                  children: [
                    SizedBox(
                      width: 44,
                      child: Checkbox(
                        value: selected,
                        onChanged: (value) => onToggle(value ?? false),
                      ),
                    ),
                    _RevisionBadge(
                      number: revision.number,
                      active: active,
                      strings: strings,
                    ),
                    const SizedBox(width: 12),
                    Expanded(child: details()),
                  ],
                ),
                const SizedBox(height: 8),
                Align(alignment: Alignment.centerRight, child: actions),
              ],
            )
          : Row(
              crossAxisAlignment: CrossAxisAlignment.center,
              children: [
                SizedBox(
                  width: 44,
                  child: Checkbox(
                    value: selected,
                    onChanged: (value) => onToggle(value ?? false),
                  ),
                ),
                _RevisionBadge(
                  number: revision.number,
                  active: active,
                  strings: strings,
                ),
                const SizedBox(width: 16),
                Expanded(child: details()),
                const SizedBox(width: 16),
                actions,
              ],
            ),
    );
  }
}

class _RevisionBadge extends StatelessWidget {
  const _RevisionBadge({
    required this.number,
    required this.active,
    required this.strings,
  });

  final int number;
  final bool active;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final color = active
        ? theme.colorScheme.primaryContainer
        : theme.colorScheme.surfaceContainerLow;
    return Tooltip(
      message: '${strings.revision} $number',
      child: Container(
        width: 56,
        height: 56,
        alignment: Alignment.center,
        decoration: BoxDecoration(color: color, shape: BoxShape.circle),
        child: Text(
          '$number',
          style: theme.textTheme.titleMedium?.copyWith(
            color: active
                ? theme.colorScheme.primary
                : theme.colorScheme.onSurfaceVariant,
            fontWeight: FontWeight.w700,
          ),
        ),
      ),
    );
  }
}

class _RevisionDetails extends StatelessWidget {
  const _RevisionDetails({
    required this.revision,
    required this.strings,
    required this.active,
    required this.compact,
    required this.singleLine,
    required this.validationColor,
  });

  final ManagedRevision revision;
  final AppStrings strings;
  final bool active;
  final bool compact;
  final bool singleLine;
  final Color validationColor;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final meta = [
      _RevisionMeta(
        icon: Icons.check_circle_outline,
        color: validationColor,
        label: singleLine
            ? strings.validationStatusLabel(revision.validationStatus)
            : '${strings.validation}: ${strings.validationStatusLabel(revision.validationStatus)}',
      ),
      _RevisionMeta(
        icon: Icons.calendar_today_outlined,
        color: theme.colorScheme.onSurfaceVariant,
        label: singleLine
            ? formatRevisionCreatedAt(revision.createdAt)
            : '${strings.created}: ${formatRevisionCreatedAt(revision.createdAt)}',
      ),
    ];
    final title = Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Flexible(
          child: Text(
            '${strings.revision} ${revision.number}',
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.w700,
            ),
          ),
        ),
        const SizedBox(width: 10),
        AppStatusPill(
          label: active ? strings.currentEffective : strings.recoverable,
          color: active
              ? theme.colorScheme.primary
              : theme.colorScheme.onSurfaceVariant,
        ),
      ],
    );
    if (singleLine) {
      return Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          title,
          const SizedBox(width: 16),
          ...meta.expand((item) sync* {
            yield item;
            if (item != meta.last) yield const SizedBox(width: 12);
          }),
        ],
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        title,
        const SizedBox(height: 8),
        if (compact)
          ...meta.map(
            (item) =>
                Padding(padding: const EdgeInsets.only(bottom: 4), child: item),
          )
        else
          Wrap(spacing: 20, runSpacing: 4, children: meta),
      ],
    );
  }
}

class _RevisionMeta extends StatelessWidget {
  const _RevisionMeta({
    required this.icon,
    required this.color,
    required this.label,
  });

  final IconData icon;
  final Color color;
  final String label;

  @override
  Widget build(BuildContext context) => Row(
    mainAxisSize: MainAxisSize.min,
    children: [
      Icon(icon, color: color, size: 18),
      const SizedBox(width: 6),
      Text(
        label,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: Theme.of(context).textTheme.bodyMedium,
      ),
    ],
  );
}

class _TableFooter extends StatelessWidget {
  const _TableFooter({
    required this.strings,
    required this.selectedCount,
    required this.page,
    required this.pageSize,
    required this.pageCount,
    required this.onPageChanged,
    required this.onPageSizeChanged,
  });

  final AppStrings strings;
  final int selectedCount;
  final int page;
  final int pageSize;
  final int pageCount;
  final ValueChanged<int> onPageChanged;
  final ValueChanged<int> onPageSizeChanged;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return SizedBox(
      height: 60,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 24),
        child: Row(
          children: [
            Text(strings.selectedRevisions(selectedCount)),
            const SizedBox(width: 20),
            Text(strings.rowsPerPage),
            const SizedBox(width: 8),
            DropdownButtonHideUnderline(
              child: DropdownButton<int>(
                key: const ValueKey('revision-page-size'),
                value: pageSize,
                items: _revisionPageSizes
                    .map(
                      (value) => DropdownMenuItem<int>(
                        value: value,
                        child: Text(strings.pageSizeLabel(value)),
                      ),
                    )
                    .toList(),
                onChanged: (value) {
                  if (value != null) onPageSizeChanged(value);
                },
              ),
            ),
            const Spacer(),
            IconButton(
              tooltip: strings.previous,
              onPressed: page <= 1 ? null : () => onPageChanged(page - 1),
              icon: const Icon(Icons.chevron_left),
            ),
            for (var value = 1; value <= pageCount; value++)
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 2),
                child: InkWell(
                  key: ValueKey('revision-page-$value'),
                  borderRadius: BorderRadius.circular(8),
                  onTap: () => onPageChanged(value),
                  child: Container(
                    width: 38,
                    height: 38,
                    alignment: Alignment.center,
                    decoration: BoxDecoration(
                      color: value == page
                          ? theme.colorScheme.primaryContainer
                          : null,
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: Text(
                      '$value',
                      style: TextStyle(
                        color: value == page
                            ? theme.colorScheme.primary
                            : theme.colorScheme.onSurface,
                        fontWeight: value == page ? FontWeight.w700 : null,
                      ),
                    ),
                  ),
                ),
              ),
            IconButton(
              tooltip: strings.next,
              onPressed: page >= pageCount
                  ? null
                  : () => onPageChanged(page + 1),
              icon: const Icon(Icons.chevron_right),
            ),
          ],
        ),
      ),
    );
  }
}

class _ErrorState extends StatelessWidget {
  const _ErrorState({
    required this.message,
    required this.retry,
    required this.strings,
  });

  final String message;
  final VoidCallback retry;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) => Center(
    child: Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(strings.revisionsUnavailable),
        const SizedBox(height: 8),
        Text(message),
        const SizedBox(height: 12),
        FilledButton(onPressed: retry, child: Text(strings.retry)),
      ],
    ),
  );
}
