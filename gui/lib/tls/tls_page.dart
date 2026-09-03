import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/state/status_controller.dart';
import 'package:davdeck/state/tls_controller.dart';
import 'package:davdeck/widgets/app_ui.dart';
import 'package:flutter/material.dart';

class TlsPage extends StatefulWidget {
  const TlsPage({
    super.key,
    required this.controller,
    this.status,
    this.onOpenLogs,
  });

  final TlsController controller;
  final StatusController? status;
  final VoidCallback? onOpenLogs;

  @override
  State<TlsPage> createState() => _TlsPageState();
}

class _TlsPageState extends State<TlsPage> {
  final hostname = TextEditingController();
  final certificatePath = TextEditingController();
  final privateKeyPath = TextEditingController();
  final draftRevision = ValueNotifier<int>(0);
  String mode = 'internal';
  String challenge = 'auto';
  String? dnsProviderId;
  Object? syncedProfile;
  bool syncingProfile = false;

  bool get hasUnsavedChanges {
    final profile = widget.controller.profile;
    final draftCertificatePath = mode == 'custom' ? certificatePath.text : '';
    final draftPrivateKeyPath = mode == 'custom' ? privateKeyPath.text : '';
    if (profile == null) {
      return mode != 'internal' ||
          (mode == 'automatic' && challenge != 'auto') ||
          (mode == 'automatic' && dnsProviderId != null) ||
          hostname.text.isNotEmpty ||
          draftCertificatePath.isNotEmpty ||
          draftPrivateKeyPath.isNotEmpty;
    }
    return mode != profile.mode ||
        hostname.text != profile.hostname ||
        challenge != profile.challenge ||
        dnsProviderId != profile.dnsProviderId ||
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
      challenge = profile.challenge;
      dnsProviderId = profile.dnsProviderId;
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
                      onOpenLogs: widget.onOpenLogs,
                      strings: strings,
                      mode: mode,
                      hostname: hostname,
                      certificatePath: certificatePath,
                      privateKeyPath: privateKeyPath,
                      challenge: challenge,
                      dnsProviderId: dnsProviderId,
                      dnsProviders: widget.controller.dnsProviders,
                      hasUnsavedChanges: hasUnsavedChanges,
                      onModeChanged: (value) {
                        mode = value;
                        // Keep the automatic certificate strategy in the draft
                        // while the user previews another mode. Switching to
                        // internal/custom must not silently turn a saved
                        // DNS-01 renewal path into HTTP-01 when the user comes
                        // back to public automatic HTTPS.
                        draftRevision.value++;
                      },
                      onChallengeChanged: (value) {
                        challenge = value;
                        if (challenge != 'dns') dnsProviderId = null;
                        draftRevision.value++;
                      },
                      onDnsProviderChanged: (value) {
                        dnsProviderId = value;
                        draftRevision.value++;
                      },
                      onManageDnsProviders:
                          widget.controller.canManageDnsProviders
                          ? () => _manageDnsProviders(context)
                          : null,
                      onSave: _save,
                      onApply: _apply,
                      onDisable: () => _disable(context),
                      onRenew: () => _renew(context),
                      onCancelRenewal: () => _cancelRenewal(context),
                      onCancelCertificate: () => _cancelCertificate(context),
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
      challenge: mode == 'automatic' ? challenge : 'auto',
      dnsProviderId: mode == 'automatic' && challenge == 'dns'
          ? dnsProviderId
          : null,
    );
    if (saved) {
      await widget.controller.refresh();
      await widget.status?.refresh();
    }
  }

  Future<void> _apply() async {
    if (await widget.controller.apply()) {
      await widget.controller.refresh();
      await widget.status?.refresh();
    }
  }

  Future<void> _renew(BuildContext context) async {
    final strings = AppStrings.of(context);
    final confirmed = await showAppDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(strings.renewCertificate),
        content: Text(strings.confirmRenewCertificate),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: Text(strings.cancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(true),
            child: Text(strings.renewCertificate),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    if (!mounted) return;
    if (await widget.controller.renew()) {
      if (!mounted) return;
      await widget.controller.refresh();
      if (mounted) await widget.status?.refresh();
    }
  }

  Future<void> _manageDnsProviders(BuildContext context) async {
    await showAppDialog<void>(
      context: context,
      builder: (dialogContext) =>
          _DnsProviderManagerDialog(controller: widget.controller),
    );
    if (mounted) await widget.status?.refresh();
    if (!mounted ||
        dnsProviderId == null ||
        widget.controller.dnsProviders.any(
          (provider) => provider.id == dnsProviderId,
        )) {
      return;
    }
    setState(() => dnsProviderId = null);
    draftRevision.value++;
  }

  Future<void> _disable(BuildContext context) async {
    final strings = AppStrings.of(context);
    final confirmed = await showAppDialog<bool>(
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
          challenge = 'auto';
          dnsProviderId = null;
          hostname.clear();
          certificatePath.clear();
          privateKeyPath.clear();
        });
        await widget.status?.refresh();
      }
    }
  }

  Future<void> _cancelCertificate(BuildContext context) async {
    final strings = AppStrings.of(context);
    final confirmed = await showAppDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(strings.cancelCertificateRequest),
        content: Text(strings.confirmCancelCertificateRequest),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: Text(strings.cancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(true),
            child: Text(strings.cancelCertificateRequest),
          ),
        ],
      ),
    );
    if (confirmed != true) return;

    final canceled = await widget.controller.cancelCertificateRequest();
    if (!mounted || widget.controller.profile != null) return;
    setState(() {
      mode = 'internal';
      challenge = 'auto';
      dnsProviderId = null;
      hostname.clear();
      certificatePath.clear();
      privateKeyPath.clear();
    });
    if (canceled) await widget.controller.refresh();
    await widget.status?.refresh();
  }

  Future<void> _cancelRenewal(BuildContext context) async {
    final strings = AppStrings.of(context);
    final confirmed = await showAppDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(strings.cancelCertificateRenewal),
        content: Text(strings.confirmCancelCertificateRenewal),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: Text(strings.cancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(true),
            child: Text(strings.cancelCertificateRenewal),
          ),
        ],
      ),
    );
    if (confirmed != true) return;

    await widget.controller.cancelRenewal();
    if (mounted) {
      await widget.controller.refresh();
      await widget.status?.refresh();
    }
  }
}

class _TlsContent extends StatelessWidget {
  const _TlsContent({
    required this.controller,
    required this.status,
    required this.onOpenLogs,
    required this.strings,
    required this.mode,
    required this.hostname,
    required this.certificatePath,
    required this.privateKeyPath,
    required this.challenge,
    required this.dnsProviderId,
    required this.dnsProviders,
    required this.hasUnsavedChanges,
    required this.onModeChanged,
    required this.onChallengeChanged,
    required this.onDnsProviderChanged,
    required this.onManageDnsProviders,
    required this.onSave,
    required this.onApply,
    required this.onDisable,
    required this.onRenew,
    required this.onCancelRenewal,
    required this.onCancelCertificate,
  });

  final TlsController controller;
  final StatusController? status;
  final VoidCallback? onOpenLogs;
  final AppStrings strings;
  final String mode;
  final TextEditingController hostname;
  final TextEditingController certificatePath;
  final TextEditingController privateKeyPath;
  final String challenge;
  final String? dnsProviderId;
  final List<ManagedDnsProvider> dnsProviders;
  final bool hasUnsavedChanges;
  final ValueChanged<String> onModeChanged;
  final ValueChanged<String> onChallengeChanged;
  final ValueChanged<String?> onDnsProviderChanged;
  final VoidCallback? onManageDnsProviders;
  final VoidCallback onSave;
  final VoidCallback onApply;
  final VoidCallback onDisable;
  final VoidCallback onRenew;
  final VoidCallback onCancelRenewal;
  final VoidCallback onCancelCertificate;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final profile = controller.profile;
    final certificate = profile?.certificateStatus;
    final port = status?.serverSettings?.httpsPort;
    final pendingApply =
        controller.pendingApply || status?.status?.pendingChanges == true;
    final canRenew =
        !pendingApply &&
        !hasUnsavedChanges &&
        profile?.mode == 'automatic' &&
        certificate != null &&
        (certificate.state == 'READY' ||
            certificate.state == 'EXPIRED' ||
            (certificate.state == 'FAILED' && certificate.renewal));
    final renewalInProgress =
        !pendingApply &&
        !hasUnsavedChanges &&
        profile?.mode == 'automatic' &&
        certificate?.renewal == true &&
        certificate?.state == 'ISSUING';
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
              if (mode == 'automatic') ...[
                const Divider(height: 28),
                _SettingRow(
                  label: strings.certificateChallenge,
                  child: DropdownButtonFormField<String>(
                    key: ValueKey<String>('challenge-$challenge'),
                    initialValue: challenge,
                    decoration: InputDecoration(
                      labelText: strings.certificateChallenge,
                    ),
                    items: [
                      DropdownMenuItem(
                        value: 'auto',
                        child: Text(strings.httpChallenge),
                      ),
                      DropdownMenuItem(
                        value: 'dns',
                        child: Text(strings.dnsChallenge),
                      ),
                    ],
                    onChanged: controller.busy
                        ? null
                        : (value) {
                            if (value != null) onChallengeChanged(value);
                          },
                  ),
                ),
                if (challenge == 'dns') ...[
                  const SizedBox(height: 16),
                  _SettingRow(
                    label: strings.dnsProvider,
                    child: LayoutBuilder(
                      builder: (context, constraints) {
                        final dropdown = DropdownButtonFormField<String>(
                          key: ValueKey<String>('dns-provider-$dnsProviderId'),
                          initialValue: dnsProviderId,
                          decoration: InputDecoration(
                            labelText: strings.dnsProvider,
                          ),
                          items: dnsProviders
                              .map(
                                (provider) => DropdownMenuItem(
                                  value: provider.id,
                                  child: Text(provider.name),
                                ),
                              )
                              .toList(growable: false),
                          onChanged: controller.busy || dnsProviders.isEmpty
                              ? null
                              : onDnsProviderChanged,
                        );
                        final manageButton = OutlinedButton.icon(
                          onPressed: controller.busy
                              ? null
                              : onManageDnsProviders,
                          icon: const Icon(Icons.settings_outlined),
                          label: Text(strings.manageDnsProviders),
                        );
                        return constraints.maxWidth < 560
                            ? Column(
                                crossAxisAlignment: CrossAxisAlignment.stretch,
                                children: [
                                  dropdown,
                                  const SizedBox(height: 10),
                                  Align(
                                    alignment: Alignment.centerLeft,
                                    child: manageButton,
                                  ),
                                ],
                              )
                            : Row(
                                children: [
                                  Expanded(child: dropdown),
                                  const SizedBox(width: 12),
                                  manageButton,
                                ],
                              );
                      },
                    ),
                  ),
                  if (dnsProviders.isEmpty) ...[
                    const SizedBox(height: 8),
                    Text(
                      strings.noDnsProviders,
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: theme.colorScheme.error,
                      ),
                    ),
                  ],
                ],
              ],
              const Divider(height: 28),
              _SettingRow(
                label: strings.certificateStatus,
                value: profile == null
                    ? strings.notConfigured
                    : profile.certificateStatus == null
                    ? strings.configured
                    : strings.certificateStateLabel(
                        pendingApply
                            ? 'WAITING_FOR_APPLY'
                            : profile.certificateStatus!.state,
                      ),
                valueColor: profile == null
                    ? theme.colorScheme.onSurfaceVariant
                    : profile.certificateStatus == null
                    ? theme.colorScheme.primary
                    : _certificateStateColor(
                        theme,
                        pendingApply
                            ? 'WAITING_FOR_APPLY'
                            : profile.certificateStatus!.state,
                      ),
              ),
              if (profile?.certificateStatus case final certificate?) ...[
                const Divider(height: 28),
                _CertificateStatusDetails(
                  strings: strings,
                  status: certificate,
                  pendingApply: pendingApply,
                  onOpenLogs: onOpenLogs,
                ),
              ],
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
            if (canRenew)
              OutlinedButton.icon(
                onPressed: controller.busy ? null : onRenew,
                icon: const Icon(Icons.autorenew_outlined),
                label: Text(strings.renewCertificate),
              ),
            if (renewalInProgress)
              OutlinedButton.icon(
                onPressed: controller.busy ? null : onCancelRenewal,
                icon: const Icon(Icons.stop_circle_outlined),
                label: Text(strings.cancelCertificateRenewal),
              ),
            if (!hasUnsavedChanges &&
                profile?.mode == 'automatic' &&
                profile?.certificateStatus?.state == 'ISSUING' &&
                profile?.certificateStatus?.renewal != true)
              OutlinedButton.icon(
                onPressed: controller.busy ? null : onCancelCertificate,
                icon: const Icon(Icons.stop_circle_outlined),
                label: Text(strings.cancelCertificateRequest),
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

Color _certificateStateColor(ThemeData theme, String state) => switch (state) {
  'READY' => theme.colorScheme.primary,
  'FAILED' || 'EXPIRED' => theme.colorScheme.error,
  _ => theme.colorScheme.tertiary,
};

class _CertificateStatusDetails extends StatelessWidget {
  const _CertificateStatusDetails({
    required this.strings,
    required this.status,
    required this.pendingApply,
    required this.onOpenLogs,
  });

  final AppStrings strings;
  final ManagedCertificateStatus status;
  final bool pendingApply;
  final VoidCallback? onOpenLogs;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final state = pendingApply ? 'WAITING_FOR_APPLY' : status.state;
    final isIssuing = state == 'ISSUING';
    final isFailed = state == 'FAILED' || state == 'EXPIRED';
    final description = switch (state) {
      'WAITING_FOR_APPLY' => strings.certificateApplying,
      'WAITING_FOR_RUNTIME' => strings.certificateWaitingForRuntime,
      'ISSUING' => strings.certificateIssuing,
      'READY' => strings.certificateReady,
      'EXPIRED' => strings.certificateExpired,
      'FAILED' => strings.certificateFailed,
      _ => strings.certificateUnknown,
    };
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            Icon(
              isIssuing
                  ? Icons.sync
                  : isFailed
                  ? Icons.error_outline
                  : state == 'READY'
                  ? Icons.verified_outlined
                  : Icons.info_outline,
              color: _certificateStateColor(theme, state),
            ),
            const SizedBox(width: 10),
            Text(
              strings.certificateStateLabel(state),
              style: theme.textTheme.titleMedium?.copyWith(
                color: _certificateStateColor(theme, state),
                fontWeight: FontWeight.w700,
              ),
            ),
          ],
        ),
        const SizedBox(height: 8),
        Text(description),
        if (isIssuing) ...[
          const SizedBox(height: 12),
          const LinearProgressIndicator(),
        ],
        if (!pendingApply && status.notAfter != null) ...[
          const SizedBox(height: 8),
          Text(strings.certificateExpiresAt(status.notAfter!)),
        ],
        const SizedBox(height: 18),
        _SettingRow(
          label: strings.certificateStorageLocation,
          child: SelectableText(status.storagePath),
        ),
        if (status.certificatePath.isNotEmpty) ...[
          const Divider(height: 28),
          _SettingRow(
            label: strings.certificatePublicFile,
            child: SelectableText(status.certificatePath),
          ),
        ],
        const SizedBox(height: 10),
        Text(
          strings.certificateStorageSafety,
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
          ),
        ),
        if (isFailed && onOpenLogs != null) ...[
          const SizedBox(height: 8),
          Align(
            alignment: Alignment.centerLeft,
            child: TextButton.icon(
              onPressed: onOpenLogs,
              icon: const Icon(Icons.receipt_long_outlined),
              label: Text(strings.viewLogs),
            ),
          ),
        ],
      ],
    );
  }
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

class _DnsProviderManagerDialog extends StatefulWidget {
  const _DnsProviderManagerDialog({required this.controller});

  final TlsController controller;

  @override
  State<_DnsProviderManagerDialog> createState() =>
      _DnsProviderManagerDialogState();
}

class _DnsProviderManagerDialogState extends State<_DnsProviderManagerDialog> {
  Object? actionError;

  TlsController get controller => widget.controller;

  Future<void> _edit([ManagedDnsProvider? provider]) async {
    setState(() => actionError = null);
    await showAppDialog<bool>(
      context: context,
      builder: (context) =>
          _DnsProviderEditorDialog(controller: controller, provider: provider),
    );
  }

  Future<void> _delete(ManagedDnsProvider provider) async {
    final strings = AppStrings.of(context);
    final confirmed = await showAppDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(strings.deleteDnsProvider),
        content: Text(strings.confirmDeleteDnsProvider(provider.name)),
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
    if (confirmed != true) return;

    final success = await controller.deleteDnsProvider(provider.id);
    if (!success && mounted) {
      setState(() => actionError = controller.error);
    }
  }

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return AlertDialog(
      title: Text(strings.manageDnsProviders),
      content: SizedBox(
        width: 620,
        child: AnimatedBuilder(
          animation: controller,
          builder: (context, _) {
            final providers = controller.dnsProviders;
            return Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(
                  strings.manageDnsProvidersDescription,
                  style: Theme.of(context).textTheme.bodyMedium,
                ),
                if (actionError != null) ...[
                  const SizedBox(height: 14),
                  AppNotice(
                    icon: Icons.error_outline,
                    text: _dnsProviderError(
                      strings,
                      actionError!,
                      deleting: true,
                    ),
                    color: Theme.of(context).colorScheme.errorContainer,
                    textColor: Theme.of(context).colorScheme.onErrorContainer,
                  ),
                ],
                const SizedBox(height: 16),
                if (providers.isEmpty)
                  Padding(
                    padding: const EdgeInsets.symmetric(vertical: 24),
                    child: Text(
                      strings.noDnsProviders,
                      textAlign: TextAlign.center,
                    ),
                  )
                else
                  ConstrainedBox(
                    constraints: const BoxConstraints(maxHeight: 360),
                    child: ListView.separated(
                      shrinkWrap: true,
                      itemCount: providers.length,
                      separatorBuilder: (context, index) =>
                          const Divider(height: 1),
                      itemBuilder: (context, index) {
                        final provider = providers[index];
                        return ListTile(
                          contentPadding: EdgeInsets.zero,
                          leading: const Icon(Icons.cloud_outlined),
                          title: Text(provider.name),
                          subtitle: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                _dnsProviderLabel(strings, provider.provider),
                              ),
                              Text(
                                provider.allowedZones.isEmpty
                                    ? '*'
                                    : provider.allowedZones.join(', '),
                              ),
                              Text(
                                provider.secretConfigured
                                    ? strings.dnsProviderSecretConfigured
                                    : strings.dnsProviderSecretMissing,
                              ),
                            ],
                          ),
                          isThreeLine: true,
                          trailing: Wrap(
                            spacing: 2,
                            children: [
                              IconButton(
                                tooltip: strings.editDnsProvider,
                                onPressed: controller.busy
                                    ? null
                                    : () => _edit(provider),
                                icon: const Icon(Icons.edit_outlined),
                              ),
                              IconButton(
                                tooltip: strings.deleteDnsProvider,
                                onPressed: controller.busy
                                    ? null
                                    : () => _delete(provider),
                                icon: const Icon(Icons.delete_outline),
                              ),
                            ],
                          ),
                        );
                      },
                    ),
                  ),
              ],
            );
          },
        ),
      ),
      actions: [
        TextButton(
          onPressed: controller.busy ? null : () => Navigator.pop(context),
          child: Text(strings.close),
        ),
        FilledButton.icon(
          onPressed: controller.busy ? null : () => _edit(),
          icon: const Icon(Icons.add),
          label: Text(strings.addDnsProvider),
        ),
      ],
    );
  }
}

class _DnsProviderEditorDialog extends StatefulWidget {
  const _DnsProviderEditorDialog({required this.controller, this.provider});

  final TlsController controller;
  final ManagedDnsProvider? provider;

  @override
  State<_DnsProviderEditorDialog> createState() =>
      _DnsProviderEditorDialogState();
}

class _DnsProviderEditorDialogState extends State<_DnsProviderEditorDialog> {
  final formKey = GlobalKey<FormState>();
  late final TextEditingController name;
  late final TextEditingController zones;
  late final TextEditingController apiToken;
  late final TextEditingController secretId;
  late final TextEditingController secretKey;
  late final TextEditingController accessKeyId;
  late final TextEditingController accessKeySecret;
  late final TextEditingController securityToken;
  late String providerType;
  bool submitting = false;
  Object? submitError;

  ManagedDnsProvider? get existing => widget.provider;

  @override
  void initState() {
    super.initState();
    final provider = existing;
    providerType = provider?.provider ?? 'cloudflare';
    name = TextEditingController(text: provider?.name);
    zones = TextEditingController(text: provider?.allowedZones.join('\n'));
    apiToken = TextEditingController();
    secretId = TextEditingController();
    secretKey = TextEditingController();
    accessKeyId = TextEditingController();
    accessKeySecret = TextEditingController();
    securityToken = TextEditingController();
  }

  @override
  void dispose() {
    name.dispose();
    zones.dispose();
    apiToken.dispose();
    secretId.dispose();
    secretKey.dispose();
    accessKeyId.dispose();
    accessKeySecret.dispose();
    securityToken.dispose();
    super.dispose();
  }

  List<TextEditingController> get _secretFields => switch (providerType) {
    'cloudflare' => [apiToken],
    'tencentcloud' => [secretId, secretKey],
    'dnspod' => [apiToken],
    'alidns' => [accessKeyId, accessKeySecret, securityToken],
    _ => const [],
  };

  bool get _hasSecretInput =>
      _secretFields.any((field) => field.text.isNotEmpty);

  bool get _canKeepExistingSecret =>
      existing != null &&
      existing!.provider == providerType &&
      existing!.secretConfigured &&
      !_hasSecretInput;

  String? _requiredSecret(String? value) {
    if (_canKeepExistingSecret) return null;
    return value == null || value.trim().isEmpty
        ? AppStrings.of(context).dnsProviderSecretRequired
        : null;
  }

  Map<String, String>? _secretValues() {
    final values = <String, String>{};
    switch (providerType) {
      case 'cloudflare':
        if (apiToken.text.isNotEmpty) values['api_token'] = apiToken.text;
      case 'tencentcloud':
        if (secretId.text.isNotEmpty) values['secret_id'] = secretId.text;
        if (secretKey.text.isNotEmpty) values['secret_key'] = secretKey.text;
      case 'dnspod':
        if (apiToken.text.isNotEmpty) values['api_token'] = apiToken.text;
      case 'alidns':
        if (accessKeyId.text.isNotEmpty) {
          values['access_key_id'] = accessKeyId.text;
        }
        if (accessKeySecret.text.isNotEmpty) {
          values['access_key_secret'] = accessKeySecret.text;
        }
        if (securityToken.text.isNotEmpty) {
          values['security_token'] = securityToken.text;
        }
    }
    if (values.isEmpty && existing != null) return null;
    return values;
  }

  List<String> _allowedZones() => zones.text
      .split(RegExp(r'[\s,]+'))
      .map((zone) => zone.trim())
      .where((zone) => zone.isNotEmpty)
      .toList(growable: false);

  Future<void> _submit() async {
    if (!formKey.currentState!.validate()) return;
    setState(() {
      submitting = true;
      submitError = null;
    });
    final success = await widget.controller.saveDnsProvider(
      id: existing?.id,
      name: name.text.trim(),
      provider: providerType,
      allowedZones: _allowedZones(),
      secret: _secretValues(),
    );
    if (!mounted) return;
    if (success) {
      Navigator.pop(context, true);
    } else {
      setState(() {
        submitting = false;
        submitError = widget.controller.error;
      });
    }
  }

  void _changeProvider(String value) {
    if (value == providerType) return;
    for (final field in [
      apiToken,
      secretId,
      secretKey,
      accessKeyId,
      accessKeySecret,
      securityToken,
    ]) {
      field.clear();
    }
    setState(() {
      providerType = value;
      submitError = null;
    });
  }

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return AlertDialog(
      title: Text(
        existing == null ? strings.addDnsProvider : strings.editDnsProvider,
      ),
      content: SizedBox(
        width: 560,
        child: Form(
          key: formKey,
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                TextFormField(
                  controller: name,
                  autofocus: true,
                  enabled: !submitting,
                  decoration: InputDecoration(
                    labelText: strings.dnsProviderName,
                  ),
                  validator: (value) => value == null || value.trim().isEmpty
                      ? strings.dnsProviderNameRequired
                      : null,
                ),
                const SizedBox(height: 14),
                DropdownButtonFormField<String>(
                  initialValue: providerType,
                  isExpanded: true,
                  decoration: InputDecoration(
                    labelText: strings.dnsProviderType,
                  ),
                  items:
                      [
                            ('cloudflare', strings.providerCloudflare),
                            ('tencentcloud', strings.providerTencentCloud),
                            ('dnspod', strings.providerDnsPod),
                            ('alidns', strings.providerAliDns),
                          ]
                          .map(
                            (entry) => DropdownMenuItem(
                              value: entry.$1,
                              child: Text(entry.$2),
                            ),
                          )
                          .toList(growable: false),
                  onChanged: submitting
                      ? null
                      : (value) => _changeProvider(value!),
                ),
                const SizedBox(height: 14),
                TextFormField(
                  controller: zones,
                  enabled: !submitting,
                  minLines: 2,
                  maxLines: 4,
                  decoration: InputDecoration(
                    labelText: strings.dnsProviderZones,
                    hintText: strings.dnsProviderZonesHint,
                    helperText: strings.dnsProviderZonesHint,
                  ),
                ),
                const SizedBox(height: 18),
                Text(
                  strings.dnsProviderSecret,
                  style: Theme.of(context).textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.w700,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  strings.dnsProviderSecretHint,
                  style: Theme.of(context).textTheme.bodySmall,
                ),
                if (providerType == 'tencentcloud') ...[
                  const SizedBox(height: 6),
                  Text(
                    strings.dnsProviderTencentCloudHint,
                    style: Theme.of(context).textTheme.bodySmall?.copyWith(
                      color: Theme.of(context).colorScheme.primary,
                    ),
                  ),
                ],
                if (providerType == 'dnspod') ...[
                  const SizedBox(height: 6),
                  Text(
                    strings.dnsProviderDnsPodHint,
                    style: Theme.of(context).textTheme.bodySmall?.copyWith(
                      color: Theme.of(context).colorScheme.primary,
                    ),
                  ),
                ],
                const SizedBox(height: 10),
                ..._secretInputs(strings),
                if (submitError != null) ...[
                  const SizedBox(height: 14),
                  AppNotice(
                    icon: Icons.error_outline,
                    text: _dnsProviderError(strings, submitError!),
                    color: Theme.of(context).colorScheme.errorContainer,
                    textColor: Theme.of(context).colorScheme.onErrorContainer,
                  ),
                ],
              ],
            ),
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: submitting ? null : () => Navigator.pop(context),
          child: Text(strings.cancel),
        ),
        FilledButton(
          onPressed: submitting ? null : _submit,
          child: Text(strings.save),
        ),
      ],
    );
  }

  List<Widget> _secretInputs(AppStrings strings) => switch (providerType) {
    'cloudflare' => [
      TextFormField(
        controller: apiToken,
        enabled: !submitting,
        obscureText: true,
        decoration: InputDecoration(labelText: strings.apiToken),
        validator: _requiredSecret,
      ),
    ],
    'tencentcloud' => [
      TextFormField(
        controller: secretId,
        enabled: !submitting,
        obscureText: true,
        decoration: InputDecoration(labelText: strings.secretId),
        validator: _requiredSecret,
      ),
      const SizedBox(height: 12),
      TextFormField(
        controller: secretKey,
        enabled: !submitting,
        obscureText: true,
        decoration: InputDecoration(labelText: strings.secretKey),
        validator: _requiredSecret,
      ),
    ],
    'dnspod' => [
      TextFormField(
        controller: apiToken,
        enabled: !submitting,
        obscureText: true,
        decoration: InputDecoration(labelText: strings.dnsPodApiToken),
        validator: _requiredSecret,
      ),
    ],
    'alidns' => [
      TextFormField(
        controller: accessKeyId,
        enabled: !submitting,
        obscureText: true,
        decoration: InputDecoration(labelText: strings.accessKeyId),
        validator: _requiredSecret,
      ),
      const SizedBox(height: 12),
      TextFormField(
        controller: accessKeySecret,
        enabled: !submitting,
        obscureText: true,
        decoration: InputDecoration(labelText: strings.accessKeySecret),
        validator: _requiredSecret,
      ),
      const SizedBox(height: 12),
      TextFormField(
        controller: securityToken,
        enabled: !submitting,
        obscureText: true,
        decoration: InputDecoration(labelText: strings.securityToken),
      ),
    ],
    _ => const [],
  };
}

String _dnsProviderLabel(AppStrings strings, String provider) =>
    switch (provider) {
      'cloudflare' => strings.providerCloudflare,
      'tencentcloud' => strings.providerTencentCloud,
      'dnspod' => strings.providerDnsPod,
      'alidns' => strings.providerAliDns,
      _ => provider,
    };

String _dnsProviderError(
  AppStrings strings,
  Object error, {
  bool deleting = false,
}) {
  if (error is DaemonApiException) {
    return switch (error.code) {
      'DNS_PROVIDER_IN_USE' => strings.dnsProviderDeleteFailed,
      'DNS_PROVIDER_SECRET_MISSING' ||
      'INVALID_DNS_PROVIDER_SECRET' => strings.dnsProviderSecretRequired,
      _ =>
        deleting
            ? strings.dnsProviderDeleteFailed
            : strings.dnsProviderSaveFailed,
    };
  }
  return deleting
      ? strings.dnsProviderDeleteFailed
      : strings.dnsProviderSaveFailed;
}
