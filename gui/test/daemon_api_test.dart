import 'dart:convert';
import 'dart:io';

import 'package:davdeck/api/daemon_api.dart';
import 'package:flutter_test/flutter_test.dart';

class FakeDiscovery implements DaemonDiscovery {
  FakeDiscovery(this.connection);
  final DaemonConnection connection;

  @override
  Future<DaemonConnection> discover() async => connection;
}

class EventuallyReadyDiscovery implements DaemonDiscovery {
  EventuallyReadyDiscovery(this.connection, this.failuresBeforeReady);

  final DaemonConnection connection;
  int failuresBeforeReady;
  int calls = 0;

  @override
  Future<DaemonConnection> discover() async {
    calls++;
    if (failuresBeforeReady > 0) {
      failuresBeforeReady--;
      throw const FileSystemException('daemon endpoint is not ready');
    }
    return connection;
  }
}

void main() {
  test(
    'discovery validates loopback endpoint and reads token abstraction',
    () async {
      final reads = <String>[];
      final discovery = PlatformDaemonDiscovery(
        environment: const {
          'HOME': '/home/test',
          'DAVDECK_ENDPOINT': 'http://127.0.0.1:8090',
          'DAVDECK_TOKEN_FILE': '/token',
        },
        isMacOS: false,
        isWindows: false,
        readFile: (path) async {
          reads.add(path);
          return 'secret\n';
        },
      );
      final connection = await discovery.discover();
      expect(connection.endpoint, Uri.parse('http://127.0.0.1:8090'));
      expect(connection.token, 'secret');
      expect(reads, ['/token']);
    },
  );

  test('discovery rejects non-loopback endpoint', () async {
    final discovery = PlatformDaemonDiscovery(
      environment: const {
        'HOME': '/home/test',
        'DAVDECK_ENDPOINT': 'http://192.0.2.1:8090',
        'DAVDECK_TOKEN_FILE': '/token',
      },
      isMacOS: false,
      isWindows: false,
      readFile: (_) async => 'secret',
    );
    await expectLater(discovery.discover(), throwsFormatException);
  });

  test(
    'retrying discovery waits for the bundled daemon to become ready',
    () async {
      final delegate = EventuallyReadyDiscovery(
        DaemonConnection(
          endpoint: Uri.parse('http://127.0.0.1:8090'),
          token: 'token',
        ),
        2,
      );
      final waits = <Duration>[];
      final discovery = RetryingDaemonDiscovery(
        delegate,
        attempts: 3,
        retryDelay: const Duration(milliseconds: 1),
        wait: (duration) async => waits.add(duration),
      );

      expect((await discovery.discover()).token, 'token');
      expect(delegate.calls, 3);
      expect(waits, [
        const Duration(milliseconds: 1),
        const Duration(milliseconds: 1),
      ]);
    },
  );

  test('management API authenticates and decodes status envelope', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));
    server.listen((request) async {
      expect(request.uri.path, '/api/v1/status');
      expect(
        request.headers.value(HttpHeaders.authorizationHeader),
        'Bearer token',
      );
      request.response.headers.contentType = ContentType.json;
      request.response.write(
        jsonEncode({
          'success': true,
          'data': {
            'name': 'DavDeck',
            'version': 'test',
            'daemon': 'RUNNING',
            'database': 'READY',
            'schema_version': 4,
            'caddy': 'FAILED',
            'webdav': 'UNKNOWN',
            'last_error_code': 'CADDY_START_FAILED',
            'portable_daemon_owned': true,
            'pending_changes': true,
          },
        }),
      );
      await request.response.close();
    });
    final api = ManagementDaemonApi(
      discovery: FakeDiscovery(
        DaemonConnection(
          endpoint: Uri.parse('http://127.0.0.1:${server.port}'),
          token: 'token',
        ),
      ),
    );
    final status = await api.status();
    expect(status.name, 'DavDeck');
    expect(status.schemaVersion, 4);
    expect(status.caddy, 'FAILED');
    expect(status.webdav, 'UNKNOWN');
    expect(status.lastErrorCode, 'CADDY_START_FAILED');
    expect(status.portableDaemonOwned, isTrue);
    expect(status.pendingChanges, isTrue);
  });

  test('management API returns stable typed errors', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));
    server.listen((request) async {
      request.response.statusCode = HttpStatus.unauthorized;
      request.response.headers.contentType = ContentType.json;
      request.response.write(
        jsonEncode({
          'success': false,
          'error': {
            'code': 'UNAUTHORIZED',
            'message': 'Authentication required',
          },
        }),
      );
      await request.response.close();
    });
    final api = ManagementDaemonApi(
      discovery: FakeDiscovery(
        DaemonConnection(
          endpoint: Uri.parse('http://127.0.0.1:${server.port}'),
          token: 'wrong',
        ),
      ),
    );
    await expectLater(
      api.status(),
      throwsA(
        isA<DaemonApiException>()
            .having((error) => error.code, 'code', 'UNAUTHORIZED')
            .having((error) => error.statusCode, 'status', 401),
      ),
    );
  });

  test('management API handles TLS desired state and apply flow', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));
    server.listen((request) async {
      request.response.headers.contentType = ContentType.json;
      Object? data;
      switch ('${request.method} ${request.uri.path}') {
        case 'GET /api/v1/tls':
          data = null;
        case 'PUT /api/v1/tls':
          final input =
              jsonDecode(await utf8.decoder.bind(request).join())
                  as Map<String, dynamic>;
          expect(input['private_key_path'], '/key.pem');
          data = {
            'id': 'tls-1',
            'mode': input['mode'],
            'hostname': input['hostname'],
            'certificate_path': input['certificate_path'],
            'private_key_path': input['private_key_path'],
          };
        case 'POST /api/v1/tls/check':
          data = {
            'ready': true,
            'checks': [
              {'name': 'certificate_pair', 'ok': true, 'message': 'valid'},
            ],
          };
        case 'POST /api/v1/config/apply':
          data = {'number': 1};
        case 'POST /api/v1/diagnostics/run':
          data = {
            'generated_at': '2026-08-20T01:02:03Z',
            'overall': 'PASS',
            'sanitized': true,
            'results': [
              {
                'id': 'database',
                'title': 'Database',
                'status': 'PASS',
                'message': 'SQLite is ready',
              },
            ],
          };
        default:
          request.response.statusCode = HttpStatus.notFound;
      }
      request.response.write(jsonEncode({'success': true, 'data': data}));
      await request.response.close();
    });
    final api = ManagementDaemonApi(
      discovery: FakeDiscovery(
        DaemonConnection(
          endpoint: Uri.parse('http://127.0.0.1:${server.port}'),
          token: 'token',
        ),
      ),
    );
    expect(await api.getTls(), isNull);
    final profile = await api.updateTls(
      mode: 'custom',
      hostname: 'dav.example.com',
      certificatePath: '/cert.pem',
      privateKeyPath: '/key.pem',
    );
    expect(profile.mode, 'custom');
    expect((await api.checkTls()).ready, isTrue);
    await api.applyConfiguration();
    final diagnostics = await api.runDiagnostics();
    expect(diagnostics.sanitized, isTrue);
    expect(diagnostics.results.single.id, 'database');
  });

  test('management API requests paged logs with safe filters', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));
    server.listen((request) async {
      expect(request.uri.path, '/api/v1/logs');
      expect(
        request.headers.value(HttpHeaders.authorizationHeader),
        'Bearer token',
      );
      expect(request.uri.queryParameters, {
        'limit': '2',
        'cursor': '5',
        'since': '2026-08-23T01:02:03.000Z',
        'level': 'ERROR',
        'component': 'caddy',
      });
      request.response.headers.contentType = ContentType.json;
      request.response.write(
        jsonEncode({
          'success': true,
          'data': {
            'records': [
              {
                'id': 4,
                'timestamp': '2026-08-23T01:02:04Z',
                'level': 'ERROR',
                'component': 'caddy',
                'message': 'safe failure',
                'fields': {'error_code': 'CADDY_START_FAILED'},
              },
            ],
            'next_cursor': 4,
            'has_more': true,
          },
        }),
      );
      await request.response.close();
    });
    final api = ManagementDaemonApi(
      discovery: FakeDiscovery(
        DaemonConnection(
          endpoint: Uri.parse('http://127.0.0.1:${server.port}'),
          token: 'token',
        ),
      ),
    );
    final page = await api.logs(
      limit: 2,
      cursor: 5,
      since: DateTime.utc(2026, 8, 23, 1, 2, 3),
      level: 'ERROR',
      component: 'caddy',
    );
    expect(page.records.single.id, 4);
    expect(page.records.single.fields['error_code'], 'CADDY_START_FAILED');
    expect(page.nextCursor, 4);
    expect(page.hasMore, isTrue);
  });

  test(
    'management API reads configuration revisions and restores through davd',
    () async {
      final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      addTearDown(() => server.close(force: true));
      server.listen((request) async {
        request.response.headers.contentType = ContentType.json;
        Object data;
        switch ('${request.method} ${request.uri.path}') {
          case 'GET /api/v1/config/state':
            data = {
              'desired_revision': 2,
              'active_revision': 1,
              'dirty': false,
              'pending': true,
            };
          case 'GET /api/v1/revisions':
            data = [
              {
                'id': 'revision-2',
                'number': 2,
                'created_at': '2026-08-23T01:02:03Z',
                'config_hash': 'hash-2',
                'validation_status': 'VALID',
                'apply_status': 'APPLIED',
                'app_version': 'test',
              },
            ];
          case 'POST /api/v1/config/apply':
          case 'POST /api/v1/revisions/revision-2/restore':
            data = {
              'id': 'revision-2',
              'number': 2,
              'created_at': '2026-08-23T01:02:03Z',
              'config_hash': 'hash-2',
              'validation_status': 'VALID',
              'apply_status': 'APPLIED',
              'app_version': 'test',
            };
          default:
            request.response.statusCode = HttpStatus.notFound;
            data = <String, dynamic>{};
        }
        request.response.write(jsonEncode({'success': true, 'data': data}));
        await request.response.close();
      });
      final api = ManagementDaemonApi(
        discovery: FakeDiscovery(
          DaemonConnection(
            endpoint: Uri.parse('http://127.0.0.1:${server.port}'),
            token: 'token',
          ),
        ),
      );
      final state = await api.configurationState();
      expect(state.pending, isTrue);
      expect(state.activeRevision, 1);
      expect((await api.listRevisions()).single.number, 2);
      expect((await api.applyConfigurationResult()).configHash, 'hash-2');
      expect((await api.restoreRevision('revision-2')).number, 2);
    },
  );

  test(
    'management API exports safe YAML and imports it with the YAML media type',
    () async {
      final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      addTearDown(() => server.close(force: true));
      final yaml = 'version: 1\nusers: []\nshares: []\n';
      var importedBody = '';
      var importedContentType = '';
      server.listen((request) async {
        request.response.headers.contentType = ContentType.json;
        Object data;
        switch ('${request.method} ${request.uri.path}') {
          case 'GET /api/v1/config/export':
            data = {
              'format': 'yaml',
              'content': yaml,
              'contains_secrets': false,
            };
          case 'POST /api/v1/config/import':
            importedContentType =
                request.headers.value(HttpHeaders.contentTypeHeader) ?? '';
            importedBody = await utf8.decoder.bind(request).join();
            data = {
              'users_created': 1,
              'users_updated': 2,
              'shares_created': 1,
              'shares_updated': 0,
              'permissions_upserted': 3,
              'tls_updated': true,
              'server_updated': true,
              'password_reset_required': ['Alice'],
              'pending_apply': true,
            };
          default:
            request.response.statusCode = HttpStatus.notFound;
            data = <String, dynamic>{};
        }
        request.response.write(jsonEncode({'success': true, 'data': data}));
        await request.response.close();
      });
      final api = ManagementDaemonApi(
        discovery: FakeDiscovery(
          DaemonConnection(
            endpoint: Uri.parse('http://127.0.0.1:${server.port}'),
            token: 'token',
          ),
        ),
      );

      expect(await api.exportConfiguration(), yaml);
      final result = await api.importConfiguration(yaml);
      expect(importedContentType, 'application/yaml');
      expect(importedBody, yaml);
      expect(result.usersCreated, 1);
      expect(result.usersUpdated, 2);
      expect(result.permissionsUpserted, 3);
      expect(result.passwordResetRequired, ['Alice']);
      expect(result.pendingApply, isTrue);
    },
  );

  test(
    'management API rejects an export marked as containing secrets',
    () async {
      final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      addTearDown(() => server.close(force: true));
      server.listen((request) async {
        request.response.headers.contentType = ContentType.json;
        request.response.write(
          jsonEncode({
            'success': true,
            'data': {
              'format': 'yaml',
              'content': 'password: leaked',
              'contains_secrets': true,
            },
          }),
        );
        await request.response.close();
      });
      final api = ManagementDaemonApi(
        discovery: FakeDiscovery(
          DaemonConnection(
            endpoint: Uri.parse('http://127.0.0.1:${server.port}'),
            token: 'token',
          ),
        ),
      );

      await expectLater(
        api.exportConfiguration(),
        throwsA(
          isA<DaemonApiException>().having(
            (error) => error.code,
            'code',
            'UNSAFE_CONFIGURATION_EXPORT',
          ),
        ),
      );
    },
  );
}
