import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/revisions/revisions_page.dart';
import 'package:davdeck/state/revision_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';

class PageRevisionApi implements RevisionApi {
  bool failRestore = false;

  @override
  Future<ManagedRevisionState> configurationState() async =>
      const ManagedRevisionState(
        desiredRevision: 2,
        activeRevision: 1,
        pending: true,
      );

  @override
  Future<List<ManagedRevision>> listRevisions() async => const [
    ManagedRevision(
      id: 'revision-2',
      number: 2,
      createdAt: '2026-08-23T01:02:03Z',
      configHash: 'hash-2',
      validationStatus: 'VALID',
      applyStatus: 'APPLIED',
      appVersion: 'test',
    ),
  ];

  @override
  Future<ManagedRevision> applyConfigurationResult() async =>
      (await listRevisions()).single;

  @override
  Future<ManagedRevision> restoreRevision(String id) async {
    if (failRestore) {
      throw const DaemonApiException('CADDY_RELOAD_FAILED', 'restore failed');
    }
    return (await listRevisions()).single;
  }
}

Widget revisionsTestApp(RevisionsPage page) => MaterialApp(
  supportedLocales: const [Locale('en'), Locale('zh', 'CN')],
  localizationsDelegates: GlobalMaterialLocalizations.delegates,
  home: page,
);

void main() {
  testWidgets('revision page shows state and confirms restore', (tester) async {
    final api = PageRevisionApi();
    final controller = RevisionController(api);
    addTearDown(controller.dispose);
    await controller.refresh();
    await tester.pumpWidget(
      revisionsTestApp(RevisionsPage(controller: controller)),
    );

    expect(find.text('Configuration state'), findsOneWidget);
    expect(find.text('Revision 2'), findsOneWidget);
    await tester.tap(find.text('Restore'));
    await tester.pumpAndSettle();
    expect(
      find.textContaining('Restore configuration revision 2?'),
      findsOneWidget,
    );
    await tester.tap(find.text('Restore').last);
    await tester.pumpAndSettle();
    expect(controller.error, isNull);
  });

  testWidgets('revision page displays safe restore failure', (tester) async {
    final api = PageRevisionApi()..failRestore = true;
    final controller = RevisionController(api);
    addTearDown(controller.dispose);
    await controller.refresh();
    await tester.pumpWidget(
      revisionsTestApp(RevisionsPage(controller: controller)),
    );

    await tester.tap(find.text('Restore'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Restore').last);
    await tester.pumpAndSettle();
    expect(find.textContaining('CADDY_RELOAD_FAILED'), findsOneWidget);
  });
}
