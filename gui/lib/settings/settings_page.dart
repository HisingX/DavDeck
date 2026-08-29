import 'dart:io';

import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/state/backup_controller.dart';
import 'package:davdeck/widgets/app_ui.dart';
import 'package:file_selector/file_selector.dart';
import 'package:flutter/material.dart';

typedef ConfigurationPathPicker = Future<String?> Function();
typedef ConfigurationContentReader = Future<String?> Function();
typedef ConfigurationContentWriter =
    Future<void> Function(String path, String content);

final _yamlTypeGroup = XTypeGroup(label: 'YAML', extensions: ['yaml', 'yml']);

Future<String?> pickConfigurationSavePath() async => (await getSaveLocation(
  suggestedName: 'davdeck-backup.yaml',
  acceptedTypeGroups: [_yamlTypeGroup],
))?.path;

Future<String?> readConfigurationFile() async {
  final file = await openFile(acceptedTypeGroups: [_yamlTypeGroup]);
  return file?.readAsString();
}

Future<void> writeConfigurationFile(String path, String content) =>
    File(path).writeAsString(content, flush: true);

class SettingsPage extends StatelessWidget {
  const SettingsPage({
    super.key,
    required this.controller,
    this.pickSavePath = pickConfigurationSavePath,
    this.readFile = readConfigurationFile,
    this.writeFile = writeConfigurationFile,
    this.onConfigurationImported,
  });

  final BackupController? controller;
  final ConfigurationPathPicker pickSavePath;
  final ConfigurationContentReader readFile;
  final ConfigurationContentWriter writeFile;
  final Future<void> Function()? onConfigurationImported;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: controller == null
          ? _SettingsContent(
              controller: controller,
              pickSavePath: pickSavePath,
              readFile: readFile,
              writeFile: writeFile,
              onConfigurationImported: onConfigurationImported,
            )
          : AnimatedBuilder(
              animation: controller!,
              builder: (context, child) => _SettingsContent(
                controller: controller,
                pickSavePath: pickSavePath,
                readFile: readFile,
                writeFile: writeFile,
                onConfigurationImported: onConfigurationImported,
              ),
            ),
    );
  }
}

class _SettingsContent extends StatelessWidget {
  const _SettingsContent({
    required this.controller,
    required this.pickSavePath,
    required this.readFile,
    required this.writeFile,
    required this.onConfigurationImported,
  });

  final BackupController? controller;
  final ConfigurationPathPicker pickSavePath;
  final ConfigurationContentReader readFile;
  final ConfigurationContentWriter writeFile;
  final Future<void> Function()? onConfigurationImported;

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return SingleChildScrollView(
      padding: appPagePadding(context),
      child: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 980),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              AppPageHeader(
                title: strings.settings,
                subtitle: strings.settingsSubtitle,
              ),
              const SizedBox(height: 28),
              AppNotice(
                icon: Icons.shield_outlined,
                text:
                    '${strings.dataSafetyDescription}\n\n${strings.backupRecommendation}',
                color: Theme.of(context).colorScheme.primaryContainer,
                textColor: Theme.of(context).colorScheme.onPrimaryContainer,
              ),
              const SizedBox(height: 20),
              AppSurface(
                padding: const EdgeInsets.all(24),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Icon(
                          Icons.save_outlined,
                          color: Theme.of(context).colorScheme.primary,
                        ),
                        const SizedBox(width: 14),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                strings.backupAndRestore,
                                style: Theme.of(context).textTheme.titleLarge
                                    ?.copyWith(fontWeight: FontWeight.w700),
                              ),
                              const SizedBox(height: 5),
                              Text(
                                strings.backupAndRestoreSubtitle,
                                style: Theme.of(context).textTheme.bodyLarge
                                    ?.copyWith(
                                      color: Theme.of(
                                        context,
                                      ).colorScheme.onSurfaceVariant,
                                    ),
                              ),
                            ],
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 18),
                    Text(
                      strings.backupContents,
                      style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                        color: Theme.of(context).colorScheme.onSurfaceVariant,
                      ),
                    ),
                    const SizedBox(height: 22),
                    LayoutBuilder(
                      builder: (context, constraints) {
                        final buttons = [
                          FilledButton.icon(
                            onPressed:
                                controller == null ||
                                    controller!.exporting ||
                                    controller!.importing
                                ? null
                                : () => _export(context),
                            icon: controller?.exporting == true
                                ? const SizedBox(
                                    width: 18,
                                    height: 18,
                                    child: CircularProgressIndicator(
                                      strokeWidth: 2,
                                    ),
                                  )
                                : const Icon(Icons.file_download_outlined),
                            label: Text(
                              controller?.exporting == true
                                  ? strings.exportingConfiguration
                                  : strings.exportConfigurationBackup,
                            ),
                          ),
                          OutlinedButton.icon(
                            onPressed:
                                controller == null ||
                                    controller!.exporting ||
                                    controller!.importing
                                ? null
                                : () => _import(context),
                            icon: controller?.importing == true
                                ? const SizedBox(
                                    width: 18,
                                    height: 18,
                                    child: CircularProgressIndicator(
                                      strokeWidth: 2,
                                    ),
                                  )
                                : const Icon(Icons.file_upload_outlined),
                            label: Text(
                              controller?.importing == true
                                  ? strings.importingConfiguration
                                  : strings.importConfigurationBackup,
                            ),
                          ),
                        ];
                        if (constraints.maxWidth < 560) {
                          return Column(
                            crossAxisAlignment: CrossAxisAlignment.stretch,
                            children: [
                              buttons[0],
                              const SizedBox(height: 12),
                              buttons[1],
                            ],
                          );
                        }
                        return Row(
                          children: [
                            Expanded(child: buttons[0]),
                            const SizedBox(width: 12),
                            Expanded(child: buttons[1]),
                          ],
                        );
                      },
                    ),
                    if (controller == null) ...[
                      const SizedBox(height: 16),
                      AppNotice(
                        icon: Icons.info_outline,
                        text: strings.backupUnavailable,
                      ),
                    ],
                    if (controller?.error != null) ...[
                      const SizedBox(height: 16),
                      AppNotice(
                        icon: Icons.error_outline,
                        text: controller!.error.toString(),
                        color: Theme.of(context).colorScheme.errorContainer,
                        textColor: Theme.of(
                          context,
                        ).colorScheme.onErrorContainer,
                      ),
                    ],
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Future<void> _export(BuildContext context) async {
    final strings = AppStrings.of(context);
    final backup = controller;
    if (backup == null) return;
    final content = await backup.exportConfiguration();
    if (!context.mounted || content == null) return;
    try {
      final path = await pickSavePath();
      if (!context.mounted || path == null || path.trim().isEmpty) return;
      await writeFile(path, content);
      if (!context.mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(strings.configurationExportedTo(path))),
      );
    } catch (_) {
      if (!context.mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(strings.configurationExportFailed)),
      );
    }
  }

  Future<void> _import(BuildContext context) async {
    final strings = AppStrings.of(context);
    final backup = controller;
    if (backup == null) return;
    String? content;
    try {
      content = await readFile();
    } catch (_) {
      if (!context.mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(strings.configurationImportFailed)),
      );
      return;
    }
    if (!context.mounted || content == null) return;
    final confirmed = await showAppDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(strings.confirmConfigurationImport),
        content: Text(strings.confirmConfigurationImportDescription),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: Text(strings.cancel),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: Text(strings.importConfigurationBackup),
          ),
        ],
      ),
    );
    if (!context.mounted || confirmed != true) return;
    final imported = await backup.importConfiguration(content);
    if (!context.mounted) return;
    if (!imported || backup.lastImport == null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(strings.configurationImportFailed)),
      );
      return;
    }
    await onConfigurationImported?.call();
    if (!context.mounted) return;
    await _showImportResult(context, backup.lastImport!, strings);
  }

  Future<void> _showImportResult(
    BuildContext context,
    ManagedConfigImportResult result,
    AppStrings strings,
  ) => showAppDialog<void>(
    context: context,
    builder: (dialogContext) => AlertDialog(
      title: Text(strings.configurationImportComplete),
      content: SingleChildScrollView(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              strings.configurationImportCounts(
                users: result.usersCreated + result.usersUpdated,
                shares: result.sharesCreated + result.sharesUpdated,
                permissions: result.permissionsUpserted,
              ),
            ),
            const SizedBox(height: 12),
            Text(
              result.passwordResetRequired.isEmpty
                  ? strings.configurationImportNoPasswordReset
                  : strings.configurationImportPasswordReset(
                      result.passwordResetRequired.join(', '),
                    ),
            ),
            if (result.pendingApply) ...[
              const SizedBox(height: 12),
              Text(strings.configurationImportPendingApply),
            ],
          ],
        ),
      ),
      actions: [
        FilledButton(
          onPressed: () => Navigator.pop(dialogContext),
          child: Text(strings.close),
        ),
      ],
    ),
  );
}
