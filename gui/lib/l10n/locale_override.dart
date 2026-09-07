import 'package:flutter/material.dart';

/// Converts the optional development-only locale override into a supported
/// Flutter locale. An unknown value returns null so the app keeps following
/// the operating-system locale.
Locale? localeFromOverride(String? value) {
  final normalized = value?.trim().toLowerCase().replaceAll('_', '-');
  return switch (normalized) {
    'en' || 'en-us' || 'en-gb' => const Locale('en'),
    'zh' || 'zh-cn' || 'zh-hans' => const Locale('zh', 'CN'),
    _ => null,
  };
}
