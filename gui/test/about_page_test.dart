import 'package:davdeck/about/about_page.dart';
import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/state/status_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  Widget buildApp(Locale locale, StatusController controller) => MaterialApp(
    locale: locale,
    supportedLocales: const [Locale('en'), Locale('zh', 'CN')],
    localizationsDelegates: GlobalMaterialLocalizations.delegates,
    home: AboutPage(controller: controller),
  );

  StatusController buildController(String version) {
    final controller = StatusController(
      _FakeDaemonApi(
        DaemonStatus(
          name: 'DavDeck',
          version: version,
          daemon: 'RUNNING',
          database: 'READY',
          schemaVersion: 1,
        ),
      ),
    );
    controller.refresh();
    return controller;
  }

  testWidgets(
    'about page shows project and open-source information in English',
    (tester) async {
      final controller = buildController('0.0.1');
      addTearDown(controller.dispose);
      await tester.pumpWidget(buildApp(const Locale('en'), controller));
      await tester.pumpAndSettle();

      expect(find.text('About'), findsOneWidget);
      expect(find.text('Version 0.0.1'), findsOneWidget);
      expect(find.text('https://github.com/HisingX/DavDeck'), findsOneWidget);
      expect(find.text('Open source'), findsOneWidget);
      expect(find.text('License'), findsOneWidget);
      expect(find.text('Language support'), findsNothing);
      expect(find.byType(Image), findsOneWidget);
    },
  );

  testWidgets('about page localizes project information in Chinese', (
    tester,
  ) async {
    final controller = buildController('0.0.1');
    addTearDown(controller.dispose);
    await tester.pumpWidget(buildApp(const Locale('zh', 'CN'), controller));
    await tester.pumpAndSettle();

    expect(find.text('关于'), findsOneWidget);
    expect(find.text('版本 0.0.1'), findsOneWidget);
    expect(find.text('开源项目'), findsOneWidget);
    expect(find.text('开源许可'), findsOneWidget);
    expect(find.text('语言支持'), findsNothing);
  });
}

class _FakeDaemonApi implements DaemonApi {
  const _FakeDaemonApi(this.value);

  final DaemonStatus value;

  @override
  Future<DaemonStatus> status() async => value;
}
