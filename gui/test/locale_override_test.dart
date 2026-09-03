import 'package:davdeck/l10n/locale_override.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('maps supported development locale overrides', () {
    expect(localeFromOverride('en'), const Locale('en'));
    expect(localeFromOverride('en-US'), const Locale('en'));
    expect(localeFromOverride('zh-CN'), const Locale('zh', 'CN'));
    expect(localeFromOverride('zh_hans'), const Locale('zh', 'CN'));
  });

  test('leaves unsupported or empty overrides on system locale behavior', () {
    expect(localeFromOverride(null), isNull);
    expect(localeFromOverride(''), isNull);
    expect(localeFromOverride('fr'), isNull);
  });
}
