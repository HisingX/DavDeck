import 'dart:async';
import 'dart:math' as math;

import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/permissions/permission_widgets.dart';
import 'package:davdeck/state/users_controller.dart';
import 'package:davdeck/widgets/app_ui.dart';
import 'package:flutter/material.dart';

Future<void> showUserPermissionsDialog(
  BuildContext context,
  UsersController controller,
  ManagedUser user,
) async {
  await showAppDialog<void>(
    context: context,
    builder: (_) => UserPermissionsDialog(controller: controller, user: user),
  );
}

class UserPermissionsDialog extends StatefulWidget {
  const UserPermissionsDialog({
    super.key,
    required this.controller,
    required this.user,
  });

  final UsersController controller;
  final ManagedUser user;

  @override
  State<UserPermissionsDialog> createState() => _UserPermissionsDialogState();
}

class _UserPermissionsDialogState extends State<UserPermissionsDialog> {
  final _searchController = TextEditingController();
  List<ManagedUserPermission> _entries = const [];
  PermissionFilter _filter = PermissionFilter.all;
  Object? _loadError;
  bool _loading = true;

  UsersController get controller => widget.controller;

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
      final entries = await controller.userPermissions(widget.user);
      if (!mounted) return;
      setState(() => _entries = entries.toList());
    } catch (caught) {
      if (!mounted) return;
      setState(() => _loadError = caught);
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  List<ManagedUserPermission> get _visibleEntries {
    final query = _searchController.text.trim().toLowerCase();
    return _entries
        .where(
          (entry) =>
              (_filter == PermissionFilter.all || entry.permission != 'NONE') &&
              (query.isEmpty ||
                  entry.shareName.toLowerCase().contains(query) ||
                  entry.shareSlug.toLowerCase().contains(query) ||
                  '/dav/${entry.shareSlug}/'.contains(query)),
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
          Row(
            children: [
              Expanded(
                child: Text(strings.userSharePermissions(widget.user.username)),
              ),
              if (!widget.user.enabled)
                AppStatusPill(
                  label: strings.disabled,
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
            ],
          ),
          const SizedBox(height: 4),
          Text(
            strings.userPermissionsSubtitle,
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 8),
          Align(
            alignment: Alignment.centerRight,
            child: _UserAutoSaveNotice(strings: strings),
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
                searchHint: strings.searchSharesForUser,
                allLabel: strings.allShares,
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
      return _UserDialogError(
        text: strings.permissionsLoadFailed,
        retry: _load,
        strings: strings,
      );
    }
    if (_entries.isEmpty) {
      return _UserDialogEmpty(
        icon: Icons.folder_copy_outlined,
        text: strings.noShares,
      );
    }
    final entries = _visibleEntries;
    if (entries.isEmpty) {
      final text = _filter == PermissionFilter.authorized
          ? strings.noAuthorizedShares
          : strings.noMatchingShares;
      return _UserDialogEmpty(icon: Icons.search_off, text: text);
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _UserPermissionHeader(strings: strings),
        const Divider(height: 1),
        Expanded(
          child: ListView.builder(
            itemCount: entries.length,
            itemBuilder: (context, index) => _UserPermissionRow(
              entry: entries[index],
              user: widget.user,
              controller: controller,
              strings: strings,
              onChanged: (permission) =>
                  _changePermission(entries[index], permission),
            ),
          ),
        ),
      ],
    );
  }

  Future<void> _changePermission(
    ManagedUserPermission entry,
    String permission,
  ) async {
    final index = _entries.indexWhere(
      (value) => value.shareId == entry.shareId,
    );
    if (index < 0) return;
    setState(() => _entries[index] = entry.copyWith(permission: permission));
    final success = await controller.setPermission(
      widget.user,
      entry,
      permission,
    );
    if (!mounted || _entries[index].permission != permission) return;
    if (!success) {
      setState(
        () => _entries[index] = _entries[index].copyWith(
          permission: controller.confirmedPermission(
            entry.shareId,
            widget.user.id,
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

class _UserPermissionHeader extends StatelessWidget {
  const _UserPermissionHeader({required this.strings});

  final AppStrings strings;

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
          Expanded(flex: 2, child: Text(strings.shareName, style: style)),
          Expanded(child: Text(strings.status, style: style)),
          Expanded(child: Text(strings.accessPermission, style: style)),
          Expanded(child: Text(strings.saveStatus, style: style)),
        ],
      ),
    );
  }
}

class _UserPermissionRow extends StatelessWidget {
  const _UserPermissionRow({
    required this.entry,
    required this.user,
    required this.controller,
    required this.strings,
    required this.onChanged,
  });

  final ManagedUserPermission entry;
  final ManagedUser user;
  final UsersController controller;
  final AppStrings strings;
  final ValueChanged<String> onChanged;

  @override
  Widget build(BuildContext context) => LayoutBuilder(
    builder: (context, constraints) {
      final wide = constraints.maxWidth >= 560;
      final status = AppStatusPill(
        label: entry.shareEnabled ? strings.enabled : strings.disabled,
        color: entry.shareEnabled
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
        state: controller.permissionSaveState(entry.shareId, user.id),
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
                  Expanded(
                    flex: 2,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(entry.shareName),
                        Text(
                          '/dav/${entry.shareSlug}/',
                          style: Theme.of(context).textTheme.bodySmall
                              ?.copyWith(
                                color: Theme.of(context).colorScheme.primary,
                              ),
                        ),
                      ],
                    ),
                  ),
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
                Expanded(child: Text(entry.shareName)),
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

class _UserAutoSaveNotice extends StatelessWidget {
  const _UserAutoSaveNotice({required this.strings});

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

class _UserDialogEmpty extends StatelessWidget {
  const _UserDialogEmpty({required this.icon, required this.text});

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

class _UserDialogError extends StatelessWidget {
  const _UserDialogError({
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
