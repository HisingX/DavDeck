import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/state/service_controller.dart';
import 'package:flutter_test/flutter_test.dart';

class FakeServiceApi implements ServiceApi {
  ManagedServiceStatus value = const ManagedServiceStatus(
    installed: false,
    state: 'NOT_INSTALLED',
  );
  final calls = <String>[];
  Object? failure;

  @override
  Future<ManagedServiceStatus> serviceStatus() async {
    calls.add('status');
    if (failure != null) throw failure!;
    return value;
  }

  @override
  Future<void> installService() => _mutate('install');

  @override
  Future<void> uninstallService() => _mutate('uninstall');

  @override
  Future<void> startService() => _mutate('start');

  @override
  Future<void> stopService() => _mutate('stop');

  Future<void> _mutate(String operation) async {
    calls.add(operation);
    if (failure != null) throw failure!;
    value = ManagedServiceStatus(
      installed: operation != 'uninstall',
      state: operation == 'start' ? 'RUNNING' : 'STOPPED',
      startsAtBoot: operation == 'install',
    );
  }
}

void main() {
  test(
    'service controller refreshes and controls lifecycle through API',
    () async {
      final api = FakeServiceApi();
      var statusRefreshes = 0;
      final controller = ServiceController(
        api,
        onChanged: () async => statusRefreshes++,
      );
      addTearDown(controller.dispose);

      await controller.refresh();
      expect(controller.state, ServiceLoadState.ready);
      expect(controller.service!.installed, isFalse);
      expect(await controller.install(), isTrue);
      expect(controller.service!.installed, isTrue);
      expect(statusRefreshes, 1);
      expect(api.calls, ['status', 'install', 'status']);

      expect(await controller.start(), isTrue);
      expect(controller.service!.state, 'RUNNING');
      expect(await controller.stop(), isTrue);
      expect(controller.service!.state, 'STOPPED');
    },
  );

  test('service controller preserves stable API errors', () async {
    final api = FakeServiceApi()
      ..failure = const DaemonApiException(
        'PRIVILEGE_REQUIRED',
        'Administrator privileges are required',
      );
    final controller = ServiceController(api);
    addTearDown(controller.dispose);

    await controller.refresh();
    expect(controller.state, ServiceLoadState.error);
    expect(controller.error, isA<DaemonApiException>());
    expect(await controller.install(), isFalse);
    expect(controller.actionError, isA<DaemonApiException>());
    expect(controller.busy, isFalse);
  });
}
