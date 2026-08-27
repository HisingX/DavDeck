import 'package:davdeck/about/about_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  Widget buildApp(Locale locale) => MaterialApp(
    locale: locale,
    supportedLocales: const [Locale('en'), Locale('zh', 'CN')],
    localizationsDelegates: GlobalMaterialLocalizations.delegates,
    home: const AboutPage(),
  );

  testWidgets(
    'about page shows project and open-source information in English',
    (tester) async {
      await tester.pumpWidget(buildApp(const Locale('en')));
      await tester.pumpAndSettle();

      expect(find.text('About'), findsOneWidget);
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
    await tester.pumpWidget(buildApp(const Locale('zh', 'CN')));
    await tester.pumpAndSettle();

    expect(find.text('关于'), findsOneWidget);
    expect(find.text('开源项目'), findsOneWidget);
    expect(find.text('开源许可'), findsOneWidget);
    expect(find.text('语言支持'), findsNothing);
  });
}
