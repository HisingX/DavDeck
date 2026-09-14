import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/revisions/revisions_page.dart';
import 'package:davdeck/state/revision_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';

class PageRevisionApi implements RevisionApi {
  bool failRestore = false;
  bool legacyRevision = false;
  bool includeActiveRevision = false;
  int revisionCount = 1;
  int configurationStateCalls = 0;
  int listRevisionsCalls = 0;
  final deletedIds = <String>[];

  @override
  Future<ManagedRevisionState> configurationState() async {
    configurationStateCalls++;
    return const ManagedRevisionState(
      desiredRevision: 3,
      activeRevision: 1,
      pending: true,
    );
  }

  @override
  Future<List<ManagedRevision>> listRevisions() async {
    listRevisionsCalls++;
    if (revisionCount > 1) {
      return List.generate(revisionCount, (index) {
        final number = revisionCount - index;
        return ManagedRevision(
          id: 'revision-$number',
          number: number,
          createdAt: DateTime.utc(2026, 8, 23, 1, 2, number).toIso8601String(),
          configHash: 'hash-$number',
          validationStatus: 'VALID',
          applyStatus: 'APPLIED',
          stateSnapshotAvailable: number != 1,
          appVersion: 'test',
        );
      });
    }
    return [
      if (includeActiveRevision)
        const ManagedRevision(
          id: 'revision-1',
          number: 1,
          createdAt: '2026-08-22T01:02:03Z',
          configHash: 'hash-1',
          validationStatus: 'VALID',
          applyStatus: 'APPLIED',
          appVersion: 'test',
        ),
      ManagedRevision(
        id: 'revision-2',
        number: 2,
        createdAt: '2026-08-23T01:02:03Z',
        configHash: 'hash-2',
        validationStatus: 'VALID',
        applyStatus: 'APPLIED',
        stateSnapshotAvailable: !legacyRevision,
        appVersion: 'test',
      ),
    ];
  }

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

  @override
  Future<void> deleteRevision(String id) async {
    deletedIds.add(id);
  }
}

Widget revisionsTestApp(RevisionsPage page) => MaterialApp(
  supportedLocales: const [Locale('en'), Locale('zh', 'CN')],
  localizationsDelegates: GlobalMaterialLocalizations.delegates,
  home: page,
);

void main() {
  test('revision timestamps use the local timezone and UI format', () {
    final local = DateTime.parse('2026-08-23T01:02:03Z').toLocal();
    String twoDigits(int value) => value.toString().padLeft(2, '0');
    final expected =
        '${local.year}-${twoDigits(local.month)}-${twoDigits(local.day)} '
        '${twoDigits(local.hour)}:${twoDigits(local.minute)}:${twoDigits(local.second)}';
    expect(formatRevisionCreatedAt('2026-08-23T01:02:03Z'), expected);
  });

  test('validation statuses are localized', () {
    expect(
      const AppStrings(Locale('zh', 'CN')).validationStatusLabel('VALID'),
      '有效',
    );
    expect(
      const AppStrings(Locale('en')).validationStatusLabel('VALID'),
      'Valid',
    );
  });

  testWidgets('revision page shows state and confirms restore', (tester) async {
    await tester.binding.setSurfaceSize(const Size(1440, 1000));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final api = PageRevisionApi();
    final controller = RevisionController(api);
    addTearDown(controller.dispose);
    await controller.refresh();
    await tester.pumpWidget(
      revisionsTestApp(RevisionsPage(controller: controller)),
    );

    expect(find.text('Configuration state'), findsOneWidget);
    expect(find.text('2'), findsWidgets);
    expect(find.text('Valid'), findsOneWidget);
    expect(find.textContaining('hash-2'), findsNothing);
    expect(find.byTooltip('Copy config hash'), findsNothing);
    expect(find.text('Actions'), findsNothing);
    await tester.tap(find.widgetWithText(TextButton, 'Restore'));
    await tester.pumpAndSettle();
    expect(
      find.textContaining('Restore configuration revision 2?'),
      findsOneWidget,
    );
    await tester.tap(find.widgetWithText(FilledButton, 'Restore'));
    await tester.pumpAndSettle();
    expect(controller.error, isNull);
  });

  testWidgets('revision page refreshes from the daemon', (tester) async {
    await tester.binding.setSurfaceSize(const Size(1440, 1000));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final api = PageRevisionApi();
    final controller = RevisionController(api);
    addTearDown(controller.dispose);
    await controller.refresh();
    await tester.pumpWidget(
      revisionsTestApp(RevisionsPage(controller: controller)),
    );

    expect(find.byTooltip('Refresh revisions'), findsOneWidget);
    expect(api.configurationStateCalls, 1);
    expect(api.listRevisionsCalls, 1);

    api.revisionCount = 3;
    await tester.tap(find.byTooltip('Refresh revisions'));
    await tester.pumpAndSettle();

    expect(api.configurationStateCalls, 2);
    expect(api.listRevisionsCalls, 2);
    expect(find.text('3 revisions'), findsOneWidget);
  });

  testWidgets('active revision is listed before newer recoverable revisions', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1440, 1000));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final api = PageRevisionApi()..includeActiveRevision = true;
    final controller = RevisionController(api);
    addTearDown(controller.dispose);
    await controller.refresh();
    await tester.pumpWidget(
      revisionsTestApp(RevisionsPage(controller: controller)),
    );

    expect(
      tester.getTopLeft(find.text('1').first).dy,
      lessThan(tester.getTopLeft(find.text('2').first).dy),
    );
    expect(find.text('Current'), findsOneWidget);
  });

  testWidgets('revision page displays safe restore failure', (tester) async {
    await tester.binding.setSurfaceSize(const Size(1440, 1000));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final api = PageRevisionApi()..failRestore = true;
    final controller = RevisionController(api);
    addTearDown(controller.dispose);
    await controller.refresh();
    await tester.pumpWidget(
      revisionsTestApp(RevisionsPage(controller: controller)),
    );

    await tester.tap(find.widgetWithText(TextButton, 'Restore'));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Restore'));
    await tester.pumpAndSettle();
    expect(find.textContaining('CADDY_RELOAD_FAILED'), findsOneWidget);
  });

  testWidgets('revision page disables restore for runtime-only revisions', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1440, 1000));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final api = PageRevisionApi()..legacyRevision = true;
    final controller = RevisionController(api);
    addTearDown(controller.dispose);
    await controller.refresh();
    await tester.pumpWidget(
      revisionsTestApp(RevisionsPage(controller: controller)),
    );

    expect(find.widgetWithText(TextButton, 'Restore'), findsNothing);
    expect(find.text('Valid'), findsOneWidget);
  });

  testWidgets('revision page confirms and deletes an unreferenced revision', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1440, 1000));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final api = PageRevisionApi();
    final controller = RevisionController(api);
    addTearDown(controller.dispose);
    await controller.refresh();
    await tester.pumpWidget(
      revisionsTestApp(RevisionsPage(controller: controller)),
    );

    await tester.tap(find.byTooltip('Delete'));
    await tester.pumpAndSettle();
    expect(
      find.textContaining('Delete configuration revision 2?'),
      findsOneWidget,
    );
    await tester.tap(find.widgetWithText(FilledButton, 'Delete'));
    await tester.pumpAndSettle();
    expect(api.deletedIds, contains('revision-2'));
  });

  testWidgets('revision page supports selecting and batch deleting revisions', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1440, 1000));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final api = PageRevisionApi();
    final controller = RevisionController(api);
    addTearDown(controller.dispose);
    await controller.refresh();
    await tester.pumpWidget(
      revisionsTestApp(RevisionsPage(controller: controller)),
    );

    await tester.tap(find.byType(Checkbox).last);
    await tester.pump();
    expect(find.text('Selected 1 Revision'), findsOneWidget);
    await tester.tap(find.text('Delete selected'));
    await tester.pumpAndSettle();
    expect(
      find.textContaining('Delete the selected 1 revisions?'),
      findsOneWidget,
    );
    await tester.tap(find.widgetWithText(FilledButton, 'Delete selected'));
    await tester.pumpAndSettle();
    expect(api.deletedIds, contains('revision-2'));
  });

  testWidgets('revision page defaults to ten rows and supports page sizes', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1440, 1000));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final api = PageRevisionApi()..revisionCount = 11;
    final controller = RevisionController(api);
    addTearDown(controller.dispose);
    await controller.refresh();
    await tester.pumpWidget(
      revisionsTestApp(RevisionsPage(controller: controller)),
    );

    expect(find.text('10 rows'), findsOneWidget);
    expect(find.byKey(const ValueKey('revision-page-2')), findsOneWidget);
    expect(
      tester.getTopLeft(find.text('10').first).dy -
          tester.getTopLeft(find.text('11').first).dy,
      closeTo(86, 1),
    );

    await tester.scrollUntilVisible(
      find.byKey(const ValueKey('revision-page-size')),
      400,
      scrollable: find.byType(Scrollable).first,
    );
    await tester.tap(find.byKey(const ValueKey('revision-page-size')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('20 rows').last);
    await tester.pumpAndSettle();

    expect(find.text('20 rows'), findsOneWidget);
    expect(find.byKey(const ValueKey('revision-page-2')), findsNothing);
  });

  testWidgets('wide revision cards put metadata on the title row', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1440, 1000));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final controller = RevisionController(PageRevisionApi());
    addTearDown(controller.dispose);
    await controller.refresh();
    await tester.pumpWidget(
      revisionsTestApp(RevisionsPage(controller: controller)),
    );

    final validationY = tester.getCenter(find.text('Valid')).dy;
    final createdY = tester
        .getCenter(find.text(formatRevisionCreatedAt('2026-08-23T01:02:03Z')))
        .dy;
    expect((validationY - createdY).abs(), lessThan(2));
  });

  testWidgets('narrow revision cards keep metadata below the title', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(900, 1000));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final controller = RevisionController(PageRevisionApi());
    addTearDown(controller.dispose);
    await controller.refresh();
    await tester.pumpWidget(
      revisionsTestApp(RevisionsPage(controller: controller)),
    );

    final titleY = tester.getCenter(find.text('Revision 2')).dy;
    final validationY = tester.getCenter(find.text('Validation: Valid')).dy;
    expect(validationY, greaterThan(titleY + 10));
  });
}
