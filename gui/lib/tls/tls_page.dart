import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/state/tls_controller.dart';
import 'package:flutter/material.dart';

class TlsPage extends StatefulWidget {
  const TlsPage({super.key, required this.controller});
  final TlsController controller;

  @override
  State<TlsPage> createState() => _TlsPageState();
}

class _TlsPageState extends State<TlsPage> {
  final hostname = TextEditingController();
  final certificatePath = TextEditingController();
  final privateKeyPath = TextEditingController();
  String mode = 'internal';
  Object? syncedProfile;

  @override
  void dispose() {
    hostname.dispose();
    certificatePath.dispose();
    privateKeyPath.dispose();
    super.dispose();
  }

  void syncProfile() {
    final profile = widget.controller.profile;
    if (profile == null || identical(profile, syncedProfile)) return;
    syncedProfile = profile;
    mode = profile.mode;
    hostname.text = profile.hostname;
    certificatePath.text = profile.certificatePath;
    privateKeyPath.text = profile.privateKeyPath;
  }

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return Scaffold(
      appBar: AppBar(title: Text(strings.https)),
      body: AnimatedBuilder(
        animation: widget.controller,
        builder: (context, _) {
          syncProfile();
          if (widget.controller.loading) {
            return const Center(child: CircularProgressIndicator());
          }
          return Stack(
            children: [
              SingleChildScrollView(
                padding: const EdgeInsets.all(24),
                child: Align(
                  alignment: Alignment.topCenter,
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(maxWidth: 760),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        Text(
                          strings.httpsWizardTitle,
                          style: Theme.of(context).textTheme.headlineSmall,
                        ),
                        const SizedBox(height: 8),
                        Text(strings.httpsWizardIntro),
                        const SizedBox(height: 24),
                        SegmentedButton<String>(
                          segments: [
                            ButtonSegment(
                              value: 'automatic',
                              icon: const Icon(Icons.public),
                              label: Text(strings.tlsAutomatic),
                            ),
                            ButtonSegment(
                              value: 'internal',
                              icon: const Icon(Icons.lan),
                              label: Text(strings.tlsInternal),
                            ),
                            ButtonSegment(
                              value: 'custom',
                              icon: const Icon(Icons.badge_outlined),
                              label: Text(strings.tlsCustom),
                            ),
                          ],
                          selected: {mode},
                          onSelectionChanged: widget.controller.busy
                              ? null
                              : (selection) =>
                                    setState(() => mode = selection.single),
                        ),
                        const SizedBox(height: 16),
                        Card(
                          child: Padding(
                            padding: const EdgeInsets.all(16),
                            child: Text(_modeDescription(strings)),
                          ),
                        ),
                        if (mode == 'internal') ...[
                          const SizedBox(height: 12),
                          _Notice(
                            icon: Icons.info_outline,
                            text: strings.internalTrustWarning,
                          ),
                        ],
                        const SizedBox(height: 20),
                        TextField(
                          controller: hostname,
                          enabled: !widget.controller.busy,
                          decoration: InputDecoration(
                            labelText: strings.hostname,
                            hintText: mode == 'automatic'
                                ? 'dav.example.com'
                                : 'dav.local',
                            border: const OutlineInputBorder(),
                          ),
                        ),
                        if (mode == 'custom') ...[
                          const SizedBox(height: 16),
                          TextField(
                            controller: certificatePath,
                            enabled: !widget.controller.busy,
                            decoration: InputDecoration(
                              labelText: strings.certificatePath,
                              border: const OutlineInputBorder(),
                            ),
                          ),
                          const SizedBox(height: 16),
                          TextField(
                            controller: privateKeyPath,
                            enabled: !widget.controller.busy,
                            decoration: InputDecoration(
                              labelText: strings.privateKeyPath,
                              helperText: strings.privateKeyPathSafety,
                              border: const OutlineInputBorder(),
                            ),
                          ),
                        ],
                        const SizedBox(height: 20),
                        Wrap(
                          spacing: 12,
                          runSpacing: 12,
                          children: [
                            FilledButton.icon(
                              onPressed: widget.controller.busy ? null : _save,
                              icon: const Icon(Icons.save_outlined),
                              label: Text(strings.saveTlsSettings),
                            ),
                            OutlinedButton.icon(
                              onPressed:
                                  widget.controller.busy ||
                                      widget.controller.profile == null
                                  ? null
                                  : widget.controller.check,
                              icon: const Icon(Icons.fact_check_outlined),
                              label: Text(strings.runPreflight),
                            ),
                            if (widget.controller.pendingApply)
                              FilledButton.tonalIcon(
                                onPressed: widget.controller.busy
                                    ? null
                                    : widget.controller.apply,
                                icon: const Icon(Icons.rocket_launch_outlined),
                                label: Text(strings.applyConfiguration),
                              ),
                            if (widget.controller.error != null &&
                                widget.controller.profile == null)
                              OutlinedButton(
                                onPressed: widget.controller.busy
                                    ? null
                                    : widget.controller.refresh,
                                child: Text(strings.retry),
                              ),
                          ],
                        ),
                        if (widget.controller.pendingApply) ...[
                          const SizedBox(height: 16),
                          _Notice(
                            icon: Icons.pending_actions,
                            text: strings.pendingTlsApply,
                          ),
                        ],
                        if (widget.controller.error case final error?) ...[
                          const SizedBox(height: 16),
                          _Notice(
                            icon: Icons.error_outline,
                            text: error.toString(),
                            error: true,
                          ),
                        ],
                        if (widget.controller.checkResult
                            case final result?) ...[
                          const SizedBox(height: 20),
                          Text(
                            result.ready
                                ? strings.preflightReady
                                : strings.preflightFailed,
                            style: Theme.of(context).textTheme.titleMedium,
                          ),
                          const SizedBox(height: 8),
                          ...result.checks.map(
                            (check) => ListTile(
                              contentPadding: EdgeInsets.zero,
                              leading: Icon(
                                check.ok ? Icons.check_circle : Icons.error,
                                color: check.ok
                                    ? Colors.green
                                    : Theme.of(context).colorScheme.error,
                              ),
                              title: Text(check.name),
                              subtitle: Text(check.message),
                            ),
                          ),
                        ],
                      ],
                    ),
                  ),
                ),
              ),
              if (widget.controller.busy) const LinearProgressIndicator(),
            ],
          );
        },
      ),
    );
  }

  String _modeDescription(AppStrings strings) => switch (mode) {
    'automatic' => strings.automaticTlsDescription,
    'custom' => strings.customTlsDescription,
    _ => strings.internalTlsDescription,
  };

  Future<void> _save() async {
    final custom = mode == 'custom';
    await widget.controller.configure(
      mode: mode,
      hostname: hostname.text,
      certificatePath: custom ? certificatePath.text : '',
      privateKeyPath: custom ? privateKeyPath.text : '',
    );
  }
}

class _Notice extends StatelessWidget {
  const _Notice({required this.icon, required this.text, this.error = false});
  final IconData icon;
  final String text;
  final bool error;

  @override
  Widget build(BuildContext context) {
    final color = error
        ? Theme.of(context).colorScheme.errorContainer
        : Theme.of(context).colorScheme.secondaryContainer;
    return DecoratedBox(
      decoration: BoxDecoration(
        color: color,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Row(
          children: [
            Icon(icon),
            const SizedBox(width: 12),
            Expanded(child: Text(text)),
          ],
        ),
      ),
    );
  }
}
