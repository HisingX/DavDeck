import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/state/logs_controller.dart';
import 'package:flutter_test/flutter_test.dart';

class FakeLogsApi implements LogsApi {
  final calls = <({int? cursor, String? level, String? component})>[];
  bool fail = false;

  @override
  Future<ManagedLogPage> logs({
    int limit = 100,
    int? cursor,
    DateTime? since,
    String? level,
    String? component,
  }) async {
    calls.add((cursor: cursor, level: level, component: component));
    if (fail) throw const DaemonApiException('LOGS_UNAVAILABLE', 'offline');
    if (cursor == null) {
      return ManagedLogPage(
        records: [
          ManagedLogRecord(
            id: 3,
            timestamp: DateTime.utc(2026, 8, 23, 1, 2, 3),
            level: 'ERROR',
            component: 'caddy',
            message: 'safe failure',
            fields: const {'error_code': 'CADDY_START_FAILED'},
          ),
          ManagedLogRecord(
            id: 2,
            timestamp: DateTime.utc(2026, 8, 23, 1, 2, 2),
            level: 'INFO',
            component: 'daemon',
            message: 'started',
          ),
        ],
        nextCursor: 2,
        hasMore: true,
      );
    }
    return ManagedLogPage(
      records: [
        ManagedLogRecord(
          id: 1,
          timestamp: DateTime.utc(2026, 8, 23, 1, 2, 1),
          level: 'WARN',
          component: 'platform',
          message: 'warning',
        ),
      ],
    );
  }
}

void main() {
  test('logs controller refreshes, filters, and paginates', () async {
    final api = FakeLogsApi();
    final controller = LogsController(api);
    addTearDown(controller.dispose);

    await controller.refresh();
    expect(controller.state, LogsLoadState.ready);
    expect(controller.records, hasLength(2));
    expect(controller.hasMore, isTrue);

    await controller.loadMore();
    expect(controller.records.map((record) => record.id), [3, 2, 1]);
    expect(controller.hasMore, isFalse);

    await controller.setLevelFilter('ERROR');
    await controller.setComponentFilter(' caddy ');
    expect(api.calls.last, (cursor: null, level: 'ERROR', component: 'caddy'));
  });

  test('refresh keeps visible records when a later request fails', () async {
    final api = FakeLogsApi();
    final controller = LogsController(api);
    addTearDown(controller.dispose);

    await controller.refresh();
    api.fail = true;
    await controller.refresh();

    expect(controller.records, hasLength(2));
    expect(controller.state, LogsLoadState.ready);
    expect(controller.error, isA<DaemonApiException>());
  });

  test('copy/export text applies a second redaction boundary', () async {
    final api = FakeLogsApi();
    final controller = LogsController(api);
    addTearDown(controller.dispose);
    await controller.refresh();
    controller.records = [
      ManagedLogRecord(
        id: 4,
        timestamp: DateTime.utc(2026, 8, 23),
        level: 'ERROR',
        component: 'runtime',
        message: 'password=secret',
        fields: const {'management_token': 'token-secret', 'safe': 'value'},
      ),
    ];

    final exported = controller.exportText();
    expect(exported, contains('[REDACTED]'));
    expect(exported, isNot(contains('secret')));
    expect(exported, isNot(contains('token-secret')));
    expect(exported, contains('value'));
  });

  test(
    'clearing the view preserves the daemon snapshot for a later refresh',
    () async {
      final api = FakeLogsApi();
      final controller = LogsController(api);
      addTearDown(controller.dispose);

      await controller.refresh();
      final json = controller.recordJson(controller.records.first);
      expect(json, contains('safe failure'));

      controller.clearView();

      expect(controller.records, isEmpty);
      expect(controller.state, LogsLoadState.ready);
      await controller.refresh();
      expect(controller.records, hasLength(2));
    },
  );
}
