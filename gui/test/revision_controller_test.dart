import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/state/revision_controller.dart';
import 'package:flutter_test/flutter_test.dart';

class FakeRevisionApi implements RevisionApi {
  const FakeRevisionApi({this.failRestore = false});

  final bool failRestore;

  @override
  Future<ManagedRevisionState> configurationState() async =>
      const ManagedRevisionState(
        desiredRevision: 2,
        activeRevision: 1,
        pending: true,
      );

  @override
  Future<List<ManagedRevision>> listRevisions() async => [
    const ManagedRevision(
      id: 'revision-2',
      number: 2,
      createdAt: '2026-08-23T01:02:03Z',
      configHash: 'hash-2',
      validationStatus: 'VALID',
      applyStatus: 'APPLIED',
      appVersion: 'test',
    ),
    const ManagedRevision(
      id: 'revision-1',
      number: 1,
      createdAt: '2026-08-23T01:01:03Z',
      configHash: 'hash-1',
      validationStatus: 'INVALID',
      applyStatus: 'FAILED',
      appVersion: 'test',
      errorCode: 'CADDY_VALIDATE_FAILED',
      errorSummary: 'invalid configuration',
    ),
  ];

  @override
  Future<ManagedRevision> applyConfigurationResult() async =>
      (await listRevisions()).first;

  @override
  Future<ManagedRevision> restoreRevision(String id) async {
    if (failRestore) {
      throw const DaemonApiException('CADDY_RELOAD_FAILED', 'restore failed');
    }
    return (await listRevisions()).first;
  }

  @override
  Future<void> deleteRevision(String id) async {}
}

void main() {
  test(
    'revision controller loads state and restores valid revisions',
    () async {
      var restoredCallbackCalled = false;
      final controller = RevisionController(
        const FakeRevisionApi(),
        onRestored: () async => restoredCallbackCalled = true,
      );
      addTearDown(controller.dispose);

      await controller.refresh();
      expect(controller.state, RevisionLoadState.ready);
      expect(controller.configuration?.pending, isTrue);
      expect(controller.revisions, hasLength(2));

      expect(await controller.restore(controller.revisions.first), isTrue);
      expect(controller.error, isNull);
      expect(restoredCallbackCalled, isTrue);
    },
  );

  test(
    'revision controller rejects restore errors without hiding them',
    () async {
      final controller = RevisionController(
        const FakeRevisionApi(failRestore: true),
      );
      addTearDown(controller.dispose);
      await controller.refresh();

      expect(await controller.restore(controller.revisions.first), isFalse);
      expect(controller.error, isA<DaemonApiException>());
      expect(controller.error.toString(), contains('CADDY_RELOAD_FAILED'));
    },
  );
}
