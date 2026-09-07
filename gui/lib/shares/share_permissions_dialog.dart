import 'dart:async';
import 'dart:math' as math;

import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/permissions/permission_widgets.dart';
import 'package:davdeck/state/shares_controller.dart';
import 'package:davdeck/widgets/app_ui.dart';
import 'package:flutter/material.dart';

Future<void> showSharePermissionsDialog(
  BuildContext context,
  SharesController controller,
  ManagedShare share,
) async {
  await showAppDialog<void>(
    context: context,
    builder: (_) =>
        SharePermissionsDialog(controller: controller, share: share),
  );
}

class SharePermissionsDialog extends StatefulWidget {
  const SharePermissionsDialog({
    super.key,
    required this.controller,
    required this.share,
  });

  final SharesController controller;
  final ManagedShare share;

  @override
  State<SharePermissionsDialog> createState() => _SharePermissionsDialogState();
}

class _SharePermissionsDialogState extends State<SharePermissionsDialog> {
  final _searchController = TextEditingController();
  List<ManagedPermission> _entries = const [];
  PermissionFilter _filter = PermissionFilter.all;
  Object? _loadError;
  bool _loading = true;

  SharesController get controller => widget.controller;

  @override
  void initState() {
    super.initState();
    unawaited(_load());
  }

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _loadError = null;
    });
    try {
      final entries = await controller.permissions(widget.share);
      if (!mounted) return;
      setState(() => _entries = entries.toList());
    } catch (caught) {
      if (!mounted) return;
      setState(() => _loadError = caught);
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  List<ManagedPermission> get _visibleEntries {
    final query = _searchController.text.trim().toLowerCase();
    return _entries
        .where(
          (entry) =>
              (_filter == PermissionFilter.all || entry.permission != 'NONE') &&
              (query.isEmpty || entry.username.toLowerCase().contains(query)),
        )
        .toList(growable: false);
  }

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    final media = MediaQuery.sizeOf(context);
    final dialogWidth = math.min(760.0, math.max(320.0, media.width - 48));
    final dialogHeight = math.min(580.0, math.max(300.0, media.height * 0.68));
    return AlertDialog(
      title: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text('${widget.share.name} ${strings.accessPermission}'),
          const SizedBox(height: 4),
          Text(
            '/dav/${widget.share.slug}/',
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
              color: Theme.of(context).colorScheme.primary,
            ),
          ),
          const SizedBox(height: 3),
          Text(
            strings.sharePermissionsSubtitle,
            style: Theme.of(context).textTheme.bodySmall?.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 8),
          Align(
            alignment: Alignment.centerRight,
            child: _AutoSaveNotice(strings: strings),
          ),
        ],
      ),
      content: SizedBox(
        width: dialogWidth,
        height: dialogHeight,
        child: AnimatedBuilder(
          animation: controller,
          builder: (context, _) => Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              PermissionFilterBar(
                searchController: _searchController,
                searchHint: strings.searchUsersForShare,
                allLabel: strings.allUsers,
                filter: _filter,
                strings: strings,
                onSearchChanged: (_) => setState(() {}),
                onFilterChanged: (value) => setState(() => _filter = value),
              ),
              const SizedBox(height: 12),
              Expanded(child: _buildBody(context, strings)),
            ],
          ),
        ),
      ),
      actions: [
        FilledButton(
          onPressed: () => Navigator.pop(context),
          child: Text(strings.done),
        ),
      ],
    );
  }

  Widget _buildBody(BuildContext context, AppStrings strings) {
    if (_loading) return const Center(child: CircularProgressIndicator());
    if (_loadError != null) {
      return _DialogError(
        text: strings.permissionsLoadFailed,
        retry: _load,
        strings: strings,
      );
    }
    if (_entries.isEmpty) {
      return _DialogEmpty(icon: Icons.people_outline, text: strings.noUsers);
    }
    final entries = _visibleEntries;
    if (entries.isEmpty) {
      final text = _filter == PermissionFilter.authorized
          ? strings.noAuthorizedUsers
          : strings.noMatchingUsers;
      return _DialogEmpty(icon: Icons.search_off, text: text);
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _PermissionHeader(strings: strings, sharePerspective: false),
        const Divider(height: 1),
        Expanded(
          child: ListView.builder(
            itemCount: entries.length,
            itemBuilder: (context, index) => _SharePermissionRow(
              entry: entries[index],
              strings: strings,
              controller: controller,
              share: widget.share,
              onChanged: (permission) =>
                  _changePermission(entries[index], permission),
            ),
          ),
        ),
      ],
    );
  }

  Future<void> _changePermission(
    ManagedPermission entry,
    String permission,
  ) async {
    final index = _entries.indexWhere((value) => value.userId == entry.userId);
    if (index < 0) return;
    setState(() => _entries[index] = entry.copyWith(permission: permission));
    final success = await controller.setPermission(
      widget.share,
      entry,
      permission,
    );
    if (!mounted || _entries[index].permission != permission) return;
    if (!success) {
      setState(
        () => _entries[index] = _entries[index].copyWith(
          permission: controller.confirmedPermission(
            widget.share.id,
            entry.userId,
            entry.permission,
          ),
        ),
      );
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(AppStrings.of(context).permissionSaveFailed)),
      );
    }
  }
}

class _PermissionHeader extends StatelessWidget {
  const _PermissionHeader({
    required this.strings,
    required this.sharePerspective,
  });

  final AppStrings strings;
  final bool sharePerspective;

  @override
  Widget build(BuildContext context) {
    final style = Theme.of(context).textTheme.bodySmall?.copyWith(
      color: Theme.of(context).colorScheme.onSurfaceVariant,
      fontWeight: FontWeight.w600,
    );
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      child: Row(
        children: [
          Expanded(
            flex: 2,
            child: Text(
              sharePerspective ? strings.shareName : strings.username,
              style: style,
            ),
          ),
          Expanded(
            child: Text(
              sharePerspective ? strings.status : strings.accountStatus,
              style: style,
            ),
          ),
          Expanded(child: Text(strings.accessPermission, style: style)),
          Expanded(child: Text(strings.saveStatus, style: style)),
        ],
      ),
    );
  }
}

class _SharePermissionRow extends StatelessWidget {
  const _SharePermissionRow({
    required this.entry,
    required this.strings,
    required this.controller,
    required this.share,
    required this.onChanged,
  });

  final ManagedPermission entry;
  final AppStrings strings;
  final SharesController controller;
  final ManagedShare share;
  final ValueChanged<String> onChanged;

  @override
  Widget build(BuildContext context) => LayoutBuilder(
    builder: (context, constraints) {
      final wide = constraints.maxWidth >= 560;
      final status = AppStatusPill(
        label: entry.userEnabled ? strings.enabled : strings.disabled,
        color: entry.userEnabled
            ? Theme.of(context).colorScheme.primary
            : Theme.of(context).colorScheme.onSurfaceVariant,
      );
      final dropdown = PermissionDropdown(
        value: entry.permission,
        strings: strings,
        onChanged: (value) {
          if (value != null) onChanged(value);
        },
      );
      final indicator = PermissionSaveIndicator(
        state: controller.permissionSaveState(share.id, entry.userId),
        strings: strings,
      );
      if (wide) {
        return Padding(
          padding: const EdgeInsets.symmetric(horizontal: 12),
          child: Column(
            children: [
              const SizedBox(height: 5),
              Row(
                children: [
                  Expanded(flex: 2, child: Text(entry.username)),
                  Expanded(child: status),
                  Expanded(child: dropdown),
                  Expanded(child: indicator),
                ],
              ),
              const Divider(height: 10),
            ],
          ),
        );
      }
      return Padding(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 8),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              children: [
                Expanded(child: Text(entry.username)),
                status,
              ],
            ),
            const SizedBox(height: 8),
            Row(
              children: [
                Expanded(child: dropdown),
                const SizedBox(width: 12),
                indicator,
              ],
            ),
            const Divider(height: 12),
          ],
        ),
      );
    },
  );
}

class _AutoSaveNotice extends StatelessWidget {
  const _AutoSaveNotice({required this.strings});

  final AppStrings strings;

  @override
  Widget build(BuildContext context) => ConstrainedBox(
    constraints: const BoxConstraints(maxWidth: 300),
    child: Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(
          Icons.check_circle_outline,
          size: 18,
          color: Theme.of(context).colorScheme.primary,
        ),
        const SizedBox(width: 6),
        Flexible(child: Text(strings.automaticPermissionSave)),
      ],
    ),
  );
}

class _DialogEmpty extends StatelessWidget {
  const _DialogEmpty({required this.icon, required this.text});

  final IconData icon;
  final String text;

  @override
  Widget build(BuildContext context) => Center(
    child: Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(
          icon,
          size: 38,
          color: Theme.of(context).colorScheme.onSurfaceVariant,
        ),
        const SizedBox(height: 10),
        Text(text, textAlign: TextAlign.center),
      ],
    ),
  );
}

class _DialogError extends StatelessWidget {
  const _DialogError({
    required this.text,
    required this.retry,
    required this.strings,
  });

  final String text;
  final Future<void> Function() retry;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) => Center(
    child: Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(
          Icons.error_outline,
          size: 38,
          color: Theme.of(context).colorScheme.error,
        ),
        const SizedBox(height: 10),
        Text(text),
        const SizedBox(height: 12),
        OutlinedButton(onPressed: retry, child: Text(strings.retry)),
      ],
    ),
  );
}
