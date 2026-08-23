import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/state/service_controller.dart';
import 'package:davdeck/state/status_controller.dart';
import 'package:flutter/material.dart';

class ServicePage extends StatelessWidget {
  const ServicePage({
    super.key,
    required this.status,
    required this.controller,
    this.onOpenDiagnostics,
  });

  final StatusController status;
  final ServiceController controller;
  final VoidCallback? onOpenDiagnostics;

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return Scaffold(
      appBar: AppBar(
        title: Text(strings.serviceManagement),
        actions: [
          IconButton(
            onPressed: controller.busy ? null : controller.refresh,
            tooltip: strings.refreshService,
            icon: const Icon(Icons.refresh),
          ),
          const SizedBox(width: 8),
        ],
      ),
      body: AnimatedBuilder(
        animation: Listenable.merge([status, controller]),
        builder: (context, _) {
          if (controller.state == ServiceLoadState.loading &&
              controller.service == null) {
            return Center(child: Text(strings.serviceLoading));
          }
          if (controller.state == ServiceLoadState.error &&
              controller.service == null) {
            return Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(strings.serviceUnavailable),
                  const SizedBox(height: 12),
                  FilledButton(
                    onPressed: controller.refresh,
                    child: Text(strings.retry),
                  ),
                ],
              ),
            );
          }
          return _ServiceContent(
            status: status,
            controller: controller,
            onOpenDiagnostics: onOpenDiagnostics,
            strings: strings,
          );
        },
      ),
    );
  }
}

class _ServiceContent extends StatelessWidget {
  const _ServiceContent({
    required this.status,
    required this.controller,
    this.onOpenDiagnostics,
    required this.strings,
  });

  final StatusController status;
  final ServiceController controller;
  final VoidCallback? onOpenDiagnostics;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) {
    final service = controller.service;
    if (service == null) return Center(child: Text(strings.serviceUnavailable));
    final daemon = status.status;
    final running = service.state == 'RUNNING' || service.state == 'STARTING';
    final errorCode = _errorCode(controller.actionError);
    return Stack(
      children: [
        ListView(
          padding: const EdgeInsets.all(24),
          children: [
            Card(
              child: Padding(
                padding: const EdgeInsets.all(20),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      strings.serviceManagement,
                      style: Theme.of(context).textTheme.headlineSmall,
                    ),
                    const SizedBox(height: 16),
                    _StateLine(
                      label: strings.daemonState,
                      value: daemon?.daemon ?? 'UNKNOWN',
                    ),
                    _StateLine(
                      label: strings.caddyState,
                      value:
                          daemon?.caddy ?? status.runtime?.caddy ?? 'UNKNOWN',
                    ),
                    _StateLine(
                      label: strings.webdavState,
                      value:
                          daemon?.webdav ?? status.runtime?.webdav ?? 'UNKNOWN',
                    ),
                    _StateLine(
                      label: strings.serviceState,
                      value: service.state,
                    ),
                    _StateLine(
                      label: strings.serviceInstalled,
                      value: service.installed
                          ? strings.serviceInstalled
                          : strings.serviceNotInstalled,
                    ),
                    _StateLine(
                      label: strings.startsAtBoot,
                      value: service.startsAtBoot ? 'Yes' : 'No',
                    ),
                    if (service.lastErrorCode != null)
                      _StateLine(
                        label: 'Last error',
                        value: service.lastErrorCode!,
                      ),
                  ],
                ),
              ),
            ),
            if (daemon?.portableDaemonOwned == true) ...[
              const SizedBox(height: 12),
              Card(
                child: ListTile(
                  leading: const Icon(Icons.portable_wifi_off),
                  title: Text(strings.portableDaemonNote),
                ),
              ),
            ],
            if (controller.actionError != null) ...[
              const SizedBox(height: 12),
              Text(
                errorCode.isEmpty
                    ? strings.serviceActionFailed
                    : '${strings.serviceActionFailed}\n$errorCode',
                style: TextStyle(color: Theme.of(context).colorScheme.error),
              ),
              if (onOpenDiagnostics != null)
                Align(
                  alignment: Alignment.centerLeft,
                  child: TextButton.icon(
                    onPressed: onOpenDiagnostics,
                    icon: const Icon(Icons.health_and_safety_outlined),
                    label: Text(strings.openDiagnostics),
                  ),
                ),
            ],
            const SizedBox(height: 20),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                if (!service.installed)
                  FilledButton.icon(
                    onPressed: controller.busy
                        ? null
                        : () => _confirm(
                            context,
                            strings.installService,
                            controller.install,
                          ),
                    icon: const Icon(Icons.add_business_outlined),
                    label: Text(strings.installService),
                  )
                else ...[
                  FilledButton.icon(
                    onPressed: controller.busy
                        ? null
                        : () => running
                              ? _confirm(
                                  context,
                                  strings.stopService,
                                  controller.stop,
                                )
                              : controller.start(),
                    icon: Icon(running ? Icons.stop : Icons.play_arrow),
                    label: Text(
                      running ? strings.stopService : strings.startService,
                    ),
                  ),
                  OutlinedButton.icon(
                    onPressed: controller.busy
                        ? null
                        : () => _confirm(
                            context,
                            strings.uninstallService,
                            controller.uninstall,
                          ),
                    icon: const Icon(Icons.delete_outline),
                    label: Text(strings.uninstallService),
                  ),
                ],
              ],
            ),
          ],
        ),
        if (controller.busy) const LinearProgressIndicator(),
      ],
    );
  }

  Future<void> _confirm(
    BuildContext context,
    String action,
    Future<bool> Function() operation,
  ) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(action),
        content: Text(strings.confirmServiceAction(action)),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: Text(strings.cancel),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: Text(action),
          ),
        ],
      ),
    );
    if (confirmed == true) await operation();
  }
}

class _StateLine extends StatelessWidget {
  const _StateLine({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.symmetric(vertical: 4),
    child: Row(
      children: [
        Expanded(child: Text(label)),
        Text(value, style: const TextStyle(fontWeight: FontWeight.w600)),
      ],
    ),
  );
}

String _errorCode(Object? error) =>
    error is DaemonApiException ? error.code : '';
