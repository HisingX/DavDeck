import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/state/shares_controller.dart';
import 'package:flutter/material.dart';

class SharesPage extends StatelessWidget {
  const SharesPage({super.key, required this.controller});
  final SharesController controller;
  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return Scaffold(
      appBar: AppBar(title: Text(strings.shares)),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: controller.busy ? null : () => _editShare(context),
        icon: const Icon(Icons.create_new_folder),
        label: Text(strings.addShare),
      ),
      body: AnimatedBuilder(
        animation: controller,
        builder: (context, _) {
          if (controller.loading) {
            return const Center(child: CircularProgressIndicator());
          }
          if (controller.error != null && controller.shares.isEmpty) {
            return Center(
              child: FilledButton(
                onPressed: controller.refresh,
                child: Text(strings.retry),
              ),
            );
          }
          if (controller.shares.isEmpty) {
            return Center(child: Text(strings.noShares));
          }
          return Stack(
            children: [
              ListView.separated(
                padding: const EdgeInsets.all(16),
                itemCount: controller.shares.length,
                separatorBuilder: (_, _) => const Divider(),
                itemBuilder: (context, index) {
                  final share = controller.shares[index];
                  return ListTile(
                    leading: Icon(
                      share.enabled ? Icons.folder_shared : Icons.folder_off,
                    ),
                    title: Text(share.name),
                    subtitle: Text('/dav/${share.slug}/\n${share.path}'),
                    isThreeLine: true,
                    trailing: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Switch(
                          value: share.enabled,
                          onChanged: controller.busy
                              ? null
                              : (value) =>
                                    controller.update(share, enabled: value),
                        ),
                        PopupMenuButton<String>(
                          onSelected: (value) {
                            if (value == 'permissions') {
                              _showPermissions(context, share);
                            } else if (value == 'edit') {
                              _editShare(context, share);
                            } else if (value == 'delete') {
                              _confirmDelete(context, share);
                            }
                          },
                          itemBuilder: (_) => [
                            PopupMenuItem(
                              value: 'permissions',
                              child: Text(strings.permissions),
                            ),
                            PopupMenuItem(
                              value: 'edit',
                              child: Text(strings.edit),
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

  Future<void> _editShare(BuildContext context, [ManagedShare? share]) async {
    final strings = AppStrings.of(context);
    final name = TextEditingController(text: share?.name);
    final slug = TextEditingController(text: share?.slug);
    final path = TextEditingController(text: share?.path);
    try {
      await showDialog<void>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: Text(share == null ? strings.addShare : strings.editShare),
          content: Column(
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
    await showDialog<void>(
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
                                  if (value == null) {
                                    return;
                                  }
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
    final confirmed = await showDialog<bool>(
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
