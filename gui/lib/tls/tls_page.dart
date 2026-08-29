import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/state/status_controller.dart';
import 'package:davdeck/state/tls_controller.dart';
import 'package:davdeck/widgets/app_ui.dart';
import 'package:flutter/material.dart';

class TlsPage extends StatefulWidget {
  const TlsPage({super.key, required this.controller, this.status});

  final TlsController controller;
  final StatusController? status;

  @override
  State<TlsPage> createState() => _TlsPageState();
}

class _TlsPageState extends State<TlsPage> {
  final hostname = TextEditingController();
  final certificatePath = TextEditingController();
  final privateKeyPath = TextEditingController();
  final draftRevision = ValueNotifier<int>(0);
  String mode = 'internal';
  Object? syncedProfile;
  bool syncingProfile = false;

  bool get hasUnsavedChanges {
    final profile = widget.controller.profile;
    final draftCertificatePath = mode == 'custom' ? certificatePath.text : '';
    final draftPrivateKeyPath = mode == 'custom' ? privateKeyPath.text : '';
    if (profile == null) {
      return mode != 'internal' ||
          hostname.text.isNotEmpty ||
          draftCertificatePath.isNotEmpty ||
          draftPrivateKeyPath.isNotEmpty;
    }
    return mode != profile.mode ||
        hostname.text != profile.hostname ||
        draftCertificatePath != profile.certificatePath ||
        draftPrivateKeyPath != profile.privateKeyPath;
  }

  @override
  void initState() {
    super.initState();
    hostname.addListener(_onDraftTextChanged);
    certificatePath.addListener(_onDraftTextChanged);
    privateKeyPath.addListener(_onDraftTextChanged);
  }

  @override
  void dispose() {
    hostname.removeListener(_onDraftTextChanged);
    certificatePath.removeListener(_onDraftTextChanged);
    privateKeyPath.removeListener(_onDraftTextChanged);
    hostname.dispose();
    certificatePath.dispose();
    privateKeyPath.dispose();
    draftRevision.dispose();
    super.dispose();
  }

  void syncProfile() {
    final profile = widget.controller.profile;
    if (profile == null || identical(profile, syncedProfile)) return;
    syncedProfile = profile;
    syncingProfile = true;
    try {
      mode = profile.mode;
      hostname.text = profile.hostname;
      certificatePath.text = profile.certificatePath;
      privateKeyPath.text = profile.privateKeyPath;
    } finally {
      syncingProfile = false;
    }
  }

  void _onDraftTextChanged() {
    if (!syncingProfile) draftRevision.value++;
  }

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    final listenable = widget.status == null
        ? Listenable.merge([widget.controller, draftRevision])
        : Listenable.merge([widget.controller, widget.status!, draftRevision]);
    return Scaffold(
      body: AnimatedBuilder(
        animation: listenable,
        builder: (context, _) {
          syncProfile();
          if (widget.controller.loading) {
            return const Center(child: CircularProgressIndicator());
          }
          return Stack(
            children: [
              SingleChildScrollView(
                padding: appPagePadding(context),
                child: Center(
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(maxWidth: 1120),
                    child: _TlsContent(
                      controller: widget.controller,
                      status: widget.status,
                      strings: strings,
                      mode: mode,
                      hostname: hostname,
                      certificatePath: certificatePath,
                      privateKeyPath: privateKeyPath,
                      hasUnsavedChanges: hasUnsavedChanges,
                      onModeChanged: (value) {
                        mode = value;
                        draftRevision.value++;
                      },
                      onSave: _save,
                      onApply: _apply,
                      onDisable: () => _disable(context),
                    ),
                  ),
                ),
              ),
              if (widget.controller.busy)
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

  Future<void> _save() async {
    final custom = mode == 'custom';
    final saved = await widget.controller.configure(
      mode: mode,
      hostname: hostname.text,
      certificatePath: custom ? certificatePath.text : '',
      privateKeyPath: custom ? privateKeyPath.text : '',
    );
    if (saved) await widget.status?.refresh();
  }

  Future<void> _apply() async {
    if (await widget.controller.apply()) {
      await widget.status?.refresh();
    }
  }

  Future<void> _disable(BuildContext context) async {
    final strings = AppStrings.of(context);
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(strings.disableHttps),
        content: Text(strings.confirmDisableHttps),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: Text(strings.cancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(true),
            child: Text(strings.disableHttps),
          ),
        ],
      ),
    );
    if (confirmed == true) {
      if (await widget.controller.disable() && mounted) {
        setState(() {
          mode = 'internal';
          hostname.clear();
          certificatePath.clear();
          privateKeyPath.clear();
        });
        await widget.status?.refresh();
      }
    }
  }
}

class _TlsContent extends StatelessWidget {
  const _TlsContent({
    required this.controller,
    required this.status,
    required this.strings,
    required this.mode,
    required this.hostname,
    required this.certificatePath,
    required this.privateKeyPath,
    required this.hasUnsavedChanges,
    required this.onModeChanged,
    required this.onSave,
    required this.onApply,
    required this.onDisable,
  });

  final TlsController controller;
  final StatusController? status;
  final AppStrings strings;
  final String mode;
  final TextEditingController hostname;
  final TextEditingController certificatePath;
  final TextEditingController privateKeyPath;
  final bool hasUnsavedChanges;
  final ValueChanged<String> onModeChanged;
  final VoidCallback onSave;
  final VoidCallback onApply;
  final VoidCallback onDisable;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final profile = controller.profile;
    final port = status?.serverSettings?.httpsPort;
    final pendingApply =
        controller.pendingApply || status?.status?.pendingChanges == true;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        AppPageHeader(
          title: strings.https,
          subtitle: strings.httpsSubtitle,
          actions: IconButton(
            tooltip: strings.refreshHttps,
            onPressed: controller.busy ? null : controller.refresh,
            icon: const Icon(Icons.refresh),
          ),
        ),
        const SizedBox(height: 32),
        _TlsModeSelector(
          strings: strings,
          mode: mode,
          enabled: !controller.busy,
          onChanged: onModeChanged,
        ),
        const SizedBox(height: 16),
        AppSurface(
          color: theme.colorScheme.primaryContainer.withValues(alpha: 0.16),
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              LayoutBuilder(
                builder: (context, constraints) {
                  final compact = constraints.maxWidth < 620;
                  final icon = Container(
                    width: 76,
                    height: 76,
                    decoration: BoxDecoration(
                      color: theme.colorScheme.surface.withValues(alpha: 0.72),
                      borderRadius: BorderRadius.circular(22),
                      border: Border.all(
                        color: theme.colorScheme.primary.withValues(
                          alpha: 0.18,
                        ),
                      ),
                    ),
                    child: Icon(
                      mode == 'automatic'
                          ? Icons.public
                          : Icons.shield_outlined,
                      size: 38,
                      color: theme.colorScheme.primary,
                    ),
                  );
                  final copy = Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        _modeTitle,
                        style: theme.textTheme.titleLarge?.copyWith(
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      const SizedBox(height: 8),
                      Text(
                        _modeDescription,
                        style: theme.textTheme.bodyLarge?.copyWith(
                          height: 1.45,
                        ),
                      ),
                    ],
                  );
                  return compact
                      ? Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [icon, const SizedBox(height: 16), copy],
                        )
                      : Row(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            icon,
                            const SizedBox(width: 20),
                            Expanded(child: copy),
                          ],
                        );
                },
              ),
              if (mode == 'internal') ...[
                const SizedBox(height: 22),
                AppNotice(
                  icon: Icons.info_outline,
                  text: strings.internalTrustWarning,
                  color: theme.colorScheme.primaryContainer.withValues(
                    alpha: 0.42,
                  ),
                  textColor: theme.colorScheme.onPrimaryContainer,
                ),
              ],
            ],
          ),
        ),
        const SizedBox(height: 24),
        AppSurface(
          padding: const EdgeInsets.fromLTRB(24, 22, 24, 18),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                strings.httpsSettings,
                style: theme.textTheme.titleLarge?.copyWith(
                  fontWeight: FontWeight.w700,
                ),
              ),
              const SizedBox(height: 18),
              _SettingRow(
                label: strings.hostname,
                child: TextField(
                  controller: hostname,
                  enabled: !controller.busy,
                  decoration: InputDecoration(
                    labelText: strings.hostname,
                    hintText: mode == 'automatic'
                        ? 'dav.example.com'
                        : 'dav.local',
                  ),
                ),
              ),
              if (port != null) ...[
                const Divider(height: 28),
                _SettingRow(label: strings.httpsPort, value: '$port'),
              ],
              if (mode == 'custom') ...[
                const Divider(height: 28),
                _SettingRow(
                  label: strings.certificatePathShort,
                  child: TextField(
                    controller: certificatePath,
                    enabled: !controller.busy,
                    decoration: InputDecoration(
                      labelText: strings.certificatePath,
                      hintText: '/path/to/certificate.pem',
                    ),
                  ),
                ),
                const Divider(height: 28),
                _SettingRow(
                  label: strings.privateKeyPathShort,
                  child: TextField(
                    controller: privateKeyPath,
                    enabled: !controller.busy,
                    decoration: InputDecoration(
                      labelText: strings.privateKeyPath,
                      hintText: '/path/to/private-key.pem',
                      helperText: strings.privateKeyPathSafety,
                    ),
                  ),
                ),
              ],
              const Divider(height: 28),
              _SettingRow(
                label: strings.certificateStatus,
                value: profile == null
                    ? strings.notConfigured
                    : strings.configured,
                valueColor: profile == null
                    ? theme.colorScheme.onSurfaceVariant
                    : theme.colorScheme.primary,
              ),
            ],
          ),
        ),
        const SizedBox(height: 20),
        Wrap(
          spacing: 12,
          runSpacing: 12,
          children: [
            if (pendingApply && !hasUnsavedChanges)
              FilledButton.icon(
                onPressed: controller.busy ? null : onApply,
                icon: const Icon(Icons.rocket_launch_outlined),
                label: Text(strings.applyConfiguration),
              ),
            if (hasUnsavedChanges)
              FilledButton.icon(
                onPressed: controller.busy ? null : onSave,
                icon: const Icon(Icons.save_outlined),
                label: Text(strings.saveTlsSettings),
              )
            else
              OutlinedButton.icon(
                onPressed: null,
                icon: const Icon(Icons.save_outlined),
                label: Text(strings.saveTlsSettings),
              ),
            OutlinedButton.icon(
              onPressed: controller.busy || profile == null || hasUnsavedChanges
                  ? null
                  : controller.check,
              icon: const Icon(Icons.fact_check_outlined),
              label: Text(strings.runPreflight),
            ),
            if (pendingApply && hasUnsavedChanges)
              OutlinedButton.icon(
                onPressed: null,
                icon: const Icon(Icons.rocket_launch_outlined),
                label: Text(strings.applyConfiguration),
              ),
            if (profile != null)
              OutlinedButton.icon(
                onPressed: controller.busy ? null : onDisable,
                icon: const Icon(Icons.lock_open_outlined),
                label: Text(strings.disableHttps),
              ),
            if (controller.error != null && profile == null)
              OutlinedButton(
                onPressed: controller.busy ? null : controller.refresh,
                child: Text(strings.retry),
              ),
          ],
        ),
        if (pendingApply) ...[
          const SizedBox(height: 16),
          AppNotice(
            icon: Icons.pending_actions,
            color: const Color(0xFFFFF4D9),
            textColor: const Color(0xFF7A5600),
            text: hasUnsavedChanges
                ? strings.pendingTlsApplyWithUnsavedChanges
                : strings.pendingTlsApply,
          ),
        ],
        if (controller.error case final error?) ...[
          const SizedBox(height: 16),
          Text(
            strings.httpsWizardTitle,
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.w700,
            ),
          ),
          const SizedBox(height: 8),
          AppNotice(
            icon: Icons.error_outline,
            text: error.toString(),
            color: theme.colorScheme.errorContainer,
            textColor: theme.colorScheme.onErrorContainer,
          ),
        ],
        if (controller.checkResult case final result?) ...[
          const SizedBox(height: 20),
          AppSurface(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  result.ready
                      ? strings.preflightReady
                      : strings.preflightFailed,
                  style: theme.textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.w700,
                  ),
                ),
                const SizedBox(height: 10),
                ...result.checks.map(
                  (check) => ListTile(
                    contentPadding: EdgeInsets.zero,
                    leading: Icon(
                      check.ok ? Icons.check_circle : Icons.error,
                      color: check.ok
                          ? theme.colorScheme.primary
                          : theme.colorScheme.error,
                    ),
                    title: Text(check.name),
                    subtitle: Text(check.message),
                  ),
                ),
              ],
            ),
          ),
        ],
      ],
    );
  }

  String get _modeTitle => switch (mode) {
    'automatic' => strings.tlsAutomaticTitle,
    'custom' => strings.tlsCustomTitle,
    _ => strings.tlsInternalTitle,
  };

  String get _modeDescription => switch (mode) {
    'automatic' => strings.automaticTlsDescription,
    'custom' => strings.customTlsDescription,
    _ => strings.internalTlsDescription,
  };
}

class _TlsModeSelector extends StatelessWidget {
  const _TlsModeSelector({
    required this.strings,
    required this.mode,
    required this.enabled,
    required this.onChanged,
  });

  final AppStrings strings;
  final String mode;
  final bool enabled;
  final ValueChanged<String> onChanged;

  @override
  Widget build(BuildContext context) => LayoutBuilder(
    builder: (context, constraints) {
      final segments = [
        ButtonSegment<String>(
          value: 'automatic',
          icon: const Icon(Icons.public),
          label: Text(strings.tlsAutomatic),
        ),
        ButtonSegment<String>(
          value: 'internal',
          icon: const Icon(Icons.lan),
          label: Text(strings.tlsInternal),
        ),
        ButtonSegment<String>(
          value: 'custom',
          icon: const Icon(Icons.badge_outlined),
          label: Text(strings.tlsCustom),
        ),
      ];
      return Wrap(
        spacing: 14,
        runSpacing: 8,
        children: [
          for (final segment in segments)
            ChoiceChip(
              avatar: segment.icon,
              label: segment.label ?? const SizedBox.shrink(),
              selected: mode == segment.value,
              selectedColor: const Color(0xFFE4F2EB),
              backgroundColor: Colors.white,
              side: BorderSide(
                color: Theme.of(context).colorScheme.outlineVariant,
              ),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(11),
              ),
              labelStyle: TextStyle(
                color: mode == segment.value
                    ? Theme.of(context).colorScheme.primary
                    : Theme.of(context).colorScheme.onSurface,
                fontWeight: FontWeight.w600,
              ),
              onSelected: enabled ? (_) => onChanged(segment.value) : null,
            ),
        ],
      );
    },
  );
}

class _SettingRow extends StatelessWidget {
  const _SettingRow({
    required this.label,
    this.child,
    this.value,
    this.valueColor,
  });

  final String label;
  final Widget? child;
  final String? value;
  final Color? valueColor;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return LayoutBuilder(
      builder: (context, constraints) {
        final compact = constraints.maxWidth < 620;
        final labelWidget = Text(
          label,
          style: theme.textTheme.titleSmall?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
          ),
        );
        final field =
            child ??
            Text(
              value ?? '',
              style: theme.textTheme.bodyLarge?.copyWith(color: valueColor),
            );
        return compact
            ? Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [labelWidget, const SizedBox(height: 8), field],
              )
            : Row(
                crossAxisAlignment: CrossAxisAlignment.center,
                children: [
                  SizedBox(width: 180, child: labelWidget),
                  Expanded(child: field),
                ],
              );
      },
    );
  }
}
