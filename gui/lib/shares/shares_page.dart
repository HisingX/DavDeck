import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/state/shares_controller.dart';
import 'package:davdeck/widgets/app_ui.dart';
import 'package:flutter/material.dart';

class SharesPage extends StatefulWidget {
  const SharesPage({super.key, required this.controller});

  final SharesController controller;

  @override
  State<SharesPage> createState() => _SharesPageState();
}

class _SharesPageState extends State<SharesPage> {
  final _searchController = TextEditingController();

  SharesController get controller => widget.controller;

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return Scaffold(
      body: AnimatedBuilder(
        animation: controller,
        builder: (context, _) {
          if (controller.loading) {
            return const Center(child: CircularProgressIndicator());
          }
          if (controller.error != null && controller.shares.isEmpty) {
            return _ErrorState(
              strings: strings,
              message: strings.sharesUnavailable,
              onRetry: controller.refresh,
            );
          }

          final query = _searchController.text.trim().toLowerCase();
          final visibleShares = query.isEmpty
              ? controller.shares
              : controller.shares
                    .where(
                      (share) =>
                          share.name.toLowerCase().contains(query) ||
                          share.slug.toLowerCase().contains(query) ||
                          share.path.toLowerCase().contains(query),
                    )
                    .toList(growable: false);
          final enabled = controller.shares
              .where((share) => share.enabled)
              .length;

          return Stack(
            children: [
              Positioned.fill(
                child: _SharesContent(
                  strings: strings,
                  shares: visibleShares,
                  totalShares: controller.shares.length,
                  enabledShares: enabled,
                  busy: controller.busy,
                  searchController: _searchController,
                  error: controller.error,
                  onSearchChanged: (_) => setState(() {}),
                  onAdd: controller.busy ? null : () => _editShare(context),
                  onToggle: (share, value) =>
                      controller.update(share, enabled: value),
                  onPermissions: (share) => _showPermissions(context, share),
                  onEdit: (share) => _editShare(context, share),
                  onDelete: (share) => _confirmDelete(context, share),
                ),
              ),
              if (controller.busy)
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

  Future<void> _editShare(BuildContext context, [ManagedShare? share]) async {
    final strings = AppStrings.of(context);
    final name = TextEditingController(text: share?.name);
    final slug = TextEditingController(text: share?.slug);
    final path = TextEditingController(text: share?.path);
    try {
      await showAppDialog<void>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: Text(share == null ? strings.addShare : strings.editShare),
          content: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                TextField(
                  controller: name,
                  autofocus: true,
                  decoration: InputDecoration(labelText: strings.shareName),
                ),
                TextField(
                  controller: slug,
                  decoration: InputDecoration(labelText: strings.slug),
                ),
                TextField(
                  controller: path,
                  decoration: InputDecoration(labelText: strings.folderPath),
                ),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: Text(strings.cancel),
            ),
            FilledButton(
              onPressed: () async {
                final success = share == null
                    ? await controller.create(name.text, slug.text, path.text)
                    : await controller.update(
                        share,
                        name: name.text,
                        slug: slug.text,
                        path: path.text,
                      );
                if (success && dialogContext.mounted) {
                  Navigator.pop(dialogContext);
                }
              },
              child: Text(strings.save),
            ),
          ],
        ),
      );
    } finally {
      await Future<void>.delayed(const Duration(milliseconds: 350));
      name.dispose();
      slug.dispose();
      path.dispose();
    }
  }

  Future<void> _showPermissions(
    BuildContext context,
    ManagedShare share,
  ) async {
    final strings = AppStrings.of(context);
    var entries = await controller.permissions(share);
    if (!context.mounted) return;
    await showAppDialog<void>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setState) => AlertDialog(
          title: Text('${strings.permissions}: ${share.name}'),
          content: SizedBox(
            width: 440,
            child: entries.isEmpty
                ? Text(strings.noUsers)
                : ListView.builder(
                    shrinkWrap: true,
                    itemCount: entries.length,
                    itemBuilder: (context, index) {
                      final entry = entries[index];
                      return ListTile(
                        title: Text(entry.username),
                        trailing: DropdownButton<String>(
                          value: entry.permission,
                          items:
                              [
                                    ('NONE', strings.noAccess),
                                    ('READ', strings.readOnly),
                                    ('READ_WRITE', strings.readWrite),
                                  ]
                                  .map(
                                    (value) => DropdownMenuItem(
                                      value: value.$1,
                                      child: Text(value.$2),
                                    ),
                                  )
                                  .toList(),
                          onChanged: controller.busy
                              ? null
                              : (value) async {
                                  if (value == null) return;
                                  if (await controller.setPermission(
                                    share,
                                    entry,
                                    value,
                                  )) {
                                    final refreshed = await controller
                                        .permissions(share);
                                    if (dialogContext.mounted) {
                                      setState(() => entries = refreshed);
                                    }
                                  }
                                },
                        ),
                      );
                    },
                  ),
          ),
          actions: [
            FilledButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: Text(strings.close),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _confirmDelete(BuildContext context, ManagedShare share) async {
    final strings = AppStrings.of(context);
    final confirmed = await showAppDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(strings.deleteShare),
        content: Text(strings.deleteSharePreservesFiles(share.name)),
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
    if (confirmed == true) await controller.delete(share);
  }
}

class _SharesContent extends StatelessWidget {
  const _SharesContent({
    required this.strings,
    required this.shares,
    required this.totalShares,
    required this.enabledShares,
    required this.busy,
    required this.searchController,
    required this.error,
    required this.onSearchChanged,
    required this.onAdd,
    required this.onToggle,
    required this.onPermissions,
    required this.onEdit,
    required this.onDelete,
  });

  final AppStrings strings;
  final List<ManagedShare> shares;
  final int totalShares;
  final int enabledShares;
  final bool busy;
  final TextEditingController searchController;
  final Object? error;
  final ValueChanged<String> onSearchChanged;
  final VoidCallback? onAdd;
  final Future<void> Function(ManagedShare share, bool value) onToggle;
  final Future<void> Function(ManagedShare share) onPermissions;
  final Future<void> Function(ManagedShare share) onEdit;
  final Future<void> Function(ManagedShare share) onDelete;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final disabled = totalShares - enabledShares;
    return SingleChildScrollView(
      padding: appPagePadding(context),
      child: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 1120),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              AppPageHeader(
                title: strings.shares,
                subtitle: strings.sharesSubtitle,
                actions: Wrap(
                  spacing: 12,
                  runSpacing: 10,
                  children: [
                    AppSearchField(
                      width: 280,
                      controller: searchController,
                      hintText: strings.searchSharesHint,
                      clearTooltip: strings.clearSearch,
                      onChanged: onSearchChanged,
                    ),
                    FilledButton.icon(
                      onPressed: onAdd,
                      icon: const Icon(Icons.add),
                      label: Text(strings.addShare),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 32),
              _ShareSummary(
                strings: strings,
                total: totalShares,
                enabled: enabledShares,
                disabled: disabled,
              ),
              if (error != null) ...[
                const SizedBox(height: 16),
                AppNotice(
                  icon: Icons.error_outline,
                  text: error.toString(),
                  color: theme.colorScheme.errorContainer,
                  textColor: theme.colorScheme.onErrorContainer,
                ),
              ],
              const SizedBox(height: 30),
              if (shares.isEmpty)
                _EmptyShares(strings: strings, filtered: totalShares > 0)
              else
                _ShareList(
                  strings: strings,
                  shares: shares,
                  busy: busy,
                  onToggle: onToggle,
                  onPermissions: onPermissions,
                  onEdit: onEdit,
                  onDelete: onDelete,
                ),
            ],
          ),
        ),
      ),
    );
  }
}

class _ShareSummary extends StatelessWidget {
  const _ShareSummary({
    required this.strings,
    required this.total,
    required this.enabled,
    required this.disabled,
  });

  final AppStrings strings;
  final int total;
  final int enabled;
  final int disabled;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final width = MediaQuery.sizeOf(context).width;
    final columns = width > 1100
        ? 4
        : width > 700
        ? 2
        : 1;
    return AppSurface(
      padding: const EdgeInsets.symmetric(horizontal: 22, vertical: 24),
      child: LayoutBuilder(
        builder: (context, constraints) => Wrap(
          children: [
            _metric(
              context,
              constraints,
              columns,
              strings.sharesTotal,
              total,
              Icons.folder_outlined,
              theme.colorScheme.primary,
              false,
            ),
            _metric(
              context,
              constraints,
              columns,
              strings.sharesEnabled,
              enabled,
              Icons.check_circle_outline,
              const Color(0xff21865d),
              columns == 4,
            ),
            _metric(
              context,
              constraints,
              columns,
              strings.sharesDisabled,
              disabled,
              Icons.pause_circle_outline,
              theme.colorScheme.onSurfaceVariant,
              columns == 4,
            ),
            _metric(
              context,
              constraints,
              columns,
              strings.protocol,
              'WebDAV',
              Icons.language,
              theme.colorScheme.onSurfaceVariant,
              columns == 4,
            ),
          ],
        ),
      ),
    );
  }

  Widget _metric(
    BuildContext context,
    BoxConstraints constraints,
    int columns,
    String label,
    Object value,
    IconData icon,
    Color color,
    bool showDivider,
  ) {
    final theme = Theme.of(context);
    return SizedBox(
      width: constraints.maxWidth / columns,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 2),
        decoration: showDivider
            ? BoxDecoration(
                border: Border(
                  left: BorderSide(color: theme.colorScheme.outlineVariant),
                ),
              )
            : null,
        child: Row(
          children: [
            Container(
              width: 46,
              height: 46,
              decoration: BoxDecoration(
                color: color.withValues(alpha: 0.09),
                shape: BoxShape.circle,
              ),
              child: Icon(icon, color: color, size: 23),
            ),
            const SizedBox(width: 14),
            Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  '$value',
                  style: theme.textTheme.titleLarge?.copyWith(
                    fontWeight: FontWeight.w700,
                  ),
                ),
                Text(
                  label,
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _ShareList extends StatelessWidget {
  const _ShareList({
    required this.strings,
    required this.shares,
    required this.busy,
    required this.onToggle,
    required this.onPermissions,
    required this.onEdit,
    required this.onDelete,
  });

  final AppStrings strings;
  final List<ManagedShare> shares;
  final bool busy;
  final Future<void> Function(ManagedShare share, bool value) onToggle;
  final Future<void> Function(ManagedShare share) onPermissions;
  final Future<void> Function(ManagedShare share) onEdit;
  final Future<void> Function(ManagedShare share) onDelete;

  @override
  Widget build(BuildContext context) => LayoutBuilder(
    builder: (context, constraints) {
      final wide = constraints.maxWidth >= 920;
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          if (wide) _ShareTableHeader(strings: strings),
          ...shares.map(
            (share) => Padding(
              padding: const EdgeInsets.only(bottom: 12),
              child: _ShareCard(
                strings: strings,
                share: share,
                busy: busy,
                wide: wide,
                onToggle: onToggle,
                onPermissions: onPermissions,
                onEdit: onEdit,
                onDelete: onDelete,
              ),
            ),
          ),
        ],
      );
    },
  );
}

class _ShareTableHeader extends StatelessWidget {
  const _ShareTableHeader({required this.strings});

  final AppStrings strings;

  @override
  Widget build(BuildContext context) {
    final style = Theme.of(context).textTheme.bodySmall?.copyWith(
      color: Theme.of(context).colorScheme.onSurfaceVariant,
      fontWeight: FontWeight.w600,
    );
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 0, 72, 10),
      child: Row(
        children: [
          Expanded(flex: 2, child: Text(strings.shareName, style: style)),
          Expanded(flex: 2, child: Text(strings.webdavPath, style: style)),
          Expanded(flex: 2, child: Text(strings.localDirectory, style: style)),
          Expanded(child: Text(strings.protocol, style: style)),
          Expanded(child: Text(strings.status, style: style)),
        ],
      ),
    );
  }
}

class _ShareCard extends StatelessWidget {
  const _ShareCard({
    required this.strings,
    required this.share,
    required this.busy,
    required this.wide,
    required this.onToggle,
    required this.onPermissions,
    required this.onEdit,
    required this.onDelete,
  });

  final AppStrings strings;
  final ManagedShare share;
  final bool busy;
  final bool wide;
  final Future<void> Function(ManagedShare share, bool value) onToggle;
  final Future<void> Function(ManagedShare share) onPermissions;
  final Future<void> Function(ManagedShare share) onEdit;
  final Future<void> Function(ManagedShare share) onDelete;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final statusColor = share.enabled
        ? theme.colorScheme.primary
        : theme.colorScheme.onSurfaceVariant;
    final top = Row(
      children: [
        Container(
          width: 42,
          height: 42,
          decoration: BoxDecoration(
            color: share.enabled
                ? theme.colorScheme.primaryContainer
                : theme.colorScheme.surfaceContainerHighest,
            borderRadius: BorderRadius.circular(12),
          ),
          child: Icon(
            share.enabled ? Icons.folder_shared_outlined : Icons.folder_off,
            color: share.enabled
                ? theme.colorScheme.onPrimaryContainer
                : theme.colorScheme.onSurfaceVariant,
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          flex: 2,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                share.name,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: theme.textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.w700,
                ),
              ),
              const SizedBox(height: 4),
              Text(
                '/dav/${share.slug}/',
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.primary,
                ),
              ),
            ],
          ),
        ),
        if (wide)
          Expanded(
            flex: 2,
            child: _ShareValue(
              icon: Icons.folder_open_outlined,
              value: '/dav/${share.slug}/',
              label: strings.webdavPath,
            ),
          ),
        if (wide)
          Expanded(
            flex: 2,
            child: _ShareValue(
              icon: Icons.folder_outlined,
              value: share.path,
              label: strings.localDirectory,
            ),
          ),
        if (wide)
          Expanded(
            child: _ShareValue(
              icon: Icons.language,
              value: 'WebDAV',
              label: strings.protocol,
            ),
          ),
        AppStatusPill(
          label: share.enabled ? strings.enabled : strings.disabled,
          color: statusColor,
        ),
        Switch(
          value: share.enabled,
          onChanged: busy ? null : (value) => onToggle(share, value),
        ),
        PopupMenuButton<String>(
          enabled: !busy,
          tooltip: strings.shareActions,
          onSelected: (value) {
            if (value == 'permissions') onPermissions(share);
            if (value == 'edit') onEdit(share);
            if (value == 'delete') onDelete(share);
          },
          itemBuilder: (_) => [
            PopupMenuItem(
              value: 'permissions',
              child: Text(strings.permissions),
            ),
            PopupMenuItem(value: 'edit', child: Text(strings.edit)),
            PopupMenuItem(value: 'delete', child: Text(strings.delete)),
          ],
        ),
      ],
    );
    return AppSurface(
      padding: const EdgeInsets.fromLTRB(18, 16, 10, 14),
      child: Column(
        children: [
          top,
          if (!wide) ...[
            const SizedBox(height: 14),
            Align(
              alignment: Alignment.centerLeft,
              child: Wrap(
                spacing: 18,
                runSpacing: 8,
                children: [
                  _ShareValue(
                    icon: Icons.folder_open_outlined,
                    value: '/dav/${share.slug}/',
                    label: strings.webdavPath,
                  ),
                  _ShareValue(
                    icon: Icons.folder_outlined,
                    value: share.path,
                    label: strings.localDirectory,
                  ),
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }
}

class _ShareValue extends StatelessWidget {
  const _ShareValue({
    required this.icon,
    required this.value,
    required this.label,
  });

  final IconData icon;
  final String value;
  final String label;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(icon, size: 18, color: theme.colorScheme.onSurfaceVariant),
        const SizedBox(width: 8),
        Flexible(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                value,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: theme.textTheme.bodyMedium,
              ),
              Text(
                label,
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _EmptyShares extends StatelessWidget {
  const _EmptyShares({required this.strings, required this.filtered});

  final AppStrings strings;
  final bool filtered;

  @override
  Widget build(BuildContext context) => AppSurface(
    padding: const EdgeInsets.symmetric(vertical: 46),
    child: Column(
      children: [
        Icon(
          filtered ? Icons.search_off : Icons.folder_copy_outlined,
          size: 42,
          color: Theme.of(context).colorScheme.onSurfaceVariant,
        ),
        const SizedBox(height: 12),
        Text(filtered ? strings.noMatchingShares : strings.noShares),
      ],
    ),
  );
}

class _ErrorState extends StatelessWidget {
  const _ErrorState({
    required this.strings,
    required this.message,
    required this.onRetry,
  });

  final AppStrings strings;
  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) => Center(
    child: Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(
          Icons.folder_off_outlined,
          size: 42,
          color: Theme.of(context).colorScheme.onSurfaceVariant,
        ),
        const SizedBox(height: 12),
        Text(message),
        const SizedBox(height: 12),
        FilledButton(onPressed: onRetry, child: Text(strings.retry)),
      ],
    ),
  );
}
