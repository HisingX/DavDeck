import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/state/service_controller.dart';
import 'package:davdeck/state/status_controller.dart';
import 'package:davdeck/widgets/app_ui.dart';
import 'package:flutter/material.dart';

class ServicePage extends StatelessWidget {
  const ServicePage({
    super.key,
    required this.status,
    required this.controller,
    this.onOpenDiagnostics,
    this.onOpenLogs,
  });

  final StatusController status;
  final ServiceController controller;
  final VoidCallback? onOpenDiagnostics;
  final VoidCallback? onOpenLogs;

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return Scaffold(
      body: AnimatedBuilder(
        animation: Listenable.merge([status, controller]),
        builder: (context, _) {
          if (controller.state == ServiceLoadState.loading &&
              controller.service == null) {
            return Center(child: Text(strings.serviceLoading));
          }
          if (controller.state == ServiceLoadState.error &&
              controller.service == null) {
            return _ErrorState(strings: strings, onRetry: controller.refresh);
          }
          return _ServiceContent(
            status: status,
            controller: controller,
            onOpenDiagnostics: onOpenDiagnostics,
            onOpenLogs: onOpenLogs,
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
    this.onOpenLogs,
    required this.strings,
  });

  final StatusController status;
  final ServiceController controller;
  final VoidCallback? onOpenDiagnostics;
  final VoidCallback? onOpenLogs;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) {
    final service = controller.service;
    if (service == null) return Center(child: Text(strings.serviceUnavailable));
    final daemon = status.status;
    final daemonState = daemon?.daemon ?? 'UNKNOWN';
    final caddyState = daemon?.caddy ?? status.runtime?.caddy ?? 'UNKNOWN';
    final webdavState = daemon?.webdav ?? status.runtime?.webdav ?? 'UNKNOWN';
    final running = service.state == 'RUNNING' || service.state == 'STARTING';
    final errorCode = _errorCode(controller.actionError);
    final theme = Theme.of(context);
    final compactHeight = MediaQuery.sizeOf(context).height < 700;

    return Stack(
      children: [
        SingleChildScrollView(
          padding: appPagePadding(context),
          child: Center(
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 1120),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  AppPageHeader(
                    title: strings.service,
                    subtitle: strings.serviceSubtitle,
                    actions: Wrap(
                      spacing: 8,
                      children: [
                        IconButton(
                          onPressed: controller.busy
                              ? null
                              : controller.refresh,
                          tooltip: strings.refreshService,
                          icon: const Icon(Icons.refresh),
                        ),
                        if (compactHeight)
                          FilledButton.icon(
                            onPressed: controller.busy
                                ? null
                                : service.installed && running
                                ? () => _confirm(
                                    context,
                                    strings.stopService,
                                    controller.stop,
                                  )
                                : service.installed
                                ? controller.start
                                : () => _confirm(
                                    context,
                                    strings.installService,
                                    controller.install,
                                  ),
                            icon: Icon(
                              service.installed && running
                                  ? Icons.stop_outlined
                                  : service.installed
                                  ? Icons.play_arrow_outlined
                                  : Icons.add_business_outlined,
                            ),
                            label: Text(
                              service.installed
                                  ? running
                                        ? strings.stopService
                                        : strings.startService
                                  : strings.installService,
                            ),
                          ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 24),
                  if (errorCode.isNotEmpty ||
                      controller.actionError != null) ...[
                    AppNotice(
                      icon: Icons.error_outline,
                      text: errorCode.isEmpty
                          ? strings.serviceActionFailed
                          : '${strings.serviceActionFailed}\n$errorCode',
                      color: theme.colorScheme.errorContainer,
                      textColor: theme.colorScheme.onErrorContainer,
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
                    const SizedBox(height: 16),
                  ],
                  _ServiceStatusGrid(
                    strings: strings,
                    values: [
                      _ServiceStatusData(
                        title: strings.daemonState,
                        value: daemonState,
                        detail: strings.daemon,
                        icon: Icons.shield_outlined,
                      ),
                      _ServiceStatusData(
                        title: strings.caddyState,
                        value: caddyState,
                        detail: strings.caddyDetail,
                        icon: Icons.language,
                      ),
                      _ServiceStatusData(
                        title: strings.webdavState,
                        value: webdavState,
                        detail: strings.webdavDetail,
                        icon: Icons.public,
                      ),
                      _ServiceStatusData(
                        title: strings.service,
                        value: service.installed
                            ? strings.serviceInstalled
                            : strings.serviceNotInstalled,
                        rawValue: service.installed
                            ? 'INSTALLED'
                            : 'NOT_INSTALLED',
                        detail: strings.startsAtBoot,
                        icon: Icons.settings_outlined,
                      ),
                    ],
                  ),
                  const SizedBox(height: 24),
                  AppSurface(
                    padding: const EdgeInsets.fromLTRB(22, 20, 22, 22),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        Text(
                          strings.serviceControl,
                          style: theme.textTheme.titleLarge?.copyWith(
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                        const SizedBox(height: 5),
                        Text(
                          strings.serviceControlSubtitle,
                          style: theme.textTheme.bodyMedium?.copyWith(
                            color: theme.colorScheme.onSurfaceVariant,
                          ),
                        ),
                        const SizedBox(height: 18),
                        LayoutBuilder(
                          builder: (context, constraints) {
                            final columns = constraints.maxWidth >= 820
                                ? 4
                                : constraints.maxWidth >= 520
                                ? 2
                                : 1;
                            final width =
                                (constraints.maxWidth - (columns - 1) * 10) /
                                columns;
                            return Wrap(
                              spacing: 10,
                              runSpacing: 10,
                              children: [
                                if (service.installed && !compactHeight)
                                  SizedBox(
                                    width: width,
                                    child: _ActionButton(
                                      icon: running
                                          ? Icons.stop_outlined
                                          : Icons.play_arrow_outlined,
                                      label: running
                                          ? strings.stopService
                                          : strings.startService,
                                      detail: running
                                          ? strings.stopServiceDescription
                                          : strings.startServiceDescription,
                                      color: running
                                          ? theme.colorScheme.error
                                          : theme.colorScheme.primary,
                                      onPressed: controller.busy
                                          ? null
                                          : () => running
                                                ? _confirm(
                                                    context,
                                                    strings.stopService,
                                                    controller.stop,
                                                  )
                                                : controller.start(),
                                    ),
                                  ),
                                if (service.installed && !compactHeight)
                                  SizedBox(
                                    width: width,
                                    child: _ActionButton(
                                      icon: Icons.restart_alt,
                                      label: strings.restartService,
                                      detail: strings.restartServiceDescription,
                                      color: const Color(0xffb87800),
                                      onPressed: controller.busy
                                          ? null
                                          : () => _confirmRestart(context),
                                    ),
                                  ),
                                if (!service.installed && !compactHeight)
                                  SizedBox(
                                    width: width,
                                    child: _ActionButton(
                                      icon: Icons.add_business_outlined,
                                      label: strings.installService,
                                      detail: strings.installServiceDescription,
                                      color: theme.colorScheme.primary,
                                      onPressed: controller.busy
                                          ? null
                                          : () => _confirm(
                                              context,
                                              strings.installService,
                                              controller.install,
                                            ),
                                    ),
                                  ),
                                if (onOpenLogs != null)
                                  SizedBox(
                                    width: width,
                                    child: _ActionButton(
                                      icon: Icons.receipt_long_outlined,
                                      label: strings.openLogs,
                                      detail: strings.openLogsDescription,
                                      color: theme.colorScheme.onSurfaceVariant,
                                      onPressed: onOpenLogs,
                                    ),
                                  ),
                              ],
                            );
                          },
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 24),
                  LayoutBuilder(
                    builder: (context, constraints) {
                      final compact = constraints.maxWidth < 760;
                      final servicePanel = _SystemServicePanel(
                        service: service,
                        controller: controller,
                        strings: strings,
                        onConfirm: _confirm,
                      );
                      final explanation = _ServiceExplanation(strings: strings);
                      return compact
                          ? Column(
                              children: [
                                servicePanel,
                                const SizedBox(height: 18),
                                explanation,
                              ],
                            )
                          : Row(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Expanded(child: servicePanel),
                                const SizedBox(width: 18),
                                Expanded(child: explanation),
                              ],
                            );
                    },
                  ),
                  if (service.lastErrorCode != null) ...[
                    const SizedBox(height: 14),
                    Text(
                      '${strings.lastError}: ${service.lastErrorCode}',
                      style: TextStyle(color: theme.colorScheme.error),
                    ),
                  ],
                ],
              ),
            ),
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
  }

  Future<void> _confirmRestart(BuildContext context) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(strings.restartService),
        content: Text(strings.confirmServiceAction(strings.restartService)),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: Text(strings.cancel),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: Text(strings.restartService),
          ),
        ],
      ),
    );
    if (confirmed == true) {
      await controller.stop();
      if (controller.actionError == null) await controller.start();
    }
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

class _ServiceStatusData {
  const _ServiceStatusData({
    required this.title,
    required this.value,
    required this.detail,
    required this.icon,
    this.rawValue,
  });

  final String title;
  final String value;
  final String? rawValue;
  final String detail;
  final IconData icon;
}

class _ServiceStatusGrid extends StatelessWidget {
  const _ServiceStatusGrid({required this.strings, required this.values});

  final AppStrings strings;
  final List<_ServiceStatusData> values;

  @override
  Widget build(BuildContext context) => LayoutBuilder(
    builder: (context, constraints) {
      final columns = constraints.maxWidth >= 700
          ? 4
          : constraints.maxWidth >= 620
          ? 2
          : 1;
      final width = (constraints.maxWidth - (columns - 1) * 12) / columns;
      return Wrap(
        spacing: 12,
        runSpacing: 12,
        children: [
          for (final value in values)
            SizedBox(
              width: width,
              child: _ServiceStatusCard(data: value, strings: strings),
            ),
        ],
      );
    },
  );
}

class _ServiceStatusCard extends StatelessWidget {
  const _ServiceStatusCard({required this.data, required this.strings});

  final _ServiceStatusData data;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final state = data.rawValue ?? data.value;
    final color = appStatusColor(context, state);
    return AppSurface(
      padding: const EdgeInsets.fromLTRB(18, 18, 18, 16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 44,
                height: 44,
                decoration: BoxDecoration(
                  color: color.withValues(alpha: 0.10),
                  shape: BoxShape.circle,
                ),
                child: Icon(data.icon, color: color),
              ),
              const Spacer(),
              Container(
                width: 9,
                height: 9,
                decoration: BoxDecoration(color: color, shape: BoxShape.circle),
              ),
            ],
          ),
          const SizedBox(height: 14),
          Text(
            data.title,
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.w700,
            ),
          ),
          const SizedBox(height: 7),
          AppStatusPill(
            label: data.rawValue == null
                ? strings.stateLabel(state)
                : data.value,
            color: color,
          ),
          const SizedBox(height: 12),
          Text(
            data.detail,
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
            style: theme.textTheme.bodySmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }
}

class _ActionButton extends StatelessWidget {
  const _ActionButton({
    required this.icon,
    required this.label,
    required this.detail,
    required this.color,
    required this.onPressed,
  });

  final IconData icon;
  final String label;
  final String detail;
  final Color color;
  final VoidCallback? onPressed;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return OutlinedButton(
      onPressed: onPressed,
      style: OutlinedButton.styleFrom(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 15),
        alignment: Alignment.centerLeft,
        side: BorderSide(color: theme.colorScheme.outlineVariant),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(13)),
      ),
      child: Row(
        children: [
          Icon(icon, color: color),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(label, style: TextStyle(color: color)),
                const SizedBox(height: 3),
                Text(
                  detail,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _SystemServicePanel extends StatelessWidget {
  const _SystemServicePanel({
    required this.service,
    required this.controller,
    required this.strings,
    required this.onConfirm,
  });

  final ManagedServiceStatus service;
  final ServiceController controller;
  final AppStrings strings;
  final Future<void> Function(
    BuildContext context,
    String action,
    Future<bool> Function() operation,
  )
  onConfirm;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return AppSurface(
      padding: const EdgeInsets.fromLTRB(22, 20, 22, 22),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            strings.systemServiceTitle,
            style: theme.textTheme.titleLarge?.copyWith(
              fontWeight: FontWeight.w700,
            ),
          ),
          const SizedBox(height: 5),
          Text(
            strings.systemServiceSubtitle,
            style: theme.textTheme.bodyMedium?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 16),
          AppNotice(
            icon: Icons.desktop_windows_outlined,
            text: strings.portableModeLabel,
            color: theme.colorScheme.surfaceContainerLowest,
            textColor: theme.colorScheme.primary,
          ),
          const SizedBox(height: 16),
          Row(
            children: [
              Expanded(child: Text(strings.serviceState)),
              AppStatusPill(
                label: service.installed
                    ? strings.serviceInstalled
                    : strings.serviceNotInstalled,
                color: service.installed
                    ? theme.colorScheme.primary
                    : theme.colorScheme.error,
              ),
            ],
          ),
          const SizedBox(height: 4),
          Text(
            service.installed
                ? strings.serviceInstalledDescription
                : strings.serviceNotInstalledDescription,
            style: theme.textTheme.bodySmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 16),
          Wrap(
            spacing: 10,
            runSpacing: 10,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              if (service.installed) ...[
                FilledButton.icon(
                  onPressed: controller.busy
                      ? null
                      : () => onConfirm(
                          context,
                          strings.uninstallService,
                          controller.uninstall,
                        ),
                  icon: const Icon(Icons.delete_outline),
                  label: Text(strings.uninstallService),
                ),
              ],
              ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 300),
                child: Row(
                  children: [
                    Expanded(
                      child: Text(
                        strings.startsAtBoot,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    const SizedBox(width: 6),
                    Switch(value: service.startsAtBoot, onChanged: null),
                  ],
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _ServiceExplanation extends StatelessWidget {
  const _ServiceExplanation({required this.strings});

  final AppStrings strings;

  @override
  Widget build(BuildContext context) => AppSurface(
    padding: const EdgeInsets.fromLTRB(22, 20, 22, 22),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          strings.serviceExplanationTitle,
          style: Theme.of(
            context,
          ).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w700),
        ),
        const SizedBox(height: 16),
        AppNotice(
          icon: Icons.info_outline,
          text: strings.portableDaemonNote,
          color: Theme.of(
            context,
          ).colorScheme.primaryContainer.withValues(alpha: 0.28),
          textColor: Theme.of(context).colorScheme.onPrimaryContainer,
        ),
      ],
    ),
  );
}

class _ErrorState extends StatelessWidget {
  const _ErrorState({required this.strings, required this.onRetry});

  final AppStrings strings;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) => Center(
    child: Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(
          Icons.settings_outlined,
          size: 42,
          color: Theme.of(context).colorScheme.onSurfaceVariant,
        ),
        const SizedBox(height: 12),
        Text(strings.serviceUnavailable),
        const SizedBox(height: 12),
        FilledButton(onPressed: onRetry, child: Text(strings.retry)),
      ],
    ),
  );
}

String _errorCode(Object? error) =>
    error is DaemonApiException ? error.code : '';
