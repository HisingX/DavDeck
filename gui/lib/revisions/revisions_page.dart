import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/state/revision_controller.dart';
import 'package:davdeck/widgets/app_ui.dart';
import 'package:flutter/material.dart';

class RevisionsPage extends StatelessWidget {
  const RevisionsPage({super.key, required this.controller});

  final RevisionController controller;

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return Scaffold(
      body: AnimatedBuilder(
        animation: controller,
        builder: (context, _) {
          if (controller.state == RevisionLoadState.loading &&
              controller.revisions.isEmpty) {
            return Center(child: Text(strings.revisionsLoading));
          }
          if (controller.state == RevisionLoadState.error &&
              controller.revisions.isEmpty) {
            return _ErrorState(
              message: controller.error.toString(),
              retry: controller.refresh,
              strings: strings,
            );
          }
          return Stack(
            children: [
              SingleChildScrollView(
                padding: appPagePadding(context),
                child: Center(
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(maxWidth: 1120),
                    child: _RevisionsContent(
                      controller: controller,
                      strings: strings,
                      onRestore: _confirmRestore,
                      onDelete: _confirmDelete,
                    ),
                  ),
                ),
              ),
              if (controller.restoringId != null)
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

  Future<void> _confirmRestore(
    BuildContext context,
    ManagedRevision revision,
    AppStrings strings,
  ) async {
    final confirmed = await showAppDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(strings.restoreRevision),
        content: Text(strings.confirmRestoreRevision(revision.number)),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: Text(strings.cancel),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: Text(strings.restore),
          ),
        ],
      ),
    );
    if (confirmed == true) await controller.restore(revision);
  }

  Future<void> _confirmDelete(
    BuildContext context,
    ManagedRevision revision,
    AppStrings strings,
  ) async {
    final confirmed = await showAppDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(strings.deleteRevision),
        content: Text(strings.confirmDeleteRevision(revision.number)),
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
    if (confirmed == true) await controller.delete(revision);
  }
}

class _RevisionsContent extends StatelessWidget {
  const _RevisionsContent({
    required this.controller,
    required this.strings,
    required this.onRestore,
    required this.onDelete,
  });

  final RevisionController controller;
  final AppStrings strings;
  final Future<void> Function(
    BuildContext context,
    ManagedRevision revision,
    AppStrings strings,
  )
  onRestore;
  final Future<void> Function(
    BuildContext context,
    ManagedRevision revision,
    AppStrings strings,
  )
  onDelete;

  @override
  Widget build(BuildContext context) {
    final state = controller.configuration;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        AppPageHeader(
          title: strings.revisions,
          subtitle: strings.revisionsSubtitle,
          actions: IconButton(
            tooltip: strings.refreshRevisions,
            onPressed: controller.state == RevisionLoadState.loading
                ? null
                : controller.refresh,
            icon: const Icon(Icons.refresh),
          ),
        ),
        const SizedBox(height: 24),
        if (state != null) ...[
          _ConfigurationCard(state: state, strings: strings),
          const SizedBox(height: 24),
        ],
        Row(
          children: [
            Text(
              strings.revisionHistory,
              style: Theme.of(
                context,
              ).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w700),
            ),
            const Spacer(),
            Text(
              strings.revisionsCount(controller.revisions.length),
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
        const SizedBox(height: 12),
        if (controller.revisions.isEmpty)
          AppSurface(
            padding: const EdgeInsets.symmetric(vertical: 42),
            child: Center(child: Text(strings.noRevisions)),
          )
        else
          ...controller.revisions.map(
            (revision) => Padding(
              padding: const EdgeInsets.only(bottom: 12),
              child: _RevisionCard(
                revision: revision,
                activeRevision: state?.activeRevision,
                desiredRevision: state?.desiredRevision,
                restoring: controller.restoringId == revision.id,
                deleting: controller.deletingId == revision.id,
                onRestore:
                    revision.validationStatus == 'VALID' &&
                        revision.stateSnapshotAvailable
                    ? () => onRestore(context, revision, strings)
                    : null,
                onDelete:
                    revision.number == state?.activeRevision ||
                        revision.number == state?.desiredRevision
                    ? null
                    : () => onDelete(context, revision, strings),
                strings: strings,
              ),
            ),
          ),
        if (controller.error != null &&
            controller.state == RevisionLoadState.ready) ...[
          const SizedBox(height: 4),
          AppNotice(
            icon: Icons.error_outline,
            text: controller.error.toString(),
            color: Theme.of(context).colorScheme.errorContainer,
            textColor: Theme.of(context).colorScheme.onErrorContainer,
          ),
        ],
      ],
    );
  }
}

class _ConfigurationCard extends StatelessWidget {
  const _ConfigurationCard({required this.state, required this.strings});

  final ManagedRevisionState state;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final color = state.pending
        ? const Color(0xffb87800)
        : theme.colorScheme.primary;
    return AppSurface(
      padding: const EdgeInsets.fromLTRB(22, 18, 22, 18),
      child: LayoutBuilder(
        builder: (context, constraints) {
          final compact = constraints.maxWidth < 650;
          final icon = Container(
            width: 54,
            height: 54,
            decoration: BoxDecoration(color: color, shape: BoxShape.circle),
            child: Icon(
              state.pending ? Icons.pending_actions : Icons.check,
              color: Colors.white,
            ),
          );
          final copy = Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                strings.configurationState,
                style: theme.textTheme.titleLarge?.copyWith(
                  fontWeight: FontWeight.w700,
                ),
              ),
              const SizedBox(height: 6),
              Text(
                '${strings.desiredRevision}: ${state.desiredRevision ?? strings.none} · '
                '${strings.activeRevision}: ${state.activeRevision ?? strings.none}',
              ),
            ],
          );
          final pill = AppStatusPill(
            label: state.pending ? strings.pending : strings.applied,
            color: color,
          );
          if (compact) {
            return Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(children: [icon, const SizedBox(width: 14), copy]),
                const SizedBox(height: 14),
                pill,
              ],
            );
          }
          return Row(
            children: [
              icon,
              const SizedBox(width: 16),
              copy,
              const Spacer(),
              pill,
            ],
          );
        },
      ),
    );
  }
}

class _RevisionCard extends StatelessWidget {
  const _RevisionCard({
    required this.revision,
    required this.activeRevision,
    required this.desiredRevision,
    required this.restoring,
    required this.deleting,
    required this.onRestore,
    required this.onDelete,
    required this.strings,
  });

  final ManagedRevision revision;
  final int? activeRevision;
  final int? desiredRevision;
  final bool restoring;
  final bool deleting;
  final VoidCallback? onRestore;
  final AppStrings strings;
  final VoidCallback? onDelete;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final active = activeRevision == revision.number;
    final desired = desiredRevision == revision.number;
    final validationColor = appStatusColor(context, revision.validationStatus);
    final applyColor = appStatusColor(context, revision.applyStatus);
    return AppSurface(
      padding: const EdgeInsets.fromLTRB(18, 17, 18, 17),
      color: active
          ? theme.colorScheme.primaryContainer.withValues(alpha: 0.18)
          : null,
      child: LayoutBuilder(
        builder: (context, constraints) {
          final compact = constraints.maxWidth < 680;
          final identity = Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                width: 54,
                height: 54,
                decoration: BoxDecoration(
                  color: theme.colorScheme.primaryContainer,
                  shape: BoxShape.circle,
                ),
                alignment: Alignment.center,
                child: Text(
                  '${revision.number}',
                  style: theme.textTheme.titleMedium?.copyWith(
                    color: theme.colorScheme.onPrimaryContainer,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ),
              const SizedBox(width: 14),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Wrap(
                      spacing: 8,
                      runSpacing: 6,
                      crossAxisAlignment: WrapCrossAlignment.center,
                      children: [
                        Text(
                          '${strings.revision} ${revision.number}',
                          style: theme.textTheme.titleMedium?.copyWith(
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                        if (active)
                          AppStatusPill(
                            label: strings.currentRevision,
                            color: theme.colorScheme.primary,
                          ),
                        if (desired)
                          AppStatusPill(
                            label: strings.desiredRevision,
                            color: theme.colorScheme.secondary,
                          ),
                      ],
                    ),
                    const SizedBox(height: 8),
                    Wrap(
                      spacing: 14,
                      runSpacing: 8,
                      children: [
                        _RevisionMeta(
                          icon: Icons.check_circle_outline,
                          text:
                              '${strings.validation}: ${revision.validationStatus}',
                          color: validationColor,
                        ),
                        _RevisionMeta(
                          icon: Icons.calendar_today_outlined,
                          text: '${strings.created}: ${revision.createdAt}',
                        ),
                      ],
                    ),
                    const SizedBox(height: 8),
                    _RevisionMeta(
                      icon: Icons.sell_outlined,
                      text: '${strings.configHash}: ${revision.configHash}',
                    ),
                    if (!revision.stateSnapshotAvailable) ...[
                      const SizedBox(height: 8),
                      _RevisionMeta(
                        icon: Icons.warning_amber_outlined,
                        text: strings.revisionStateUnavailable,
                        color: theme.colorScheme.error,
                      ),
                    ],
                  ],
                ),
              ),
            ],
          );
          final actions = Column(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              AppStatusPill(label: revision.applyStatus, color: applyColor),
              if (onRestore != null) ...[
                const SizedBox(height: 10),
                OutlinedButton.icon(
                  onPressed: restoring ? null : onRestore,
                  icon: const Icon(Icons.restore),
                  label: Text(restoring ? strings.restoring : strings.restore),
                ),
              ],
              if (onDelete != null) ...[
                const SizedBox(height: 10),
                OutlinedButton.icon(
                  onPressed: deleting ? null : onDelete,
                  icon: const Icon(Icons.delete_outline),
                  label: Text(deleting ? strings.deleting : strings.delete),
                ),
              ],
            ],
          );
          if (compact) {
            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                identity,
                const SizedBox(height: 14),
                Align(alignment: Alignment.centerRight, child: actions),
              ],
            );
          }
          return Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(child: identity),
              const SizedBox(width: 18),
              actions,
            ],
          );
        },
      ),
    );
  }
}

class _RevisionMeta extends StatelessWidget {
  const _RevisionMeta({required this.icon, required this.text, this.color});

  final IconData icon;
  final String text;
  final Color? color;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(
          icon,
          size: 16,
          color: color ?? theme.colorScheme.onSurfaceVariant,
        ),
        const SizedBox(width: 6),
        Flexible(
          fit: FlexFit.loose,
          child: Text(
            text,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: theme.textTheme.bodySmall?.copyWith(color: color),
          ),
        ),
      ],
    );
  }
}

class _ErrorState extends StatelessWidget {
  const _ErrorState({
    required this.message,
    required this.retry,
    required this.strings,
  });

  final String message;
  final VoidCallback retry;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) => Center(
    child: Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(strings.revisionsUnavailable),
        const SizedBox(height: 8),
        Text(message),
        const SizedBox(height: 12),
        FilledButton(onPressed: retry, child: Text(strings.retry)),
      ],
    ),
  );
}
