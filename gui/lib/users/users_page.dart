import 'dart:convert';

import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/state/users_controller.dart';
import 'package:flutter/material.dart';

class UsersPage extends StatelessWidget {
  const UsersPage({super.key, required this.controller});
  final UsersController controller;

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return Scaffold(
      appBar: AppBar(title: Text(strings.users)),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: controller.busy ? null : () => _showCreate(context),
        icon: const Icon(Icons.person_add),
        label: Text(strings.addUser),
      ),
      body: AnimatedBuilder(
        animation: controller,
        builder: (context, _) {
          if (controller.loading) {
            return const Center(child: CircularProgressIndicator());
          }
          if (controller.loadError != null && controller.users.isEmpty) {
            return _ErrorView(controller: controller, strings: strings);
          }
          if (controller.users.isEmpty) {
            return Center(child: Text(strings.noUsers));
          }
          return Stack(
            children: [
              ListView.separated(
                padding: const EdgeInsets.all(16),
                itemCount: controller.users.length,
                separatorBuilder: (_, _) => const Divider(),
                itemBuilder: (context, index) {
                  final user = controller.users[index];
                  return ListTile(
                    leading: Icon(
                      user.enabled ? Icons.person : Icons.person_off,
                    ),
                    title: Text(user.username),
                    subtitle: Text(
                      user.enabled ? strings.enabled : strings.disabled,
                    ),
                    trailing: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Switch(
                          value: user.enabled,
                          onChanged: controller.busy
                              ? null
                              : (value) => controller.setEnabled(user, value),
                        ),
                        PopupMenuButton<String>(
                          onSelected: (value) {
                            if (value == 'password') {
                              _showPassword(context, user);
                            }
                            if (value == 'delete') {
                              _confirmDelete(context, user);
                            }
                          },
                          itemBuilder: (_) => [
                            PopupMenuItem(
                              value: 'password',
                              child: Text(strings.changePassword),
                            ),
                            PopupMenuItem(
                              value: 'delete',
                              child: Text(strings.delete),
                            ),
                          ],
                        ),
                      ],
                    ),
                  );
                },
              ),
              if (controller.busy) const LinearProgressIndicator(),
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

class _ErrorView extends StatelessWidget {
  const _ErrorView({required this.controller, required this.strings});
  final UsersController controller;
  final AppStrings strings;
  @override
  Widget build(BuildContext context) => Center(
    child: Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(strings.usersUnavailable),
        const SizedBox(height: 12),
        FilledButton(onPressed: controller.refresh, child: Text(strings.retry)),
      ],
    ),
  );
}
