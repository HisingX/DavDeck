import 'dart:async';
import 'dart:convert';
import 'dart:io';

const _maximumResponseBytes = 1024 * 1024;

class DaemonStatus {
  const DaemonStatus({
    required this.name,
    required this.version,
    required this.daemon,
    required this.database,
    required this.schemaVersion,
    this.caddy = 'UNKNOWN',
    this.webdav = 'UNKNOWN',
    this.service = const ManagedServiceStatus(
      installed: false,
      state: 'NOT_INSTALLED',
    ),
    this.lastErrorCode,
    this.portableDaemonOwned = false,
    this.pendingChanges = false,
  });

  factory DaemonStatus.fromJson(Map<String, dynamic> json) => DaemonStatus(
    name: json['name'] as String,
    version: json['version'] as String,
    daemon: json['daemon'] as String,
    database: json['database'] as String,
    schemaVersion: json['schema_version'] as int,
    caddy: json['caddy'] as String? ?? 'UNKNOWN',
    webdav: json['webdav'] as String? ?? 'UNKNOWN',
    service: json['service'] == null
        ? const ManagedServiceStatus(installed: false, state: 'NOT_INSTALLED')
        : ManagedServiceStatus.fromJson(
            json['service'] as Map<String, dynamic>,
          ),
    lastErrorCode: json['last_error_code'] as String?,
    portableDaemonOwned: json['portable_daemon_owned'] as bool? ?? false,
    pendingChanges: json['pending_changes'] as bool? ?? false,
  );

  final String name;
  final String version;
  final String daemon;
  final String database;
  final int schemaVersion;
  final String caddy;
  final String webdav;
  final ManagedServiceStatus service;
  final String? lastErrorCode;
  final bool portableDaemonOwned;
  final bool pendingChanges;
}

class ManagedServiceStatus {
  const ManagedServiceStatus({
    required this.installed,
    required this.state,
    this.startsAtBoot = false,
    this.lastErrorCode,
  });

  factory ManagedServiceStatus.fromJson(Map<String, dynamic> json) =>
      ManagedServiceStatus(
        installed: json['installed'] as bool? ?? false,
        state: json['state'] as String? ?? 'UNKNOWN',
        startsAtBoot: json['starts_at_boot'] as bool? ?? false,
        lastErrorCode: json['last_error_code'] as String?,
      );

  final bool installed;
  final String state;
  final bool startsAtBoot;
  final String? lastErrorCode;
}

class DaemonConnection {
  const DaemonConnection({required this.endpoint, required this.token});

  final Uri endpoint;
  final String token;
}

class DaemonApiException implements Exception {
  const DaemonApiException(this.code, this.message, {this.statusCode});

  final String code;
  final String message;
  final int? statusCode;

  @override
  String toString() => '$code: $message';
}

abstract interface class DaemonDiscovery {
  Future<DaemonConnection> discover();
}

typedef FileReader = Future<String> Function(String path);

class RetryingDaemonDiscovery implements DaemonDiscovery {
  RetryingDaemonDiscovery(
    this.delegate, {
    this.attempts = 25,
    this.retryDelay = const Duration(milliseconds: 200),
    Future<void> Function(Duration)? wait,
  }) : wait = wait ?? Future<void>.delayed;

  final DaemonDiscovery delegate;
  final int attempts;
  final Duration retryDelay;
  final Future<void> Function(Duration) wait;

  @override
  Future<DaemonConnection> discover() async {
    Object? lastError;
    StackTrace? lastStackTrace;
    for (var attempt = 0; attempt < attempts; attempt++) {
      try {
        return await delegate.discover();
      } catch (error, stackTrace) {
        lastError = error;
        lastStackTrace = stackTrace;
        if (attempt + 1 < attempts) {
          await wait(retryDelay);
        }
      }
    }
    Error.throwWithStackTrace(lastError!, lastStackTrace!);
  }
}

class PlatformDaemonDiscovery implements DaemonDiscovery {
  PlatformDaemonDiscovery({
    Map<String, String>? environment,
    FileReader? readFile,
    bool? isMacOS,
    bool? isWindows,
  }) : environment = environment ?? Platform.environment,
       readFile = readFile ?? ((path) => File(path).readAsString()),
       isMacOS = isMacOS ?? Platform.isMacOS,
       isWindows = isWindows ?? Platform.isWindows;

  final Map<String, String> environment;
  final FileReader readFile;
  final bool isMacOS;
  final bool isWindows;

  @override
  Future<DaemonConnection> discover() async {
    final endpointText =
        environment['DAVDECK_ENDPOINT'] ??
        (await readFile(_endpointPath())).trim();
    final endpoint = Uri.parse(endpointText);
    final address = InternetAddress.tryParse(endpoint.host);
    if (endpoint.scheme != 'http' ||
        endpoint.userInfo.isNotEmpty ||
        endpoint.path.isNotEmpty ||
        endpoint.hasQuery ||
        endpoint.hasFragment ||
        !endpoint.hasPort ||
        address == null ||
        !address.isLoopback) {
      throw const FormatException(
        'DavDeck endpoint must be loopback HTTP with an explicit port',
      );
    }
    final tokenPath = environment['DAVDECK_TOKEN_FILE'] ?? _tokenPath();
    final token = (await readFile(tokenPath)).trim();
    if (token.isEmpty) {
      throw const FormatException('DavDeck management token is empty');
    }
    return DaemonConnection(endpoint: endpoint, token: token);
  }

  String _endpointPath() {
    if (isMacOS) {
      return '${_requiredEnvironment('HOME')}/Library/Caches/DavDeck/run/management.endpoint';
    }
    if (isWindows) {
      return '${_requiredEnvironment('LOCALAPPDATA')}\\DavDeck\\run\\management.endpoint';
    }
    final runtime = environment['XDG_RUNTIME_DIR'];
    if (runtime != null && runtime.isNotEmpty) {
      return '$runtime/DavDeck/management.endpoint';
    }
    return '${_requiredEnvironment('HOME')}/.cache/DavDeck/run/management.endpoint';
  }

  String _tokenPath() {
    if (isMacOS) {
      return '${_requiredEnvironment('HOME')}/Library/Application Support/DavDeck/management.token';
    }
    if (isWindows) {
      return '${_requiredEnvironment('APPDATA')}\\DavDeck\\management.token';
    }
    final config =
        environment['XDG_CONFIG_HOME'] ??
        '${_requiredEnvironment('HOME')}/.config';
    return '$config/DavDeck/management.token';
  }

  String _requiredEnvironment(String name) {
    final value = environment[name];
    if (value == null || value.isEmpty) {
      throw FormatException('$name is required for DavDeck discovery');
    }
    return value;
  }
}

abstract interface class DaemonApi {
  Future<DaemonStatus> status();
}

class ManagedLogRecord {
  const ManagedLogRecord({
    required this.id,
    required this.timestamp,
    required this.level,
    required this.component,
    required this.message,
    this.fields = const {},
  });

  factory ManagedLogRecord.fromJson(Map<String, dynamic> json) {
    final rawFields = json['fields'];
    return ManagedLogRecord(
      id: json['id'] as int,
      timestamp: DateTime.parse(json['timestamp'] as String),
      level: json['level'] as String? ?? 'INFO',
      component: json['component'] as String? ?? 'daemon',
      message: json['message'] as String? ?? '',
      fields: rawFields is Map
          ? Map<String, dynamic>.from(rawFields)
          : const {},
    );
  }

  final int id;
  final DateTime timestamp;
  final String level;
  final String component;
  final String message;
  final Map<String, dynamic> fields;
}

class ManagedLogPage {
  const ManagedLogPage({
    this.records = const [],
    this.nextCursor,
    this.hasMore = false,
  });

  factory ManagedLogPage.fromJson(Map<String, dynamic> json) => ManagedLogPage(
    records: (json['records'] as List<dynamic>? ?? const [])
        .map(
          (value) => ManagedLogRecord.fromJson(value as Map<String, dynamic>),
        )
        .toList(growable: false),
    nextCursor: json['next_cursor'] as int?,
    hasMore: json['has_more'] as bool? ?? false,
  );

  final List<ManagedLogRecord> records;
  final int? nextCursor;
  final bool hasMore;
}

abstract interface class LogsApi {
  Future<ManagedLogPage> logs({
    int limit = 100,
    int? cursor,
    DateTime? since,
    String? level,
    String? component,
  });
}

class ManagedServerStatus {
  const ManagedServerStatus({
    required this.caddy,
    this.webdav = 'UNKNOWN',
    this.lastErrorCode,
    this.pendingChanges = false,
  });
  factory ManagedServerStatus.fromJson(Map<String, dynamic> json) =>
      ManagedServerStatus(
        caddy: json['caddy'] as String? ?? 'UNKNOWN',
        webdav: json['webdav'] as String? ?? 'UNKNOWN',
        lastErrorCode: json['last_error_code'] as String?,
        pendingChanges: json['pending_changes'] as bool? ?? false,
      );
  final String caddy;
  final String webdav;
  final String? lastErrorCode;
  final bool pendingChanges;
}

class ManagedServerSettings {
  const ManagedServerSettings({
    required this.httpPort,
    required this.httpsPort,
  });
  factory ManagedServerSettings.fromJson(Map<String, dynamic> json) =>
      ManagedServerSettings(
        httpPort: json['http_port'] as int,
        httpsPort: json['https_port'] as int,
      );
  final int httpPort;
  final int httpsPort;
}

abstract interface class ServerApi {
  Future<ManagedServerStatus> serverStatus();
  Future<void> startServer();
  Future<void> stopServer();
  Future<void> restartServer();
}

abstract interface class ServerSettingsApi {
  Future<ManagedServerSettings> serverSettings();
  Future<ManagedServerSettings> updateServerPorts(int httpPort, int httpsPort);
}

abstract interface class ServiceApi {
  Future<ManagedServiceStatus> serviceStatus();
  Future<void> installService();
  Future<void> uninstallService();
  Future<void> startService();
  Future<void> stopService();
}

class ManagedUser {
  const ManagedUser({
    required this.id,
    required this.username,
    required this.enabled,
  });
  factory ManagedUser.fromJson(Map<String, dynamic> json) => ManagedUser(
    id: json['id'] as String,
    username: json['username'] as String,
    enabled: json['enabled'] as bool,
  );
  final String id;
  final String username;
  final bool enabled;
}

abstract interface class UserApi {
  Future<List<ManagedUser>> listUsers();
  Future<ManagedUser> createUser(String username, String password);
  Future<ManagedUser> setUserEnabled(String id, bool enabled);
  Future<void> changeUserPassword(String id, String password);
  Future<void> deleteUser(String id);
}

class ManagedShare {
  const ManagedShare({
    required this.id,
    required this.name,
    required this.slug,
    required this.path,
    required this.enabled,
  });
  factory ManagedShare.fromJson(Map<String, dynamic> json) => ManagedShare(
    id: json['id'] as String,
    name: json['name'] as String,
    slug: json['slug'] as String,
    path: json['path'] as String,
    enabled: json['enabled'] as bool,
  );
  final String id;
  final String name;
  final String slug;
  final String path;
  final bool enabled;
}

class ManagedPermission {
  const ManagedPermission({
    required this.shareId,
    required this.userId,
    required this.username,
    required this.permission,
  });
  factory ManagedPermission.fromJson(Map<String, dynamic> json) =>
      ManagedPermission(
        shareId: json['share_id'] as String,
        userId: json['user_id'] as String,
        username: json['username'] as String,
        permission: json['permission'] as String,
      );
  final String shareId;
  final String userId;
  final String username;
  final String permission;
}

abstract interface class ShareApi {
  Future<List<ManagedShare>> listShares();
  Future<ManagedShare> createShare(String name, String slug, String path);
  Future<ManagedShare> updateShare(
    String id, {
    String? name,
    String? slug,
    String? path,
    bool? enabled,
  });
  Future<void> deleteShare(String id);
  Future<List<ManagedPermission>> listPermissions(String shareId);
  Future<ManagedPermission> setPermission(
    String shareId,
    String userId,
    String permission,
  );
}

class ManagedTlsProfile {
  const ManagedTlsProfile({
    required this.id,
    required this.mode,
    required this.hostname,
    required this.certificatePath,
    required this.privateKeyPath,
  });

  factory ManagedTlsProfile.fromJson(Map<String, dynamic> json) =>
      ManagedTlsProfile(
        id: json['id'] as String,
        mode: json['mode'] as String,
        hostname: json['hostname'] as String,
        certificatePath: json['certificate_path'] as String? ?? '',
        privateKeyPath: json['private_key_path'] as String? ?? '',
      );

  final String id;
  final String mode;
  final String hostname;
  final String certificatePath;
  final String privateKeyPath;
}

class ManagedTlsCheck {
  const ManagedTlsCheck({
    required this.name,
    required this.ok,
    required this.message,
  });
  factory ManagedTlsCheck.fromJson(Map<String, dynamic> json) =>
      ManagedTlsCheck(
        name: json['name'] as String,
        ok: json['ok'] as bool,
        message: json['message'] as String,
      );
  final String name;
  final bool ok;
  final String message;
}

class ManagedTlsCheckResult {
  const ManagedTlsCheckResult({required this.ready, required this.checks});
  factory ManagedTlsCheckResult.fromJson(Map<String, dynamic> json) =>
      ManagedTlsCheckResult(
        ready: json['ready'] as bool,
        checks: (json['checks'] as List<dynamic>)
            .map(
              (value) =>
                  ManagedTlsCheck.fromJson(value as Map<String, dynamic>),
            )
            .toList(growable: false),
      );
  final bool ready;
  final List<ManagedTlsCheck> checks;
}

abstract interface class TlsApi {
  Future<ManagedTlsProfile?> getTls();
  Future<ManagedTlsProfile> updateTls({
    required String mode,
    required String hostname,
    String certificatePath = '',
    String privateKeyPath = '',
  });
  Future<ManagedTlsCheckResult> checkTls();
}

abstract interface class ConfigurationApi {
  Future<void> applyConfiguration();
}

class ManagedRevision {
  const ManagedRevision({
    required this.id,
    required this.number,
    required this.createdAt,
    required this.configHash,
    required this.validationStatus,
    required this.applyStatus,
    required this.appVersion,
    this.errorCode,
    this.errorSummary,
  });

  factory ManagedRevision.fromJson(Map<String, dynamic> json) =>
      ManagedRevision(
        id: json['id'] as String,
        number: json['number'] as int,
        createdAt: json['created_at'] as String? ?? '',
        configHash: json['config_hash'] as String? ?? '',
        validationStatus: json['validation_status'] as String? ?? 'UNKNOWN',
        applyStatus: json['apply_status'] as String? ?? 'NOT_APPLIED',
        appVersion: json['app_version'] as String? ?? '',
        errorCode: json['error_code'] as String?,
        errorSummary: json['error_summary'] as String?,
      );

  final String id;
  final int number;
  final String createdAt;
  final String configHash;
  final String validationStatus;
  final String applyStatus;
  final String appVersion;
  final String? errorCode;
  final String? errorSummary;
}

class ManagedRevisionState {
  const ManagedRevisionState({
    this.desiredRevision,
    this.activeRevision,
    this.dirty = false,
    this.pending = false,
  });

  factory ManagedRevisionState.fromJson(Map<String, dynamic> json) =>
      ManagedRevisionState(
        desiredRevision: json['desired_revision'] as int?,
        activeRevision: json['active_revision'] as int?,
        dirty: json['dirty'] as bool? ?? false,
        pending: json['pending'] as bool? ?? false,
      );

  final int? desiredRevision;
  final int? activeRevision;
  final bool dirty;
  final bool pending;
}

abstract interface class RevisionApi {
  Future<ManagedRevisionState> configurationState();
  Future<List<ManagedRevision>> listRevisions();
  Future<ManagedRevision> applyConfigurationResult();
  Future<ManagedRevision> restoreRevision(String id);
}

class DiagnosticResult {
  const DiagnosticResult({
    required this.id,
    required this.title,
    required this.status,
    required this.code,
    required this.message,
  });
  factory DiagnosticResult.fromJson(Map<String, dynamic> json) =>
      DiagnosticResult(
        id: json['id'] as String,
        title: json['title'] as String,
        status: json['status'] as String,
        code: json['code'] as String? ?? '',
        message: json['message'] as String,
      );
  final String id;
  final String title;
  final String status;
  final String code;
  final String message;
}

class DiagnosticReport {
  const DiagnosticReport({
    required this.generatedAt,
    required this.overall,
    required this.sanitized,
    required this.results,
  });
  factory DiagnosticReport.fromJson(Map<String, dynamic> json) =>
      DiagnosticReport(
        generatedAt: json['generated_at'] as String,
        overall: json['overall'] as String,
        sanitized: json['sanitized'] as bool,
        results: (json['results'] as List<dynamic>)
            .map(
              (value) =>
                  DiagnosticResult.fromJson(value as Map<String, dynamic>),
            )
            .toList(growable: false),
      );
  final String generatedAt;
  final String overall;
  final bool sanitized;
  final List<DiagnosticResult> results;
}

abstract interface class DiagnosticsApi {
  Future<DiagnosticReport> runDiagnostics();
}

abstract interface class ManagementApi
    implements
        DaemonApi,
        LogsApi,
        ServerApi,
        ServerSettingsApi,
        ServiceApi,
        UserApi,
        ShareApi,
        TlsApi,
        ConfigurationApi,
        DiagnosticsApi {}

typedef HttpClientFactory = HttpClient Function();

class ManagementDaemonApi implements ManagementApi, RevisionApi {
  ManagementDaemonApi({
    required this.discovery,
    HttpClientFactory? httpClientFactory,
  }) : httpClientFactory = httpClientFactory ?? HttpClient.new;

  final DaemonDiscovery discovery;
  final HttpClientFactory httpClientFactory;

  @override
  Future<ManagedLogPage> logs({
    int limit = 100,
    int? cursor,
    DateTime? since,
    String? level,
    String? component,
  }) async {
    final query = <String, String>{'limit': '$limit'};
    if (cursor != null) query['cursor'] = '$cursor';
    if (since != null) query['since'] = since.toUtc().toIso8601String();
    if (level != null && level.isNotEmpty) query['level'] = level;
    if (component != null && component.isNotEmpty) {
      query['component'] = component;
    }
    return ManagedLogPage.fromJson(
      await request('GET', '/api/v1/logs', queryParameters: query)
          as Map<String, dynamic>,
    );
  }

  @override
  Future<DaemonStatus> status() async {
    final data = await request('GET', '/api/v1/status');
    return DaemonStatus.fromJson(data as Map<String, dynamic>);
  }

  @override
  Future<ManagedServiceStatus> serviceStatus() async =>
      ManagedServiceStatus.fromJson(
        await request('GET', '/api/v1/service/status') as Map<String, dynamic>,
      );

  @override
  Future<void> installService() async =>
      request('POST', '/api/v1/service/install');

  @override
  Future<void> uninstallService() async =>
      request('POST', '/api/v1/service/uninstall');

  @override
  Future<void> startService() async => request('POST', '/api/v1/service/start');

  @override
  Future<void> stopService() async => request('POST', '/api/v1/service/stop');

  @override
  Future<ManagedServerStatus> serverStatus() async =>
      ManagedServerStatus.fromJson(
        await request('GET', '/api/v1/server/status') as Map<String, dynamic>,
      );
  @override
  Future<void> startServer() async => request('POST', '/api/v1/server/start');
  @override
  Future<void> stopServer() async => request('POST', '/api/v1/server/stop');
  @override
  Future<void> restartServer() async =>
      request('POST', '/api/v1/server/restart');

  @override
  Future<ManagedServerSettings> serverSettings() async =>
      ManagedServerSettings.fromJson(
        await request('GET', '/api/v1/server/settings') as Map<String, dynamic>,
      );

  @override
  Future<ManagedServerSettings> updateServerPorts(
    int httpPort,
    int httpsPort,
  ) async => ManagedServerSettings.fromJson(
    await request(
          'PUT',
          '/api/v1/server/settings',
          body: {'http_port': httpPort, 'https_port': httpsPort},
        )
        as Map<String, dynamic>,
  );

  @override
  Future<List<ManagedUser>> listUsers() async {
    final data = await request('GET', '/api/v1/users') as List<dynamic>;
    return data
        .map((value) => ManagedUser.fromJson(value as Map<String, dynamic>))
        .toList(growable: false);
  }

  @override
  Future<ManagedUser> createUser(String username, String password) async =>
      ManagedUser.fromJson(
        await request(
              'POST',
              '/api/v1/users',
              body: {'username': username, 'password': password},
            )
            as Map<String, dynamic>,
      );

  @override
  Future<ManagedUser> setUserEnabled(String id, bool enabled) async =>
      ManagedUser.fromJson(
        await request('PATCH', '/api/v1/users/$id', body: {'enabled': enabled})
            as Map<String, dynamic>,
      );

  @override
  Future<void> changeUserPassword(String id, String password) async {
    await request(
      'POST',
      '/api/v1/users/$id/password',
      body: {'password': password},
    );
  }

  @override
  Future<void> deleteUser(String id) async {
    await request('DELETE', '/api/v1/users/$id');
  }

  @override
  Future<List<ManagedShare>> listShares() async {
    final data = await request('GET', '/api/v1/shares') as List<dynamic>;
    return data
        .map((value) => ManagedShare.fromJson(value as Map<String, dynamic>))
        .toList(growable: false);
  }

  @override
  Future<ManagedShare> createShare(
    String name,
    String slug,
    String path,
  ) async => ManagedShare.fromJson(
    await request(
          'POST',
          '/api/v1/shares',
          body: {'name': name, 'slug': slug, 'path': path},
        )
        as Map<String, dynamic>,
  );
  @override
  Future<ManagedShare> updateShare(
    String id, {
    String? name,
    String? slug,
    String? path,
    bool? enabled,
  }) async {
    final body = <String, Object>{};
    if (name != null) body['name'] = name;
    if (slug != null) body['slug'] = slug;
    if (path != null) body['path'] = path;
    if (enabled != null) body['enabled'] = enabled;
    return ManagedShare.fromJson(
      await request('PATCH', '/api/v1/shares/$id', body: body)
          as Map<String, dynamic>,
    );
  }

  @override
  Future<void> deleteShare(String id) async {
    await request('DELETE', '/api/v1/shares/$id');
  }

  @override
  Future<List<ManagedPermission>> listPermissions(String shareId) async {
    final data =
        await request('GET', '/api/v1/shares/$shareId/permissions')
            as List<dynamic>;
    return data
        .map(
          (value) => ManagedPermission.fromJson(value as Map<String, dynamic>),
        )
        .toList(growable: false);
  }

  @override
  Future<ManagedPermission> setPermission(
    String shareId,
    String userId,
    String permission,
  ) async => ManagedPermission.fromJson(
    await request(
          'PUT',
          '/api/v1/shares/$shareId/permissions/$userId',
          body: {'permission': permission},
        )
        as Map<String, dynamic>,
  );

  @override
  Future<ManagedTlsProfile?> getTls() async {
    final data = await request('GET', '/api/v1/tls');
    if (data == null) return null;
    return ManagedTlsProfile.fromJson(data as Map<String, dynamic>);
  }

  @override
  Future<ManagedTlsProfile> updateTls({
    required String mode,
    required String hostname,
    String certificatePath = '',
    String privateKeyPath = '',
  }) async => ManagedTlsProfile.fromJson(
    await request(
          'PUT',
          '/api/v1/tls',
          body: {
            'mode': mode,
            'hostname': hostname,
            'certificate_path': certificatePath,
            'private_key_path': privateKeyPath,
          },
        )
        as Map<String, dynamic>,
  );

  @override
  Future<ManagedTlsCheckResult> checkTls() async =>
      ManagedTlsCheckResult.fromJson(
        await request('POST', '/api/v1/tls/check') as Map<String, dynamic>,
      );

  @override
  Future<void> applyConfiguration() async {
    await request('POST', '/api/v1/config/apply');
  }

  @override
  Future<ManagedRevisionState> configurationState() async =>
      ManagedRevisionState.fromJson(
        await request('GET', '/api/v1/config/state') as Map<String, dynamic>,
      );

  @override
  Future<List<ManagedRevision>> listRevisions() async =>
      (await request('GET', '/api/v1/revisions') as List<dynamic>)
          .map(
            (value) => ManagedRevision.fromJson(value as Map<String, dynamic>),
          )
          .toList(growable: false);

  @override
  Future<ManagedRevision> applyConfigurationResult() async =>
      ManagedRevision.fromJson(
        await request('POST', '/api/v1/config/apply') as Map<String, dynamic>,
      );

  @override
  Future<ManagedRevision> restoreRevision(String id) async =>
      ManagedRevision.fromJson(
        await request(
              'POST',
              '/api/v1/revisions/${Uri.encodeComponent(id)}/restore',
            )
            as Map<String, dynamic>,
      );

  @override
  Future<DiagnosticReport> runDiagnostics() async => DiagnosticReport.fromJson(
    await request('POST', '/api/v1/diagnostics/run') as Map<String, dynamic>,
  );

  Future<Object?> request(
    String method,
    String path, {
    Object? body,
    Map<String, String>? queryParameters,
  }) async {
    final connection = await discovery.discover();
    final baseUri = connection.endpoint.resolve(path);
    final uri = queryParameters == null
        ? baseUri
        : baseUri.replace(queryParameters: queryParameters);
    final client = httpClientFactory()
      ..connectionTimeout = const Duration(seconds: 5);
    try {
      final request = await client.openUrl(method, uri);
      request.headers.set(
        HttpHeaders.authorizationHeader,
        'Bearer ${connection.token}',
      );
      if (body != null) {
        request.headers.contentType = ContentType.json;
        request.write(jsonEncode(body));
      }
      final response = await request.close().timeout(
        const Duration(seconds: 10),
      );
      final bytes = <int>[];
      await for (final chunk in response) {
        bytes.addAll(chunk);
        if (bytes.length > _maximumResponseBytes) {
          throw const DaemonApiException(
            'RESPONSE_TOO_LARGE',
            'DavDeck response exceeded the safety limit',
          );
        }
      }
      final payload = jsonDecode(utf8.decode(bytes)) as Map<String, dynamic>;
      if (response.statusCode < 200 ||
          response.statusCode >= 300 ||
          payload['success'] != true) {
        final error = payload['error'] as Map<String, dynamic>?;
        throw DaemonApiException(
          error?['code'] as String? ?? 'REQUEST_FAILED',
          error?['message'] as String? ?? 'DavDeck request failed',
          statusCode: response.statusCode,
        );
      }
      return payload['data'];
    } on TimeoutException {
      throw const DaemonApiException(
        'DAEMON_TIMEOUT',
        'DavDeck did not respond in time',
      );
    } finally {
      client.close(force: true);
    }
  }
}

class LocalDaemonApi extends ManagementDaemonApi {
  LocalDaemonApi({Map<String, String>? environment})
    : super(
        discovery: RetryingDaemonDiscovery(
          PlatformDaemonDiscovery(environment: environment),
        ),
      );
}
