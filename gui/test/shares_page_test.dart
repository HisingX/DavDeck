import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/shares/shares_page.dart';
import 'package:davdeck/state/shares_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';

class FakeShareApi implements ShareApi {
  final shares = <ManagedShare>[
    const ManagedShare(
      id: 'share-1',
      name: 'Documents',
      slug: 'documents',
      path: '/srv/documents',
      enabled: true,
    ),
  ];
  var permissions = <ManagedPermission>[
    const ManagedPermission(
      shareId: 'share-1',
      userId: 'user-1',
      username: 'Alice',
      permission: 'READ',
    ),
  ];
  @override
  Future<List<ManagedShare>> listShares() async => List.of(shares);
  @override
  Future<ManagedShare> createShare(
    String name,
    String slug,
    String path,
  ) async {
    final share = ManagedShare(
      id: 'share-2',
      name: name,
      slug: slug,
      path: path,
      enabled: true,
    );
    shares.add(share);
    return share;
  }

  @override
  Future<ManagedShare> updateShare(
    String id, {
    String? name,
    String? slug,
    String? path,
    bool? enabled,
  }) async {
    final index = shares.indexWhere((share) => share.id == id);
    final old = shares[index];
    final updated = ManagedShare(
      id: id,
      name: name ?? old.name,
      slug: slug ?? old.slug,
      path: path ?? old.path,
      enabled: enabled ?? old.enabled,
    );
    shares[index] = updated;
    return updated;
  }

  @override
  Future<void> deleteShare(String id) async {
    shares.removeWhere((share) => share.id == id);
  }

  @override
  Future<List<ManagedPermission>> listPermissions(String shareId) async =>
      List.of(permissions);
  @override
  Future<ManagedPermission> setPermission(
    String shareId,
    String userId,
    String permission,
  ) async {
    final current = permissions.single;
    final updated = ManagedPermission(
      shareId: shareId,
      userId: userId,
      username: current.username,
      permission: permission,
    );
    permissions = [updated];
    return updated;
  }
}

Widget testApp(SharesController controller) => MaterialApp(
  supportedLocales: const [Locale('en'), Locale('zh', 'CN')],
  localizationsDelegates: GlobalMaterialLocalizations.delegates,
  home: SharesPage(controller: controller),
);

void main() {
  testWidgets('shares page exposes explicit ACL values', (tester) async {
    final controller = SharesController(FakeShareApi());
    await controller.refresh();
    addTearDown(controller.dispose);
    await tester.pumpWidget(testApp(controller));
    await tester.pumpAndSettle();
    expect(find.text('Documents'), findsOneWidget);
    await tester.tap(find.byType(PopupMenuButton<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Permissions'));
    await tester.pumpAndSettle();
    expect(find.text('Alice'), findsOneWidget);
    await tester.tap(find.byType(DropdownButton<String>));
    await tester.pumpAndSettle();
    expect(find.text('No access'), findsOneWidget);
    expect(find.text('Read only'), findsWidgets);
    expect(find.text('Read & write'), findsOneWidget);
  });

  testWidgets('share deletion warns that physical files are preserved', (
    tester,
  ) async {
    final controller = SharesController(FakeShareApi());
    await controller.refresh();
    addTearDown(controller.dispose);
    await tester.pumpWidget(testApp(controller));
    await tester.pumpAndSettle();
    await tester.tap(find.byType(PopupMenuButton<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Delete'));
    await tester.pumpAndSettle();
    expect(find.textContaining('physical files are preserved'), findsOneWidget);
  });
}
