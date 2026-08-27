import 'dart:convert';

import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/state/users_controller.dart';
import 'package:davdeck/widgets/app_ui.dart';
import 'package:flutter/material.dart';

class UsersPage extends StatefulWidget {
  const UsersPage({super.key, required this.controller});

  final UsersController controller;

  @override
  State<UsersPage> createState() => _UsersPageState();
}

class _UsersPageState extends State<UsersPage> {
  final _searchController = TextEditingController();

  UsersController get controller => widget.controller;

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
          if (controller.loadError != null && controller.users.isEmpty) {
            return _ErrorView(controller: controller, strings: strings);
          }

          final query = _searchController.text.trim().toLowerCase();
          final visibleUsers = query.isEmpty
              ? controller.users
              : controller.users
                    .where(
                      (user) => user.username.toLowerCase().contains(query),
                    )
                    .toList(growable: false);
          final enabledUsers = controller.users
              .where((user) => user.enabled)
              .length;
          final disabledUsers = controller.users.length - enabledUsers;

          return Stack(
            children: [
              Positioned.fill(
                child: _UsersContent(
                  strings: strings,
                  users: visibleUsers,
                  totalUsers: controller.users.length,
                  enabledUsers: enabledUsers,
                  disabledUsers: disabledUsers,
                  searchController: _searchController,
                  busy: controller.busy,
                  onSearchChanged: (_) => setState(() {}),
                  onAddUser: controller.busy
                      ? null
                      : () => _showCreate(context),
                  onToggle: (user, value) => controller.setEnabled(user, value),
                  onChangePassword: (user) => _showPassword(context, user),
                  onDelete: (user) => _confirmDelete(context, user),
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

  Future<void> _showCreate(BuildContext context) async {
    final strings = AppStrings.of(context);
    final username = TextEditingController();
    final password = TextEditingController();
    final formKey = GlobalKey<FormState>();
    try {
      await showDialog<void>(
        context: context,
        builder: (dialogContext) {
          Object? submitError;
          return StatefulBuilder(
            builder: (context, setDialogState) => AlertDialog(
              title: Text(strings.addUser),
              content: Form(
                key: formKey,
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    TextFormField(
                      controller: username,
                      autofocus: true,
                      decoration: InputDecoration(labelText: strings.username),
                      validator: (value) =>
                          value == null || value.trim().isEmpty
                          ? strings.usernameRequired
                          : null,
                    ),
                    TextFormField(
                      controller: password,
                      obscureText: true,
                      decoration: InputDecoration(labelText: strings.password),
                      validator: (value) {
                        final length = utf8.encode(value ?? '').length;
                        return length < 8 || length > 72
                            ? strings.passwordLengthRequirement
                            : null;
                      },
                    ),
                    if (submitError != null) ...[
                      const SizedBox(height: 12),
                      Text(
                        _createUserError(strings, submitError!),
                        style: TextStyle(
                          color: Theme.of(context).colorScheme.error,
                        ),
                      ),
                    ],
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
                    if (!formKey.currentState!.validate()) return;
                    final success = await controller.create(
                      username.text,
                      password.text,
                    );
                    password.clear();
                    if (success && dialogContext.mounted) {
                      Navigator.pop(dialogContext);
                    } else if (dialogContext.mounted) {
                      setDialogState(
                        () => submitError = controller.actionError,
                      );
                    }
                  },
                  child: Text(strings.create),
                ),
              ],
            ),
          );
        },
      );
    } finally {
      password.clear();
      await Future<void>.delayed(const Duration(milliseconds: 350));
      username.dispose();
      password.dispose();
    }
  }

  Future<void> _showPassword(BuildContext context, ManagedUser user) async {
    final strings = AppStrings.of(context);
    final password = TextEditingController();
    try {
      await showDialog<void>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: Text('${strings.changePassword}: ${user.username}'),
          content: TextField(
            controller: password,
            autofocus: true,
            obscureText: true,
            decoration: InputDecoration(labelText: strings.newPassword),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: Text(strings.cancel),
            ),
            FilledButton(
              onPressed: () async {
                final success = await controller.changePassword(
                  user,
                  password.text,
                );
                password.clear();
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
      password.clear();
      await Future<void>.delayed(const Duration(milliseconds: 350));
      password.dispose();
    }
  }

  Future<void> _confirmDelete(BuildContext context, ManagedUser user) async {
    final strings = AppStrings.of(context);
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(strings.deleteUser),
        content: Text(strings.deleteUserPreservesFiles(user.username)),
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
    if (confirmed == true) await controller.delete(user);
  }

  String _createUserError(AppStrings strings, Object error) {
    if (error is DaemonApiException) {
      return switch (error.code) {
        'USER_ALREADY_EXISTS' => strings.usernameAlreadyExists,
        'INVALID_USERNAME' => strings.invalidUsername,
        'INVALID_PASSWORD' => strings.passwordLengthRequirement,
        _ => strings.createUserFailed,
      };
    }
    return strings.createUserFailed;
  }
}

class _UsersContent extends StatelessWidget {
  const _UsersContent({
    required this.strings,
    required this.users,
    required this.totalUsers,
    required this.enabledUsers,
    required this.disabledUsers,
    required this.searchController,
    required this.busy,
    required this.onSearchChanged,
    required this.onAddUser,
    required this.onToggle,
    required this.onChangePassword,
    required this.onDelete,
  });

  final AppStrings strings;
  final List<ManagedUser> users;
  final int totalUsers;
  final int enabledUsers;
  final int disabledUsers;
  final TextEditingController searchController;
  final bool busy;
  final ValueChanged<String> onSearchChanged;
  final VoidCallback? onAddUser;
  final Future<void> Function(ManagedUser user, bool value) onToggle;
  final Future<void> Function(ManagedUser user) onChangePassword;
  final Future<void> Function(ManagedUser user) onDelete;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return SingleChildScrollView(
      padding: appPagePadding(context),
      child: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 1120),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              _PageHeader(
                strings: strings,
                searchController: searchController,
                busy: busy,
                onSearchChanged: onSearchChanged,
                onAddUser: onAddUser,
              ),
              const SizedBox(height: 32),
              _UsersSummary(
                strings: strings,
                total: totalUsers,
                enabled: enabledUsers,
                disabled: disabledUsers,
              ),
              const SizedBox(height: 30),
              if (users.isEmpty)
                _EmptyUsers(strings: strings, filtered: totalUsers > 0)
              else ...[
                Row(
                  children: [
                    Text(
                      strings.users,
                      style: theme.textTheme.titleLarge?.copyWith(
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    const Spacer(),
                    Text(
                      strings.usersCount(users.length),
                      style: theme.textTheme.bodyMedium?.copyWith(
                        color: theme.colorScheme.onSurfaceVariant,
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 12),
                ...users.map(
                  (user) => Padding(
                    padding: const EdgeInsets.only(bottom: 12),
                    child: _UserCard(
                      strings: strings,
                      user: user,
                      busy: busy,
                      onToggle: onToggle,
                      onChangePassword: onChangePassword,
                      onDelete: onDelete,
                    ),
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}

class _PageHeader extends StatelessWidget {
  const _PageHeader({
    required this.strings,
    required this.searchController,
    required this.busy,
    required this.onSearchChanged,
    required this.onAddUser,
  });

  final AppStrings strings;
  final TextEditingController searchController;
  final bool busy;
  final ValueChanged<String> onSearchChanged;
  final VoidCallback? onAddUser;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return LayoutBuilder(
      builder: (context, constraints) {
        final compact = constraints.maxWidth < 520;
        final actions = _UserActions(
          strings: strings,
          searchController: searchController,
          busy: busy,
          onSearchChanged: onSearchChanged,
          onAddUser: onAddUser,
          compact: compact,
        );

        final title = Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              strings.users,
              style: theme.textTheme.headlineMedium?.copyWith(
                fontWeight: FontWeight.w700,
                letterSpacing: -0.4,
              ),
            ),
            const SizedBox(height: 6),
            Text(
              strings.usersSubtitle,
              style: theme.textTheme.bodyLarge?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ],
        );

        if (compact) {
          return Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [title, const SizedBox(height: 20), actions],
          );
        }

        return Row(
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            Expanded(child: title),
            const SizedBox(width: 24),
            actions,
          ],
        );
      },
    );
  }
}

class _UserActions extends StatelessWidget {
  const _UserActions({
    required this.strings,
    required this.searchController,
    required this.busy,
    required this.onSearchChanged,
    required this.onAddUser,
    required this.compact,
  });

  final AppStrings strings;
  final TextEditingController searchController;
  final bool busy;
  final ValueChanged<String> onSearchChanged;
  final VoidCallback? onAddUser;
  final bool compact;

  @override
  Widget build(BuildContext context) {
    final search = SizedBox(
      width: compact ? null : 270,
      child: _UserSearchField(
        controller: searchController,
        hintText: strings.searchUsersHint,
        clearTooltip: strings.clearSearch,
        onChanged: onSearchChanged,
      ),
    );
    final addButton = FilledButton.icon(
      onPressed: onAddUser,
      icon: const Icon(Icons.person_add_outlined),
      label: Text(strings.addUser),
    );

    if (compact) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [search, const SizedBox(height: 12), addButton],
      );
    }

    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [search, const SizedBox(width: 12), addButton],
    );
  }
}

class _UserSearchField extends StatefulWidget {
  const _UserSearchField({
    required this.controller,
    required this.hintText,
    required this.clearTooltip,
    required this.onChanged,
  });

  final TextEditingController controller;
  final String hintText;
  final String clearTooltip;
  final ValueChanged<String> onChanged;

  @override
  State<_UserSearchField> createState() => _UserSearchFieldState();
}

class _UserSearchFieldState extends State<_UserSearchField> {
  late final FocusNode _focusNode;

  @override
  void initState() {
    super.initState();
    _focusNode = FocusNode()..addListener(_handleFocusChange);
  }

  @override
  void dispose() {
    _focusNode
      ..removeListener(_handleFocusChange)
      ..dispose();
    super.dispose();
  }

  void _handleFocusChange() => setState(() {});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    return Container(
      constraints: const BoxConstraints(minHeight: 56),
      padding: const EdgeInsets.only(left: 16, right: 6),
      decoration: BoxDecoration(
        color: scheme.surface,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(
          color: _focusNode.hasFocus ? scheme.primary : scheme.outline,
          width: _focusNode.hasFocus ? 2 : 1,
        ),
      ),
      child: Row(
        children: [
          Icon(Icons.search, color: scheme.onSurfaceVariant),
          const SizedBox(width: 10),
          Expanded(
            child: Stack(
              alignment: Alignment.centerLeft,
              children: [
                if (widget.controller.text.isEmpty)
                  IgnorePointer(
                    child: Text(
                      widget.hintText,
                      style: theme.textTheme.bodyLarge?.copyWith(
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                  ),
                EditableText(
                  controller: widget.controller,
                  focusNode: _focusNode,
                  style: theme.textTheme.bodyLarge!,
                  cursorColor: scheme.primary,
                  backgroundCursorColor: scheme.onSurface,
                  selectionColor: scheme.primary.withValues(alpha: 0.2),
                  maxLines: 1,
                  textInputAction: TextInputAction.search,
                  onChanged: (value) {
                    setState(() {});
                    widget.onChanged(value);
                  },
                  selectionControls: materialTextSelectionControls,
                ),
              ],
            ),
          ),
          if (widget.controller.text.isNotEmpty)
            IconButton(
              tooltip: widget.clearTooltip,
              onPressed: () {
                widget.controller.clear();
                setState(() {});
                widget.onChanged('');
              },
              icon: const Icon(Icons.close),
            ),
        ],
      ),
    );
  }
}

class _UsersSummary extends StatelessWidget {
  const _UsersSummary({
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
    final metrics = [
      _SummaryMetric(
        label: strings.usersTotal,
        value: total,
        icon: Icons.people_alt_outlined,
        color: theme.colorScheme.primary,
      ),
      _SummaryMetric(
        label: strings.usersEnabled,
        value: enabled,
        icon: Icons.check_circle_outline,
        color: const Color(0xff21865d),
      ),
      _SummaryMetric(
        label: strings.usersInactive,
        value: disabled,
        icon: Icons.pause_circle_outline,
        color: theme.colorScheme.onSurfaceVariant,
      ),
    ];
    return AppSurface(
      padding: const EdgeInsets.symmetric(horizontal: 22, vertical: 24),
      child: LayoutBuilder(
        builder: (context, constraints) {
          if (constraints.maxWidth < 420) {
            return Column(
              children: [
                for (var i = 0; i < metrics.length; i++) ...[
                  metrics[i],
                  if (i < metrics.length - 1) const Divider(height: 25),
                ],
              ],
            );
          }
          return Row(
            children: [
              for (var i = 0; i < metrics.length; i++) ...[
                Expanded(child: metrics[i]),
                if (i < metrics.length - 1)
                  Container(
                    width: 1,
                    height: 52,
                    color: theme.colorScheme.outlineVariant,
                  ),
              ],
            ],
          );
        },
      ),
    );
  }
}

class _SummaryMetric extends StatelessWidget {
  const _SummaryMetric({
    required this.label,
    required this.value,
    required this.icon,
    required this.color,
  });

  final String label;
  final int value;
  final IconData icon;
  final Color color;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return LayoutBuilder(
      builder: (context, constraints) {
        final compact = constraints.maxWidth < 180;
        final iconSize = compact ? 32.0 : 46.0;
        return Padding(
          padding: EdgeInsets.symmetric(horizontal: compact ? 6 : 18),
          child: Row(
            children: [
              Container(
                width: iconSize,
                height: iconSize,
                decoration: BoxDecoration(
                  color: color.withValues(alpha: 0.09),
                  shape: BoxShape.circle,
                ),
                child: Icon(icon, color: color, size: compact ? 18 : 23),
              ),
              SizedBox(width: compact ? 7 : 14),
              Expanded(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      '$value',
                      style: theme.textTheme.titleLarge?.copyWith(
                        fontWeight: FontWeight.w700,
                        color: theme.colorScheme.onSurface,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      label,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: theme.textTheme.bodyMedium?.copyWith(
                        color: theme.colorScheme.onSurfaceVariant,
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}

class _UserCard extends StatelessWidget {
  const _UserCard({
    required this.strings,
    required this.user,
    required this.busy,
    required this.onToggle,
    required this.onChangePassword,
    required this.onDelete,
  });

  final AppStrings strings;
  final ManagedUser user;
  final bool busy;
  final Future<void> Function(ManagedUser user, bool value) onToggle;
  final Future<void> Function(ManagedUser user) onChangePassword;
  final Future<void> Function(ManagedUser user) onDelete;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final statusColor = user.enabled
        ? theme.colorScheme.primary
        : theme.colorScheme.onSurfaceVariant;

    return Container(
      padding: const EdgeInsets.fromLTRB(24, 22, 14, 20),
      decoration: BoxDecoration(
        color: theme.colorScheme.surface,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: theme.colorScheme.outlineVariant),
        boxShadow: [
          BoxShadow(
            color: theme.colorScheme.shadow.withValues(alpha: 0.04),
            blurRadius: 18,
            offset: const Offset(0, 6),
          ),
        ],
      ),
      child: Column(
        children: [
          Row(
            children: [
              _UserAvatar(username: user.username, enabled: user.enabled),
              const SizedBox(width: 14),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      user.username,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: theme.textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    const SizedBox(height: 7),
                    _StatusPill(
                      label: user.enabled ? strings.enabled : strings.disabled,
                      color: statusColor,
                    ),
                  ],
                ),
              ),
              Switch(
                value: user.enabled,
                onChanged: busy ? null : (value) => onToggle(user, value),
              ),
              PopupMenuButton<String>(
                enabled: !busy,
                tooltip: strings.userActions,
                onSelected: (value) {
                  if (value == 'password') onChangePassword(user);
                  if (value == 'delete') onDelete(user);
                },
                itemBuilder: (_) => [
                  PopupMenuItem(
                    value: 'password',
                    child: Text(strings.changePassword),
                  ),
                  PopupMenuItem(value: 'delete', child: Text(strings.delete)),
                ],
              ),
            ],
          ),
          const SizedBox(height: 20),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.fromLTRB(8, 16, 8, 2),
            decoration: BoxDecoration(
              border: Border(
                top: BorderSide(color: theme.colorScheme.outlineVariant),
              ),
            ),
            child: Wrap(
              spacing: 24,
              runSpacing: 8,
              children: [
                _UserDetail(
                  icon: Icons.badge_outlined,
                  label: strings.accountId,
                  value: user.id,
                ),
                _UserDetail(
                  icon: Icons.folder_shared_outlined,
                  label: strings.accountType,
                  value: strings.webdavAccount,
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _UserAvatar extends StatelessWidget {
  const _UserAvatar({required this.username, required this.enabled});

  final String username;
  final bool enabled;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return CircleAvatar(
      radius: 30,
      backgroundColor: enabled
          ? scheme.primaryContainer
          : scheme.surfaceContainerHighest,
      child: Text(
        username.isEmpty ? '?' : username.substring(0, 1).toUpperCase(),
        style: TextStyle(
          color: enabled ? scheme.onPrimaryContainer : scheme.onSurfaceVariant,
          fontWeight: FontWeight.w700,
        ),
      ),
    );
  }
}

class _StatusPill extends StatelessWidget {
  const _StatusPill({required this.label, required this.color});

  final String label;
  final Color color;

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
    decoration: BoxDecoration(
      color: color.withValues(alpha: 0.11),
      borderRadius: BorderRadius.circular(99),
    ),
    child: Text(
      label,
      style: TextStyle(color: color, fontSize: 12, fontWeight: FontWeight.w700),
    ),
  );
}

class _UserDetail extends StatelessWidget {
  const _UserDetail({
    required this.icon,
    required this.label,
    required this.value,
  });

  final IconData icon;
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(icon, size: 17, color: theme.colorScheme.onSurfaceVariant),
        const SizedBox(width: 7),
        Text(
          '$label  ',
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
          ),
        ),
        Text(
          value,
          style: theme.textTheme.bodySmall?.copyWith(
            fontWeight: FontWeight.w600,
          ),
        ),
      ],
    );
  }
}

class _EmptyUsers extends StatelessWidget {
  const _EmptyUsers({required this.strings, required this.filtered});

  final AppStrings strings;
  final bool filtered;

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 44),
    decoration: BoxDecoration(
      color: Theme.of(context).colorScheme.surface,
      borderRadius: BorderRadius.circular(18),
      border: Border.all(color: Theme.of(context).colorScheme.outlineVariant),
    ),
    child: Column(
      children: [
        Icon(
          filtered ? Icons.search_off : Icons.people_outline,
          size: 40,
          color: Theme.of(context).colorScheme.onSurfaceVariant,
        ),
        const SizedBox(height: 12),
        Text(filtered ? strings.noMatchingUsers : strings.noUsers),
      ],
    ),
  );
}

class _ErrorView extends StatelessWidget {
  const _ErrorView({required this.controller, required this.strings});

  final UsersController controller;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) => Center(
    child: Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(
          Icons.cloud_off_outlined,
          size: 40,
          color: Theme.of(context).colorScheme.onSurfaceVariant,
        ),
        const SizedBox(height: 12),
        Text(strings.usersUnavailable),
        const SizedBox(height: 12),
        FilledButton(onPressed: controller.refresh, child: Text(strings.retry)),
      ],
    ),
  );
}
