import 'dart:async';
import 'dart:convert';

import 'package:davdeck/api/daemon_api.dart';
import 'package:flutter/foundation.dart';

enum LogsLoadState { loading, ready, error }

class LogsController extends ChangeNotifier {
  LogsController(this.api, {bool startAutoRefresh = false})
    : autoRefreshEnabled = startAutoRefresh {
    if (startAutoRefresh) {
      _autoRefreshTimer = Timer.periodic(autoRefreshInterval, (_) => refresh());
    }
  }

  static const pageSize = 100;
  static const autoRefreshInterval = Duration(seconds: 30);

  final LogsApi api;
  LogsLoadState state = LogsLoadState.loading;
  List<ManagedLogRecord> records = const [];
  Object? error;
  bool refreshing = false;
  bool loadingMore = false;
  bool hasMore = false;
  int? nextCursor;
  String levelFilter = '';
  String componentFilter = '';
  bool autoRefreshEnabled = false;
  Timer? _autoRefreshTimer;

  Future<void> refresh() async {
    if (refreshing || loadingMore) return;
    refreshing = true;
    error = null;
    if (records.isEmpty) state = LogsLoadState.loading;
    notifyListeners();
    try {
      final page = await api.logs(
        limit: pageSize,
        level: levelFilter.isEmpty ? null : levelFilter,
        component: componentFilter.isEmpty ? null : componentFilter,
      );
      records = page.records;
      hasMore = page.hasMore;
      nextCursor = page.nextCursor;
      state = LogsLoadState.ready;
    } catch (caught) {
      error = caught;
      if (records.isEmpty) state = LogsLoadState.error;
    } finally {
      refreshing = false;
      notifyListeners();
    }
  }

  Future<void> loadMore() async {
    if (refreshing || loadingMore || !hasMore || nextCursor == null) return;
    loadingMore = true;
    error = null;
    notifyListeners();
    try {
      final page = await api.logs(
        limit: pageSize,
        cursor: nextCursor,
        level: levelFilter.isEmpty ? null : levelFilter,
        component: componentFilter.isEmpty ? null : componentFilter,
      );
      final existingIds = records.map((record) => record.id).toSet();
      records = [
        ...records,
        ...page.records.where((record) => !existingIds.contains(record.id)),
      ];
      hasMore = page.hasMore;
      nextCursor = page.nextCursor;
    } catch (caught) {
      error = caught;
    } finally {
      loadingMore = false;
      notifyListeners();
    }
  }

  Future<void> setLevelFilter(String value) async {
    if (levelFilter == value) return;
    levelFilter = value;
    await refresh();
  }

  Future<void> setComponentFilter(String value) async {
    final normalized = value.trim();
    if (componentFilter == normalized) return;
    componentFilter = normalized;
    await refresh();
  }

  void setAutoRefresh(bool enabled) {
    autoRefreshEnabled = enabled;
    _autoRefreshTimer?.cancel();
    _autoRefreshTimer = enabled
        ? Timer.periodic(autoRefreshInterval, (_) => refresh())
        : null;
    notifyListeners();
  }

  /// Clears the current UI snapshot. The daemon-owned log store remains intact
  /// and a subsequent refresh can load the records again.
  void clearView() {
    records = const [];
    hasMore = false;
    nextCursor = null;
    error = null;
    state = LogsLoadState.ready;
    notifyListeners();
  }

  String recordJson(ManagedLogRecord record) {
    const encoder = JsonEncoder.withIndent('  ');
    return encoder.convert(_safeRecord(record));
  }

  String exportText() {
    final encoder = const JsonEncoder.withIndent('  ');
    return encoder.convert(records.map(_safeRecord).toList(growable: false));
  }

  @override
  void dispose() {
    _autoRefreshTimer?.cancel();
    super.dispose();
  }
}

Map<String, dynamic> _safeRecord(ManagedLogRecord record) => {
  'id': record.id,
  'timestamp': record.timestamp.toUtc().toIso8601String(),
  'level': record.level,
  'component': _sanitizeText(record.component),
  'message': _sanitizeText(record.message),
  if (record.fields.isNotEmpty) 'fields': _sanitizeValue(record.fields),
};

final _privateKeyPattern = RegExp(
  r'-----BEGIN [^-]*PRIVATE KEY-----.*?-----END [^-]*PRIVATE KEY-----',
  caseSensitive: false,
  dotAll: true,
);
final _jsonSecretPattern = RegExp(
  r'''("(?:password(?:_hash)?|management[_-]?token|authorization|bearer|private[_-]?key(?:_path)?|dns[_-]?(?:api[_-]?)?token|api[_-]?key|client[_-]?secret|secret|credential)s?"\s*:\s*)("[^"]*"|'[^']*'|[^,\s}]+)''',
  caseSensitive: false,
);
final _secretPattern = RegExp(
  r'''(authorization\s*:\s*bearer\s+|\b(?:password(?:_hash)?|management[_-]?token|token|private[_-]?key|dns[_-]?(?:api[_-]?)?token|api[_-]?key|secret)\b\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;]+)''',
  caseSensitive: false,
);

String _sanitizeText(String value) {
  var sanitized = value.replaceAll(_privateKeyPattern, '[REDACTED]');
  sanitized = sanitized.replaceAllMapped(
    _jsonSecretPattern,
    (match) => '${match.group(1)}"[REDACTED]"',
  );
  return sanitized.replaceAllMapped(
    _secretPattern,
    (match) => '${match.group(1)}[REDACTED]',
  );
}

bool _isSensitiveField(String key) {
  final normalized = key.toLowerCase().replaceAll(RegExp(r'[_.-]'), '');
  return switch (normalized) {
    'password' ||
    'passwordhash' ||
    'managementtoken' ||
    'authorization' ||
    'bearer' ||
    'privatekey' ||
    'privatekeypath' ||
    'dnstoken' ||
    'dnsapitoken' ||
    'apikey' ||
    'clientsecret' ||
    'secret' ||
    'credential' ||
    'credentials' ||
    'token' => true,
    _ =>
      normalized.endsWith('password') ||
          normalized.endsWith('passwordhash') ||
          normalized.endsWith('token') ||
          normalized.endsWith('secret') ||
          normalized.endsWith('privatekey') ||
          normalized.endsWith('apikey'),
  };
}

dynamic _sanitizeValue(dynamic value) {
  if (value is String) return _sanitizeText(value);
  if (value is List) return value.map(_sanitizeValue).toList(growable: false);
  if (value is Map) {
    return <String, dynamic>{
      for (final entry in value.entries)
        '${entry.key}': _isSensitiveField('${entry.key}')
            ? '[REDACTED]'
            : _sanitizeValue(entry.value),
    };
  }
  return value;
}
