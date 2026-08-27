import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/state/status_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

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
      body: AnimatedBuilder(
        animation: controller,
        builder: (context, _) => switch (controller.state) {
          LoadState.loading => _LoadingState(strings: strings),
          LoadState.error => _ErrorState(
            controller: controller,
            strings: strings,
          ),
          LoadState.ready => _DashboardContent(
            controller: controller,
            strings: strings,
            onOpenService: onOpenService,
          ),
        },
      ),
    );
  }
}

class _LoadingState extends StatelessWidget {
  const _LoadingState({required this.strings});

  final AppStrings strings;

  @override
  Widget build(BuildContext context) => Center(
    child: Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        const SizedBox(
          width: 28,
          height: 28,
          child: CircularProgressIndicator(strokeWidth: 3),
        ),
        const SizedBox(height: 16),
        Text(strings.loading),
      ],
    ),
  );
}

class _ErrorState extends StatelessWidget {
  const _ErrorState({required this.controller, required this.strings});

  final StatusController controller;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 64,
              height: 64,
              decoration: BoxDecoration(
                color: colors.errorContainer,
                shape: BoxShape.circle,
              ),
              child: Icon(
                Icons.cloud_off_rounded,
                color: colors.onErrorContainer,
                size: 30,
              ),
            ),
            const SizedBox(height: 20),
            Text(
              strings.unavailable,
              textAlign: TextAlign.center,
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 16),
            FilledButton.icon(
              onPressed: controller.refresh,
              icon: const Icon(Icons.refresh_rounded),
              label: Text(strings.retry),
            ),
          ],
        ),
      ),
    );
  }
}

class _DashboardContent extends StatelessWidget {
  const _DashboardContent({
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
    return LayoutBuilder(
      builder: (context, constraints) {
        final horizontalPadding = constraints.maxWidth < 680 ? 20.0 : 40.0;
        final compact = constraints.maxWidth < 900;
        final shortViewport = constraints.maxHeight < 700;
        return SingleChildScrollView(
          padding: EdgeInsets.fromLTRB(
            horizontalPadding,
            40,
            horizontalPadding,
            40,
          ),
          child: Center(
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 1120),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _PageHeader(
                    title: '${status.name} · ${strings.dashboard}',
                    subtitle: strings.dashboardSubtitle,
                    onRefresh: controller.refresh,
                  ),
                  if (compact &&
                      status.pendingChanges &&
                      controller.configuration != null) ...[
                    const SizedBox(height: 18),
                    _PendingConfiguration(
                      controller: controller,
                      strings: strings,
                    ),
                  ],
                  if (compact && shortViewport) ...[
                    const SizedBox(height: 18),
                    _ServiceControlPanel(
                      status: status,
                      controller: controller,
                      strings: strings,
                      onOpenService: onOpenService,
                      includePending: false,
                    ),
                  ],
                  const SizedBox(height: 24),
                  _HeroStatusPanel(
                    status: status,
                    controller: controller,
                    strings: strings,
                  ),
                  const SizedBox(height: 24),
                  _ComponentStatusGrid(status: status, strings: strings),
                  if (compact && !shortViewport) ...[
                    const SizedBox(height: 24),
                    _ServiceControlPanel(
                      status: status,
                      controller: controller,
                      strings: strings,
                      onOpenService: onOpenService,
                      includePending: false,
                    ),
                    const SizedBox(height: 24),
                    _EndpointsPanel(controller: controller, strings: strings),
                    const SizedBox(height: 24),
                  ],
                  if (!compact) ...[
                    const SizedBox(height: 24),
                    _ControlAndEndpoints(
                      status: status,
                      controller: controller,
                      strings: strings,
                      onOpenService: onOpenService,
                    ),
                    const SizedBox(height: 24),
                  ],
                  _SystemInfoPanel(status: status, strings: strings),
                ],
              ),
            ),
          ),
        );
      },
    );
  }
}

class _PageHeader extends StatelessWidget {
  const _PageHeader({
    required this.title,
    required this.subtitle,
    required this.onRefresh,
  });

  final String title;
  final String subtitle;
  final VoidCallback onRefresh;

  @override
  Widget build(BuildContext context) => Row(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      Expanded(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              title,
              style: Theme.of(context).textTheme.headlineLarge?.copyWith(
                fontWeight: FontWeight.w700,
                letterSpacing: -0.8,
              ),
            ),
            const SizedBox(height: 6),
            Text(
              subtitle,
              style: Theme.of(
                context,
              ).textTheme.bodyLarge?.copyWith(color: const Color(0xFF64706C)),
            ),
          ],
        ),
      ),
      IconButton(
        onPressed: onRefresh,
        tooltip: AppStrings.of(context).refreshDashboard,
        icon: const Icon(Icons.refresh_rounded),
      ),
    ],
  );
}

class _HeroStatusPanel extends StatelessWidget {
  const _HeroStatusPanel({
    required this.status,
    required this.controller,
    required this.strings,
  });

  final DaemonStatus status;
  final StatusController controller;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) => _DashboardPanel(
    emphasized: true,
    child: Padding(
      padding: const EdgeInsets.all(26),
      child: Column(
        children: [
          LayoutBuilder(
            builder: (context, constraints) {
              return Align(
                alignment: Alignment.centerLeft,
                child: _HeroIdentity(status: status, strings: strings),
              );
            },
          ),
          const SizedBox(height: 20),
          const Divider(height: 1),
          const SizedBox(height: 16),
          Row(
            children: [
              Icon(
                Icons.info_outline_rounded,
                size: 18,
                color: Theme.of(context).colorScheme.primary,
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  status.portableDaemonOwned
                      ? strings.portableDaemonNote
                      : strings.localApiConnected,
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    color: const Color(0xFF697570),
                  ),
                ),
              ),
              if (status.pendingChanges)
                _StatusPill(label: strings.pending, tone: _StatusTone.warning),
            ],
          ),
          if (status.lastErrorCode != null) ...[
            const SizedBox(height: 12),
            _InlineError(text: '${strings.lastError}: ${status.lastErrorCode}'),
          ],
          if (controller.runtime != null &&
              controller.runtime!.lastErrorCode != null) ...[
            const SizedBox(height: 8),
            _InlineError(
              text:
                  '${strings.lastError}: ${controller.runtime!.lastErrorCode}',
            ),
          ],
        ],
      ),
    ),
  );
}

class _HeroIdentity extends StatelessWidget {
  const _HeroIdentity({required this.status, required this.strings});

  final DaemonStatus status;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) => Row(
    children: [
      Stack(
        clipBehavior: Clip.none,
        children: [
          Container(
            width: 76,
            height: 76,
            decoration: BoxDecoration(
              color: const Color(0xFFF7FBF9),
              border: Border.all(color: const Color(0xFFD8E8E0)),
              borderRadius: BorderRadius.circular(22),
            ),
            child: Icon(
              Icons.dns_rounded,
              color: Theme.of(context).colorScheme.primary,
              size: 40,
            ),
          ),
          Positioned(
            right: -7,
            bottom: -5,
            child: _StateIcon(state: _overallState(status), size: 30),
          ),
        ],
      ),
      const SizedBox(width: 18),
      Expanded(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              status.name,
              style: Theme.of(
                context,
              ).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w600),
            ),
            const SizedBox(height: 8),
            _StatusPill(
              label: _overallState(status) == 'RUNNING'
                  ? strings.dashboardHealthy
                  : strings.dashboardAttention,
              tone: _toneFor(_overallState(status)),
            ),
          ],
        ),
      ),
    ],
  );
}

class _ComponentStatusGrid extends StatelessWidget {
  const _ComponentStatusGrid({required this.status, required this.strings});

  final DaemonStatus status;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) {
    final cards = [
      _ComponentData(
        title: strings.daemon,
        status: status.daemon,
        detail: strings.daemonHealthy,
        icon: Icons.shield_outlined,
      ),
      _ComponentData(
        title: strings.database,
        status: status.database,
        detail: strings.databaseHealthy,
        icon: Icons.storage_outlined,
      ),
      _ComponentData(
        title: 'Caddy',
        status: status.caddy,
        detail: strings.caddyDetail,
        icon: Icons.language_rounded,
      ),
      _ComponentData(
        title: 'WebDAV',
        status: status.webdav,
        detail: strings.webdavDetail,
        icon: Icons.public_rounded,
      ),
      _ComponentData(
        title: strings.service,
        status: status.service.state,
        detail: status.service.installed
            ? strings.serviceInstalled
            : strings.serviceNotInstalled,
        icon: Icons.settings_outlined,
      ),
    ];

    return LayoutBuilder(
      builder: (context, constraints) {
        final columns = constraints.maxWidth >= 1100
            ? 5
            : constraints.maxWidth >= 720
            ? 3
            : constraints.maxWidth >= 480
            ? 2
            : 1;
        const gap = 14.0;
        final width = (constraints.maxWidth - (columns - 1) * gap) / columns;
        return Wrap(
          spacing: gap,
          runSpacing: gap,
          children: [
            for (final card in cards)
              SizedBox(
                width: width,
                child: _ComponentStatusCard(data: card, strings: strings),
              ),
          ],
        );
      },
    );
  }
}

class _ComponentData {
  const _ComponentData({
    required this.title,
    required this.status,
    required this.detail,
    required this.icon,
  });

  final String title;
  final String status;
  final String detail;
  final IconData icon;
}

class _ComponentStatusCard extends StatelessWidget {
  const _ComponentStatusCard({required this.data, required this.strings});

  final _ComponentData data;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) => _DashboardPanel(
    child: Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          LayoutBuilder(
            builder: (context, constraints) {
              final icon = Container(
                width: 38,
                height: 38,
                decoration: BoxDecoration(
                  color: _toneFor(data.status).background(context),
                  shape: BoxShape.circle,
                ),
                child: Icon(
                  data.icon,
                  size: 21,
                  color: _toneFor(data.status).foreground(context),
                ),
              );
              final title = Text(
                strings.locale.languageCode == 'zh'
                    ? data.title
                    : '${data.title}: ${data.status}',
                overflow: TextOverflow.ellipsis,
                style: Theme.of(
                  context,
                ).textTheme.titleSmall?.copyWith(fontWeight: FontWeight.w600),
              );
              final pill = strings.locale.languageCode == 'zh'
                  ? _StatusPill(
                      label: strings.stateLabel(data.status),
                      tone: _toneFor(data.status),
                    )
                  : const SizedBox.shrink();
              if (constraints.maxWidth < 260) {
                return Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    icon,
                    const SizedBox(width: 10),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [title, const SizedBox(height: 6), pill],
                      ),
                    ),
                  ],
                );
              }
              return Row(
                children: [
                  icon,
                  const SizedBox(width: 10),
                  Expanded(child: title),
                  Flexible(child: pill),
                ],
              );
            },
          ),
          const SizedBox(height: 15),
          const Divider(height: 1),
          const SizedBox(height: 12),
          Row(
            children: [
              Container(
                width: 9,
                height: 9,
                decoration: BoxDecoration(
                  color: _toneFor(data.status).dot(context),
                  shape: BoxShape.circle,
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  data.detail,
                  overflow: TextOverflow.ellipsis,
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    color: const Color(0xFF6A756F),
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    ),
  );
}

class _ControlAndEndpoints extends StatelessWidget {
  const _ControlAndEndpoints({
    required this.status,
    required this.controller,
    required this.strings,
    this.onOpenService,
  });

  final DaemonStatus status;
  final StatusController controller;
  final AppStrings strings;
  final VoidCallback? onOpenService;

  @override
  Widget build(BuildContext context) => LayoutBuilder(
    builder: (context, constraints) {
      final isWide = constraints.maxWidth >= 800;
      final panelWidth = isWide
          ? (constraints.maxWidth - 16) / 2
          : constraints.maxWidth;
      final control = SizedBox(
        width: panelWidth,
        child: _ServiceControlPanel(
          status: status,
          controller: controller,
          strings: strings,
          onOpenService: onOpenService,
          includePending: true,
        ),
      );
      final endpoints = SizedBox(
        width: panelWidth,
        child: _EndpointsPanel(controller: controller, strings: strings),
      );
      return isWide
          ? Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [control, const SizedBox(width: 16), endpoints],
            )
          : Column(children: [control, const SizedBox(height: 16), endpoints]);
    },
  );
}

class _ServiceControlPanel extends StatelessWidget {
  const _ServiceControlPanel({
    required this.status,
    required this.controller,
    required this.strings,
    this.onOpenService,
    this.includePending = true,
  });

  final DaemonStatus status;
  final StatusController controller;
  final AppStrings strings;
  final VoidCallback? onOpenService;
  final bool includePending;

  @override
  Widget build(BuildContext context) => _DashboardPanel(
    child: Padding(
      padding: const EdgeInsets.all(20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            strings.serviceControl,
            style: Theme.of(
              context,
            ).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w600),
          ),
          const SizedBox(height: 4),
          Text(
            strings.serviceControlSubtitle,
            style: Theme.of(
              context,
            ).textTheme.bodySmall?.copyWith(color: const Color(0xFF75807B)),
          ),
          const SizedBox(height: 18),
          Row(
            children: [
              Expanded(
                child: _ActionButton(
                  label: strings.start,
                  icon: Icons.play_arrow_rounded,
                  filled: true,
                  onPressed: controller.server == null || controller.busy
                      ? null
                      : () =>
                            controller.control(controller.server!.startServer),
                ),
              ),
              const SizedBox(width: 10),
              Expanded(
                child: _ActionButton(
                  label: strings.stop,
                  icon: Icons.stop_rounded,
                  onPressed: controller.server == null || controller.busy
                      ? null
                      : () => controller.control(controller.server!.stopServer),
                ),
              ),
              const SizedBox(width: 10),
              Expanded(
                child: _ActionButton(
                  label: strings.restart,
                  icon: Icons.refresh_rounded,
                  onPressed: controller.server == null || controller.busy
                      ? null
                      : () => controller.control(
                          controller.server!.restartServer,
                        ),
                ),
              ),
            ],
          ),
          if (onOpenService != null) ...[
            const SizedBox(height: 14),
            _PanelLink(
              icon: Icons.settings_outlined,
              label: strings.openService,
              onTap: onOpenService!,
            ),
          ],
          if (includePending &&
              status.pendingChanges &&
              controller.configuration != null) ...[
            const SizedBox(height: 12),
            _PendingConfiguration(controller: controller, strings: strings),
          ],
          if (controller.actionError != null) ...[
            const SizedBox(height: 12),
            _InlineError(
              text: '${strings.caddyActionFailed}\n${controller.actionError}',
            ),
          ],
          if (controller.applyResult case final revision?) ...[
            const SizedBox(height: 12),
            Text(
              strings.configurationAppliedRevision(revision.number),
              style: TextStyle(color: Theme.of(context).colorScheme.primary),
            ),
          ],
        ],
      ),
    ),
  );
}

class _ActionButton extends StatelessWidget {
  const _ActionButton({
    required this.label,
    required this.icon,
    required this.onPressed,
    this.filled = false,
  });

  final String label;
  final IconData icon;
  final VoidCallback? onPressed;
  final bool filled;

  @override
  Widget build(BuildContext context) {
    final child = Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [Icon(icon, size: 19), const SizedBox(width: 7), Text(label)],
    );
    final style = ButtonStyle(
      minimumSize: const WidgetStatePropertyAll(Size(0, 52)),
      padding: const WidgetStatePropertyAll(
        EdgeInsets.symmetric(horizontal: 10, vertical: 8),
      ),
      shape: WidgetStatePropertyAll(
        RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      ),
      foregroundColor: WidgetStateProperty.resolveWith(
        (states) => states.contains(WidgetState.disabled)
            ? const Color(0xFFA6B0AB)
            : const Color(0xFF4E5E57),
      ),
      side: const WidgetStatePropertyAll(BorderSide(color: Color(0xFFD9E3DE))),
    );
    if (filled) {
      return FilledButton(
        onPressed: onPressed,
        style: style.copyWith(
          backgroundColor: WidgetStateProperty.resolveWith(
            (states) => states.contains(WidgetState.disabled)
                ? const Color(0xFFDCE9E3)
                : const Color(0xFF167B59),
          ),
          foregroundColor: WidgetStateProperty.resolveWith(
            (states) => states.contains(WidgetState.disabled)
                ? const Color(0xFF8EA399)
                : Colors.white,
          ),
          side: const WidgetStatePropertyAll(BorderSide.none),
        ),
        child: child,
      );
    }
    return OutlinedButton(onPressed: onPressed, style: style, child: child);
  }
}

class _PendingConfiguration extends StatelessWidget {
  const _PendingConfiguration({
    required this.controller,
    required this.strings,
  });

  final StatusController controller;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.fromLTRB(12, 10, 8, 10),
    decoration: BoxDecoration(
      color: const Color(0xFFFFF8E7),
      borderRadius: BorderRadius.circular(12),
      border: Border.all(color: const Color(0xFFF0D58A)),
    ),
    child: Row(
      children: [
        const Icon(
          Icons.pending_actions_rounded,
          size: 18,
          color: Color(0xFF9A6A00),
        ),
        const SizedBox(width: 8),
        Expanded(
          child: Text(
            strings.pendingConfiguration,
            style: const TextStyle(color: Color(0xFF765400)),
          ),
        ),
        TextButton(
          onPressed: controller.busy ? null : controller.applyPending,
          child: Text(strings.applyConfiguration),
        ),
      ],
    ),
  );
}

class _EndpointsPanel extends StatelessWidget {
  const _EndpointsPanel({required this.controller, required this.strings});

  final StatusController controller;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) {
    final settings = controller.serverSettings;
    final httpPort = settings?.httpPort;
    final httpsPort = settings?.httpsPort;
    final publicBasePath = settings?.publicBasePath ?? '/dav';
    final endpointPath = publicBasePath == '/'
        ? '/'
        : '${publicBasePath.replaceFirst(RegExp(r'/$'), '')}/';
    return _DashboardPanel(
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              strings.accessEndpoints,
              style: Theme.of(
                context,
              ).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w600),
            ),
            const SizedBox(height: 4),
            Text(
              strings.accessEndpointsSubtitle,
              style: Theme.of(
                context,
              ).textTheme.bodySmall?.copyWith(color: const Color(0xFF75807B)),
            ),
            const SizedBox(height: 16),
            _EndpointRow(
              protocol: 'HTTP',
              port: httpPort,
              endpointPath: endpointPath,
              icon: Icons.language_rounded,
              onCopy: httpPort == null
                  ? null
                  : () => _copyEndpoint(
                      context,
                      'http://localhost:$httpPort$endpointPath',
                      strings,
                    ),
            ),
            const SizedBox(height: 10),
            _EndpointRow(
              protocol: 'HTTPS',
              port: httpsPort,
              endpointPath: endpointPath,
              icon: Icons.lock_outline_rounded,
              onCopy: httpsPort == null
                  ? null
                  : () => _copyEndpoint(
                      context,
                      'https://localhost:$httpsPort$endpointPath',
                      strings,
                    ),
            ),
            const SizedBox(height: 12),
            _PanelLink(
              icon: Icons.tune_rounded,
              label: strings.editPorts,
              onTap: controller.busy
                  ? null
                  : () => _editPorts(context, controller, strings),
            ),
          ],
        ),
      ),
    );
  }
}

class _EndpointRow extends StatelessWidget {
  const _EndpointRow({
    required this.protocol,
    required this.port,
    required this.endpointPath,
    required this.icon,
    required this.onCopy,
  });

  final String protocol;
  final int? port;
  final String endpointPath;
  final IconData icon;
  final VoidCallback? onCopy;

  @override
  Widget build(BuildContext context) {
    final url = port == null
        ? '—'
        : '${protocol.toLowerCase()}://localhost:$port$endpointPath';
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: const Color(0xFFFBFCFC),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: const Color(0xFFE5EBE8)),
      ),
      child: Row(
        children: [
          Icon(icon, size: 21, color: Theme.of(context).colorScheme.primary),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  protocol,
                  style: Theme.of(
                    context,
                  ).textTheme.labelLarge?.copyWith(fontWeight: FontWeight.w700),
                ),
                Text(
                  port == null ? '—' : 'Port $port',
                  style: Theme.of(context).textTheme.labelSmall?.copyWith(
                    color: const Color(0xFF7A8580),
                  ),
                ),
              ],
            ),
          ),
          Flexible(
            child: Text(
              url,
              overflow: TextOverflow.ellipsis,
              textAlign: TextAlign.right,
              style: Theme.of(
                context,
              ).textTheme.bodyMedium?.copyWith(color: const Color(0xFF53605A)),
            ),
          ),
          const SizedBox(width: 4),
          IconButton(
            onPressed: onCopy,
            tooltip: 'Copy $protocol endpoint',
            icon: const Icon(Icons.copy_all_outlined, size: 19),
          ),
        ],
      ),
    );
  }
}

Future<void> _copyEndpoint(
  BuildContext context,
  String endpoint,
  AppStrings strings,
) async {
  await Clipboard.setData(ClipboardData(text: endpoint));
  if (!context.mounted) return;
  ScaffoldMessenger.of(
    context,
  ).showSnackBar(SnackBar(content: Text(strings.endpointCopied)));
}

class _SystemInfoPanel extends StatelessWidget {
  const _SystemInfoPanel({required this.status, required this.strings});

  final DaemonStatus status;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) {
    final items = [
      _SystemInfo(
        icon: Icons.layers_outlined,
        label: strings.schema,
        value: '${strings.schema} ${status.schemaVersion}',
      ),
      _SystemInfo(
        icon: Icons.sell_outlined,
        label: strings.version,
        value: status.version,
      ),
      _SystemInfo(
        icon: Icons.settings_suggest_outlined,
        label: strings.runtimeMode,
        value: status.portableDaemonOwned
            ? strings.portableMode
            : strings.managedMode,
      ),
      _SystemInfo(
        icon: Icons.pending_actions_outlined,
        label: strings.configurationState,
        value: status.pendingChanges ? strings.pending : strings.applied,
      ),
    ];
    return _DashboardPanel(
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              strings.systemInformation,
              style: Theme.of(
                context,
              ).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w600),
            ),
            const SizedBox(height: 16),
            LayoutBuilder(
              builder: (context, constraints) {
                final columns = constraints.maxWidth >= 900
                    ? 4
                    : constraints.maxWidth >= 580
                    ? 2
                    : 1;
                const gap = 16.0;
                final width =
                    (constraints.maxWidth - (columns - 1) * gap) / columns;
                return Wrap(
                  spacing: gap,
                  runSpacing: 12,
                  children: [
                    for (final item in items)
                      SizedBox(
                        width: width,
                        child: _SystemInfoItem(item: item),
                      ),
                  ],
                );
              },
            ),
          ],
        ),
      ),
    );
  }
}

class _SystemInfo {
  const _SystemInfo({
    required this.icon,
    required this.label,
    required this.value,
  });

  final IconData icon;
  final String label;
  final String value;
}

class _SystemInfoItem extends StatelessWidget {
  const _SystemInfoItem({required this.item});

  final _SystemInfo item;

  @override
  Widget build(BuildContext context) => Row(
    children: [
      Icon(item.icon, size: 25, color: Theme.of(context).colorScheme.primary),
      const SizedBox(width: 12),
      Expanded(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              item.label,
              style: Theme.of(
                context,
              ).textTheme.bodySmall?.copyWith(color: const Color(0xFF7A8580)),
            ),
            const SizedBox(height: 2),
            Text(
              item.value,
              overflow: TextOverflow.ellipsis,
              style: Theme.of(
                context,
              ).textTheme.bodyMedium?.copyWith(fontWeight: FontWeight.w600),
            ),
          ],
        ),
      ),
    ],
  );
}

class _DashboardPanel extends StatelessWidget {
  const _DashboardPanel({required this.child, this.emphasized = false});

  final Widget child;
  final bool emphasized;

  @override
  Widget build(BuildContext context) => Container(
    decoration: BoxDecoration(
      color: emphasized ? const Color(0xFFFCFEFD) : Colors.white,
      borderRadius: BorderRadius.circular(16),
      border: Border.all(
        color: emphasized ? const Color(0xFFCFE1D9) : const Color(0xFFE1E8E4),
      ),
      boxShadow: const [
        BoxShadow(
          color: Color(0x0D1A2B24),
          blurRadius: 12,
          offset: Offset(0, 4),
        ),
      ],
    ),
    child: child,
  );
}

class _PanelLink extends StatelessWidget {
  const _PanelLink({required this.icon, required this.label, this.onTap});

  final IconData icon;
  final String label;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) => Material(
    color: const Color(0xFFF5FAF7),
    borderRadius: BorderRadius.circular(11),
    child: InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(11),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 11),
        child: Row(
          children: [
            Icon(icon, size: 19, color: Theme.of(context).colorScheme.primary),
            const SizedBox(width: 9),
            Expanded(
              child: Text(
                label,
                style: TextStyle(
                  color: Theme.of(context).colorScheme.primary,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ),
            Icon(
              Icons.chevron_right_rounded,
              size: 20,
              color: Theme.of(context).colorScheme.primary,
            ),
          ],
        ),
      ),
    ),
  );
}

class _InlineError extends StatelessWidget {
  const _InlineError({required this.text});

  final String text;

  @override
  Widget build(BuildContext context) => Container(
    width: double.infinity,
    padding: const EdgeInsets.all(12),
    decoration: BoxDecoration(
      color: Theme.of(context).colorScheme.errorContainer,
      borderRadius: BorderRadius.circular(10),
    ),
    child: Text(
      text,
      style: TextStyle(color: Theme.of(context).colorScheme.onErrorContainer),
    ),
  );
}

enum _StatusTone { positive, neutral, warning, negative }

String _overallState(DaemonStatus status) {
  final componentStates = [
    status.daemon,
    status.database,
    status.caddy,
    status.webdav,
    status.service.state,
  ];
  if (componentStates.any(
    (state) =>
        state.toUpperCase() == 'FAILED' || state.toUpperCase() == 'ERROR',
  )) {
    return 'FAILED';
  }
  if (status.daemon.toUpperCase() == 'RUNNING' &&
      status.database.toUpperCase() == 'READY') {
    return 'RUNNING';
  }
  return status.daemon;
}

_StatusTone _toneFor(String state) => switch (state.toUpperCase()) {
  'RUNNING' ||
  'READY' ||
  'ENABLED' ||
  'YES' ||
  'INSTALLED' => _StatusTone.positive,
  'STARTING' || 'STOPPING' || 'DEGRADED' => _StatusTone.warning,
  'FAILED' || 'ERROR' => _StatusTone.negative,
  _ => _StatusTone.neutral,
};

extension on _StatusTone {
  Color background(BuildContext context) => switch (this) {
    _StatusTone.positive => const Color(0xFFE8F7EE),
    _StatusTone.warning => const Color(0xFFFFF4D9),
    _StatusTone.negative => Theme.of(context).colorScheme.errorContainer,
    _StatusTone.neutral => const Color(0xFFF0F2F2),
  };

  Color foreground(BuildContext context) => switch (this) {
    _StatusTone.positive => const Color(0xFF18834A),
    _StatusTone.warning => const Color(0xFF9A6A00),
    _StatusTone.negative => Theme.of(context).colorScheme.onErrorContainer,
    _StatusTone.neutral => const Color(0xFF65706D),
  };

  Color dot(BuildContext context) => switch (this) {
    _StatusTone.positive => const Color(0xFF39B864),
    _StatusTone.warning => const Color(0xFFE2A91A),
    _StatusTone.negative => Theme.of(context).colorScheme.error,
    _StatusTone.neutral => const Color(0xFF9BA19F),
  };
}

class _StatusPill extends StatelessWidget {
  const _StatusPill({required this.label, required this.tone});

  final String label;
  final _StatusTone tone;

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 4),
    decoration: BoxDecoration(
      color: tone.background(context),
      borderRadius: BorderRadius.circular(999),
    ),
    child: Text(
      label,
      overflow: TextOverflow.ellipsis,
      style: TextStyle(
        color: tone.foreground(context),
        fontSize: 12,
        fontWeight: FontWeight.w600,
      ),
    ),
  );
}

class _StateIcon extends StatelessWidget {
  const _StateIcon({required this.state, required this.size});

  final String state;
  final double size;

  @override
  Widget build(BuildContext context) => Container(
    width: size,
    height: size,
    decoration: BoxDecoration(
      color: _toneFor(state).dot(context),
      shape: BoxShape.circle,
      border: Border.all(color: Colors.white, width: 2),
    ),
    child: Icon(
      _toneFor(state) == _StatusTone.positive
          ? Icons.check_rounded
          : Icons.priority_high_rounded,
      size: size * 0.58,
      color: Colors.white,
    ),
  );
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
