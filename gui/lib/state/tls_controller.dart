import 'dart:async';

import 'package:davdeck/api/daemon_api.dart';
import 'package:flutter/foundation.dart';

class TlsController extends ChangeNotifier {
  TlsController(
    this.api,
    this.configurationApi, {
    this.dnsProviderApi,
    this.tlsDnsApi,
  });
  final TlsApi api;
  final ConfigurationApi configurationApi;
  final DnsProviderApi? dnsProviderApi;
  final TlsDnsApi? tlsDnsApi;

  bool loading = true;
  bool busy = false;
  Object? error;
  ManagedTlsProfile? profile;
  ManagedTlsCheckResult? checkResult;
  List<ManagedDnsProvider> dnsProviders = const [];
  bool pendingApply = false;
  Timer? _certificateStatusTimer;
  bool _refreshInFlight = false;

  Future<void> refresh() async {
    if (_refreshInFlight) return;
    _refreshInFlight = true;
    loading = true;
    error = null;
    notifyListeners();
    try {
      profile = await api.getTls();
      if (dnsProviderApi != null) {
        dnsProviders = await dnsProviderApi!.listDnsProviders();
      }
    } catch (caught) {
      error = caught;
    } finally {
      loading = false;
      _refreshInFlight = false;
      notifyListeners();
      _scheduleCertificateStatusRefresh();
    }
  }

  void _scheduleCertificateStatusRefresh() {
    _certificateStatusTimer?.cancel();
    _certificateStatusTimer = null;
    if (pendingApply || profile?.certificateStatus?.state != 'ISSUING') {
      return;
    }
    _certificateStatusTimer = Timer(const Duration(seconds: 3), () {
      unawaited(refresh());
    });
  }

  @override
  void dispose() {
    _certificateStatusTimer?.cancel();
    super.dispose();
  }

  Future<bool> configure({
    required String mode,
    required String hostname,
    String certificatePath = '',
    String privateKeyPath = '',
    String challenge = 'auto',
    String? dnsProviderId,
  }) async {
    busy = true;
    error = null;
    checkResult = null;
    notifyListeners();
    try {
      profile = challenge == 'dns' && tlsDnsApi != null
          ? await tlsDnsApi!.updateTlsWithChallenge(
              mode: mode,
              hostname: hostname,
              challenge: challenge,
              dnsProviderId: dnsProviderId,
              certificatePath: certificatePath,
              privateKeyPath: privateKeyPath,
            )
          : await api.updateTls(
              mode: mode,
              hostname: hostname,
              certificatePath: certificatePath,
              privateKeyPath: privateKeyPath,
            );
      pendingApply = true;
      return true;
    } catch (caught) {
      error = caught;
      return false;
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  bool get canManageDnsProviders => dnsProviderApi != null;

  Future<bool> saveDnsProvider({
    String? id,
    required String name,
    required String provider,
    List<String> allowedZones = const [],
    Map<String, String>? secret,
  }) async {
    final providerApi = dnsProviderApi;
    if (providerApi == null) return false;
    busy = true;
    error = null;
    notifyListeners();
    try {
      await providerApi.saveDnsProvider(
        id: id,
        name: name,
        provider: provider,
        allowedZones: allowedZones,
        secret: secret,
      );
      dnsProviders = await providerApi.listDnsProviders();
      pendingApply = true;
      return true;
    } catch (caught) {
      error = caught;
      return false;
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  Future<bool> deleteDnsProvider(String id) async {
    final providerApi = dnsProviderApi;
    if (providerApi == null) return false;
    busy = true;
    error = null;
    notifyListeners();
    try {
      await providerApi.deleteDnsProvider(id);
      dnsProviders = await providerApi.listDnsProviders();
      pendingApply = true;
      return true;
    } catch (caught) {
      error = caught;
      return false;
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  Future<bool> check() async {
    busy = true;
    error = null;
    checkResult = null;
    notifyListeners();
    try {
      checkResult = await api.checkTls();
      return true;
    } catch (caught) {
      error = caught;
      return false;
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  Future<bool> disable() async {
    busy = true;
    error = null;
    checkResult = null;
    notifyListeners();
    try {
      await api.disableTls();
      profile = null;
      pendingApply = true;
      return true;
    } catch (caught) {
      error = caught;
      return false;
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  Future<bool> cancelCertificateRequest() async {
    busy = true;
    error = null;
    checkResult = null;
    notifyListeners();
    try {
      await api.disableTls();
      profile = null;
      pendingApply = true;
      await configurationApi.applyConfiguration();
      pendingApply = false;
      return true;
    } catch (caught) {
      error = caught;
      return false;
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  Future<bool> apply() async {
    busy = true;
    error = null;
    notifyListeners();
    try {
      await configurationApi.applyConfiguration();
      pendingApply = false;
      return true;
    } catch (caught) {
      error = caught;
      return false;
    } finally {
      busy = false;
      notifyListeners();
    }
  }
}
