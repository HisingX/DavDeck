import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/state/status_controller.dart';
import 'package:flutter/material.dart';

class DashboardPage extends StatelessWidget {
  const DashboardPage({
    super.key,
    required this.controller,
    this.onOpenService,
  });

  final StatusController controller;
  final VoidCallback? onOpenService;

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return Scaffold(
      appBar: AppBar(title: Text('DavDeck · ${strings.dashboard}')),
      body: AnimatedBuilder(
        animation: controller,
        builder: (context, _) => switch (controller.state) {
          LoadState.loading => Center(
            child: Semantics(
              label: strings.loading,
              child: const CircularProgressIndicator(),
            ),
          ),
          LoadState.error => Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(strings.unavailable),
                const SizedBox(height: 12),
                FilledButton(
                  onPressed: controller.refresh,
                  child: Text(strings.retry),
                ),
              ],
            ),
          ),
          LoadState.ready => _StatusCard(
            controller: controller,
            strings: strings,
            onOpenService: onOpenService,
          ),
        },
      ),
    );
  }
}

class _StatusCard extends StatelessWidget {
  const _StatusCard({
    required this.controller,
    required this.strings,
    this.onOpenService,
  });

  final StatusController controller;
  final AppStrings strings;
  final VoidCallback? onOpenService;

  @override
  Widget build(BuildContext context) {
    final status = controller.status!;
    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: Center(
        child: Card(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  status.name,
                  style: Theme.of(context).textTheme.headlineSmall,
                ),
                const SizedBox(height: 16),
                Text('${strings.daemon}: ${status.daemon}'),
                Text('${strings.database}: ${status.database}'),
                Text('${strings.schema}: ${status.schemaVersion}'),
                Text('${strings.version}: ${status.version}'),
                Text('Caddy: ${status.caddy}'),
                Text('WebDAV: ${status.webdav}'),
                Text(
                  'Service: ${status.service.state} · installed: ${status.service.installed} · starts at boot: ${status.service.startsAtBoot}',
                ),
                if (onOpenService != null)
                  TextButton.icon(
                    onPressed: onOpenService,
                    icon: const Icon(Icons.miscellaneous_services_outlined),
                    label: Text(strings.openService),
                  ),
                if (status.pendingChanges) Text(strings.pendingConfiguration),
                if (status.pendingChanges && controller.configuration != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 8),
                    child: FilledButton.tonalIcon(
                      onPressed: controller.busy
                          ? null
                          : controller.applyPending,
                      icon: const Icon(Icons.rocket_launch_outlined),
                      label: Text(strings.applyConfiguration),
                    ),
                  ),
                if (controller.applyResult case final revision?)
                  Text(strings.configurationAppliedRevision(revision.number)),
                if (status.lastErrorCode != null)
                  Text('Last error: ${status.lastErrorCode}'),
                if (status.portableDaemonOwned)
                  const Text('Portable daemon: owned by GUI'),
                if (controller.runtime != null)
                  Text('Caddy: ${controller.runtime!.caddy}'),
                if (controller.actionError != null) ...[
                  const SizedBox(height: 8),
                  Text(
                    '${strings.caddyActionFailed}\n${controller.actionError}',
                    style: TextStyle(
                      color: Theme.of(context).colorScheme.error,
                    ),
                  ),
                ],
                if (controller.serverSettings != null) ...[
                  const SizedBox(height: 8),
                  Text(
                    '${strings.ports}: HTTP ${controller.serverSettings!.httpPort} · HTTPS ${controller.serverSettings!.httpsPort}',
                  ),
                  TextButton(
                    onPressed: controller.busy
                        ? null
                        : () => _editPorts(context, controller, strings),
                    child: Text(strings.editPorts),
                  ),
                ],
                if (controller.server != null) ...[
                  const SizedBox(height: 16),
                  Wrap(
                    spacing: 8,
                    children: [
                      FilledButton(
                        onPressed: controller.busy
                            ? null
                            : () => controller.control(
                                controller.server!.startServer,
                              ),
                        child: const Text('Start'),
                      ),
                      OutlinedButton(
                        onPressed: controller.busy
                            ? null
                            : () => controller.control(
                                controller.server!.stopServer,
                              ),
                        child: const Text('Stop'),
                      ),
                      OutlinedButton(
                        onPressed: controller.busy
                            ? null
                            : () => controller.control(
                                controller.server!.restartServer,
                              ),
                        child: const Text('Restart'),
                      ),
                    ],
                  ),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }
}

Future<void> _editPorts(
  BuildContext context,
  StatusController controller,
  AppStrings strings,
) async {
  final settings = controller.serverSettings;
  if (settings == null) return;
  final http = TextEditingController(text: settings.httpPort.toString());
  final https = TextEditingController(text: settings.httpsPort.toString());
  String? error;
  await showDialog<void>(
    context: context,
    builder: (dialogContext) => StatefulBuilder(
      builder: (context, setState) => AlertDialog(
        title: Text(strings.editPorts),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
              controller: http,
              keyboardType: TextInputType.number,
              decoration: InputDecoration(labelText: strings.httpPort),
            ),
            TextField(
              controller: https,
              keyboardType: TextInputType.number,
              decoration: InputDecoration(labelText: strings.httpsPort),
            ),
            if (error != null)
              Padding(
                padding: const EdgeInsets.only(top: 12),
                child: Text(
                  error!,
                  style: TextStyle(color: Theme.of(context).colorScheme.error),
                ),
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
              final httpPort = int.tryParse(http.text);
              final httpsPort = int.tryParse(https.text);
              if (httpPort == null ||
                  httpsPort == null ||
                  httpPort < 1 ||
                  httpPort > 65535 ||
                  httpsPort < 1 ||
                  httpsPort > 65535) {
                setState(() => error = strings.portRequired);
                return;
              }
              if (httpPort == httpsPort) {
                setState(() => error = strings.portsMustDiffer);
                return;
              }
              try {
                await controller.updatePorts(httpPort, httpsPort);
                if (dialogContext.mounted) Navigator.pop(dialogContext);
              } catch (_) {
                setState(() => error = strings.savePortsFailed);
              }
            },
            child: Text(strings.savePorts),
          ),
        ],
      ),
    ),
  );
  http.dispose();
  https.dispose();
}
