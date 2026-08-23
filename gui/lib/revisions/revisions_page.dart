import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/state/revision_controller.dart';
import 'package:flutter/material.dart';

class RevisionsPage extends StatelessWidget {
  const RevisionsPage({super.key, required this.controller});

  final RevisionController controller;

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return Scaffold(
      appBar: AppBar(
        title: Text(strings.revisions),
        actions: [
          IconButton(
            tooltip: strings.refreshRevisions,
            onPressed: controller.state == RevisionLoadState.loading
                ? null
                : controller.refresh,
            icon: const Icon(Icons.refresh),
          ),
        ],
      ),
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
              ListView(
                padding: const EdgeInsets.all(24),
                children: [
                  if (controller.configuration case final state?)
                    _StateCard(state: state, strings: strings),
                  if (controller.configuration != null)
                    const SizedBox(height: 12),
                  if (controller.revisions.isEmpty)
                    Center(child: Text(strings.noRevisions))
                  else
                    ...controller.revisions.map(
                      (revision) => _RevisionCard(
                        revision: revision,
                        restoring: controller.restoringId == revision.id,
                        onRestore: revision.validationStatus == 'VALID'
                            ? () => _confirmRestore(
                                context,
                                controller,
                                revision,
                                strings,
                              )
                            : null,
                        strings: strings,
                      ),
                    ),
                  if (controller.error != null &&
                      controller.state == RevisionLoadState.ready) ...[
                    const SizedBox(height: 12),
                    _ErrorNotice(message: controller.error.toString()),
                  ],
                ],
              ),
              if (controller.restoringId != null)
                const LinearProgressIndicator(),
            ],
          );
        },
      ),
    );
  }

  Future<void> _confirmRestore(
    BuildContext context,
    RevisionController controller,
    ManagedRevision revision,
    AppStrings strings,
  ) async {
    final confirmed = await showDialog<bool>(
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
    if (confirmed == true) {
      await controller.restore(revision);
    }
  }
}

class _StateCard extends StatelessWidget {
  const _StateCard({required this.state, required this.strings});

  final ManagedRevisionState state;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) => Card(
    child: ListTile(
      leading: Icon(
        state.pending ? Icons.pending_actions : Icons.check_circle,
        color: state.pending ? Colors.orange : Colors.green,
      ),
      title: Text(strings.configurationState),
      subtitle: Text(
        '${strings.desiredRevision}: ${state.desiredRevision ?? strings.none} · '
        '${strings.activeRevision}: ${state.activeRevision ?? strings.none}',
      ),
      trailing: Text(state.pending ? strings.pending : strings.applied),
    ),
  );
}

class _RevisionCard extends StatelessWidget {
  const _RevisionCard({
    required this.revision,
    required this.restoring,
    required this.onRestore,
    required this.strings,
  });

  final ManagedRevision revision;
  final bool restoring;
  final VoidCallback? onRestore;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) => Card(
    child: Padding(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  '${strings.revision} ${revision.number}',
                  style: Theme.of(context).textTheme.titleMedium,
                ),
              ),
              Chip(label: Text(revision.applyStatus)),
            ],
          ),
          Text('${strings.validation}: ${revision.validationStatus}'),
          Text('${strings.created}: ${revision.createdAt}'),
          SelectableText('${strings.configHash}: ${revision.configHash}'),
          if (revision.errorCode != null)
            Text(
              '${revision.errorCode}: ${revision.errorSummary ?? ''}',
              style: TextStyle(color: Theme.of(context).colorScheme.error),
            ),
          if (onRestore != null) ...[
            const SizedBox(height: 8),
            Align(
              alignment: Alignment.centerRight,
              child: FilledButton.tonalIcon(
                onPressed: restoring ? null : onRestore,
                icon: const Icon(Icons.restore),
                label: Text(restoring ? strings.restoring : strings.restore),
              ),
            ),
          ],
        ],
      ),
    ),
  );
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

class _ErrorNotice extends StatelessWidget {
  const _ErrorNotice({required this.message});

  final String message;

  @override
  Widget build(BuildContext context) => ListTile(
    leading: Icon(
      Icons.error_outline,
      color: Theme.of(context).colorScheme.error,
    ),
    title: Text(
      message,
      style: TextStyle(color: Theme.of(context).colorScheme.error),
    ),
  );
}
