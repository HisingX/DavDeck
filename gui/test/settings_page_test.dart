import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/settings/settings_page.dart';
import 'package:davdeck/state/backup_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';

class FakeBackupApi implements BackupApi {
  FakeBackupApi({this.exported = 'version: 1\nusers: []\nshares: []\n'});

  final String exported;
  String? imported;

  @override
  Future<String> exportConfiguration() async => exported;

  @override
  Future<ManagedConfigImportResult> importConfiguration(String content) async {
    imported = content;
    return const ManagedConfigImportResult(
      usersCreated: 1,
      sharesUpdated: 1,
      permissionsUpserted: 2,
      passwordResetRequired: ['Alice'],
      pendingApply: true,
    );
  }
}

Widget settingsTestApp(SettingsPage page) => MaterialApp(
  locale: const Locale('en'),
  supportedLocales: const [Locale('en'), Locale('zh', 'CN')],
  localizationsDelegates: GlobalMaterialLocalizations.delegates,
  home: page,
);

void main() {
  testWidgets('settings explains data retention and exports a backup', (
    tester,
  ) async {
    final api = FakeBackupApi();
    final controller = BackupController(api);
    addTearDown(controller.dispose);
    String? writtenPath;
    String? writtenContent;
    final page = SettingsPage(
      controller: controller,
      pickSavePath: () async => '/tmp/davdeck-backup.yaml',
      readFile: () async => null,
      writeFile: (path, content) async {
        writtenPath = path;
        writtenContent = content;
      },
    );

    await tester.pumpWidget(settingsTestApp(page));
    await tester.pumpAndSettle();
    expect(
      find.textContaining('preserves user configuration by default'),
      findsOneWidget,
    );
    await tester.scrollUntilVisible(
      find.text('Export configuration backup'),
      500,
    );
    await tester.tap(find.text('Export configuration backup'));
    await tester.pumpAndSettle();

    expect(writtenPath, '/tmp/davdeck-backup.yaml');
    expect(writtenContent, api.exported);
    expect(
      find.textContaining('Configuration backup exported to'),
      findsOneWidget,
    );
  });

  testWidgets(
    'settings confirms import and explains password and apply follow-up',
    (tester) async {
      final api = FakeBackupApi();
      final controller = BackupController(api);
      addTearDown(controller.dispose);
      const yaml = 'version: 1\nusers: []\nshares: []\n';
      final page = SettingsPage(
        controller: controller,
        pickSavePath: () async => null,
        readFile: () async => yaml,
        writeFile: (_, _) async {},
      );

      await tester.pumpWidget(settingsTestApp(page));
      await tester.pumpAndSettle();
      await tester.scrollUntilVisible(
        find.text('Import configuration backup'),
        500,
      );
      await tester.tap(find.text('Import configuration backup'));
      await tester.pumpAndSettle();
      expect(find.text('Confirm configuration import'), findsOneWidget);
      expect(
        find.textContaining('does not delete existing shared directories'),
        findsOneWidget,
      );

      await tester.tap(find.text('Import configuration backup').last);
      await tester.pumpAndSettle();
      expect(api.imported, yaml);
      expect(find.text('Configuration backup imported'), findsOneWidget);
      expect(
        find.textContaining('Accounts requiring a new password: Alice'),
        findsOneWidget,
      );
      expect(
        find.textContaining('Apply configuration on the Dashboard'),
        findsOneWidget,
      );
    },
  );
}
