import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/dashboard/dashboard_page.dart';
import 'package:davdeck/diagnostics/diagnostics_page.dart';
import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/logs/logs_page.dart';
import 'package:davdeck/service/service_page.dart';
import 'package:davdeck/shares/shares_page.dart';
import 'package:davdeck/state/shares_controller.dart';
import 'package:davdeck/state/diagnostics_controller.dart';
import 'package:davdeck/state/logs_controller.dart';
import 'package:davdeck/state/revision_controller.dart';
import 'package:davdeck/state/service_controller.dart';
import 'package:davdeck/state/status_controller.dart';
import 'package:davdeck/state/tls_controller.dart';
import 'package:davdeck/state/users_controller.dart';
import 'package:davdeck/tls/tls_page.dart';
import 'package:davdeck/users/users_page.dart';
import 'package:davdeck/revisions/revisions_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';

void main() => runApp(const DavDeckApp());

class DavDeckApp extends StatefulWidget {
  const DavDeckApp({super.key, this.api, this.locale});

  final ManagementApi? api;
  final Locale? locale;

  @override
  State<DavDeckApp> createState() => _DavDeckAppState();
}

class _DavDeckAppState extends State<DavDeckApp> {
  late final StatusController controller;
  late final UsersController usersController;
  late final SharesController sharesController;
  late final TlsController tlsController;
  late final DiagnosticsController diagnosticsController;
  late final LogsController logsController;
  late final ServiceController serviceController;
  late final RevisionController? revisionController;

  @override
  void initState() {
    super.initState();
    final api = widget.api ?? LocalDaemonApi();
    final RevisionApi? revisionApi = api is RevisionApi
        ? api as RevisionApi
        : null;
    controller = StatusController(api, api, api, api, revisionApi)..refresh();
    usersController = UsersController(api)..refresh();
    sharesController = SharesController(api)..refresh();
    tlsController = TlsController(api, api)..refresh();
    diagnosticsController = DiagnosticsController(api);
    logsController = LogsController(api)..refresh();
    serviceController = ServiceController(api, onChanged: controller.refresh)
      ..refresh();
    revisionController = revisionApi == null
        ? null
        : (RevisionController(revisionApi)..refresh());
  }

  @override
  void dispose() {
    controller.dispose();
    usersController.dispose();
    sharesController.dispose();
    tlsController.dispose();
    diagnosticsController.dispose();
    logsController.dispose();
    serviceController.dispose();
    revisionController?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'DavDeck',
      debugShowCheckedModeBanner: false,
      locale: widget.locale,
      supportedLocales: const [Locale('en'), Locale('zh', 'CN')],
      localizationsDelegates: GlobalMaterialLocalizations.delegates,
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xFF356859)),
        useMaterial3: true,
      ),
      home: _AppShell(
        status: controller,
        users: usersController,
        shares: sharesController,
        tls: tlsController,
        diagnostics: diagnosticsController,
        logs: logsController,
        service: serviceController,
        revisions: revisionController,
      ),
    );
  }
}

class _AppShell extends StatefulWidget {
  const _AppShell({
    required this.status,
    required this.users,
    required this.shares,
    required this.tls,
    required this.diagnostics,
    required this.logs,
    required this.service,
    required this.revisions,
  });
  final StatusController status;
  final UsersController users;
  final SharesController shares;
  final TlsController tls;
  final DiagnosticsController diagnostics;
  final LogsController logs;
  final ServiceController service;
  final RevisionController? revisions;
  @override
  State<_AppShell> createState() => _AppShellState();
}

class _AppShellState extends State<_AppShell> {
  int selected = 0;
  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    return Scaffold(
      body: Row(
        children: [
          NavigationRail(
            selectedIndex: selected,
            onDestinationSelected: (value) => setState(() => selected = value),
            labelType: NavigationRailLabelType.all,
            destinations: [
              NavigationRailDestination(
                icon: const Icon(Icons.dashboard_outlined),
                selectedIcon: const Icon(Icons.dashboard),
                label: Text(strings.dashboard),
              ),
              NavigationRailDestination(
                icon: const Icon(Icons.people_outline),
                selectedIcon: const Icon(Icons.people),
                label: Text(strings.users),
              ),
              NavigationRailDestination(
                icon: const Icon(Icons.folder_shared_outlined),
                selectedIcon: const Icon(Icons.folder_shared),
                label: Text(strings.shares),
              ),
              NavigationRailDestination(
                icon: const Icon(Icons.lock_outline),
                selectedIcon: const Icon(Icons.lock),
                label: Text(strings.https),
              ),
              NavigationRailDestination(
                icon: const Icon(Icons.miscellaneous_services_outlined),
                selectedIcon: const Icon(Icons.miscellaneous_services),
                label: Text(strings.service),
              ),
              NavigationRailDestination(
                icon: const Icon(Icons.receipt_long_outlined),
                selectedIcon: const Icon(Icons.receipt_long),
                label: Text(strings.logs),
              ),
              NavigationRailDestination(
                icon: const Icon(Icons.health_and_safety_outlined),
                selectedIcon: const Icon(Icons.health_and_safety),
                label: Text(strings.diagnostics),
              ),
              if (widget.revisions != null)
                NavigationRailDestination(
                  icon: const Icon(Icons.history_outlined),
                  selectedIcon: const Icon(Icons.history),
                  label: Text(strings.revisions),
                ),
            ],
          ),
          const VerticalDivider(width: 1),
          Expanded(
            child: IndexedStack(
              index: selected,
              children: [
                DashboardPage(
                  controller: widget.status,
                  onOpenService: () => setState(() => selected = 4),
                ),
                UsersPage(controller: widget.users),
                SharesPage(controller: widget.shares),
                TlsPage(controller: widget.tls),
                ServicePage(
                  status: widget.status,
                  controller: widget.service,
                  onOpenDiagnostics: () => setState(() => selected = 6),
                ),
                LogsPage(
                  controller: widget.logs,
                  onOpenDiagnostics: () => setState(() => selected = 6),
                ),
                DiagnosticsPage(controller: widget.diagnostics),
                if (widget.revisions != null)
                  RevisionsPage(controller: widget.revisions!),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
