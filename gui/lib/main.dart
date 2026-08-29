import 'package:davdeck/api/daemon_api.dart';
import 'package:davdeck/about/about_page.dart';
import 'package:davdeck/dashboard/dashboard_page.dart';
import 'package:davdeck/diagnostics/diagnostics_page.dart';
import 'package:davdeck/desktop/desktop_lifecycle.dart';
import 'package:davdeck/l10n/app_strings.dart';
import 'package:davdeck/logs/logs_page.dart';
import 'package:davdeck/shares/shares_page.dart';
import 'package:davdeck/settings/settings_page.dart';
import 'package:davdeck/state/shares_controller.dart';
import 'package:davdeck/state/backup_controller.dart';
import 'package:davdeck/state/diagnostics_controller.dart';
import 'package:davdeck/state/logs_controller.dart';
import 'package:davdeck/state/revision_controller.dart';
import 'package:davdeck/state/status_controller.dart';
import 'package:davdeck/state/tls_controller.dart';
import 'package:davdeck/state/users_controller.dart';
import 'package:davdeck/tls/tls_page.dart';
import 'package:davdeck/users/users_page.dart';
import 'package:davdeck/revisions/revisions_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';

final _desktopLifecycle = DesktopLifecycle();

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await _desktopLifecycle.initialize();
  runApp(const DavDeckApp());
}

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
  late final RevisionController? revisionController;
  late final BackupController? backupController;

  @override
  void initState() {
    super.initState();
    final api = widget.api ?? LocalDaemonApi();
    final RevisionApi? revisionApi = api is RevisionApi
        ? api as RevisionApi
        : null;
    controller = StatusController(api, api, api, api, revisionApi, api)
      ..refresh();
    usersController = UsersController(api)..refresh();
    sharesController = SharesController(api)..refresh();
    tlsController = TlsController(api, api)..refresh();
    diagnosticsController = DiagnosticsController(api);
    logsController = LogsController(api, startAutoRefresh: true)..refresh();
    revisionController = revisionApi == null
        ? null
        : (RevisionController(
            revisionApi,
            onRestored: _refreshAfterConfigurationStateRestore,
          )..refresh());
    final BackupApi? backupApi = api is BackupApi ? api as BackupApi : null;
    backupController = backupApi == null ? null : BackupController(backupApi);
  }

  Future<void> _refreshAfterConfigurationStateRestore() async {
    await Future.wait([
      controller.refresh(),
      usersController.refresh(),
      sharesController.refresh(),
      tlsController.refresh(),
    ]);
  }

  Future<void> _refreshAfterConfigurationImport() async {
    final refreshes = <Future<void>>[_refreshAfterConfigurationStateRestore()];
    if (revisionController != null) {
      refreshes.add(revisionController!.refresh());
    }
    await Future.wait(refreshes);
  }

  @override
  void dispose() {
    controller.dispose();
    usersController.dispose();
    sharesController.dispose();
    tlsController.dispose();
    diagnosticsController.dispose();
    logsController.dispose();
    revisionController?.dispose();
    backupController?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    const scheme = ColorScheme.light(
      primary: Color(0xFF087A55),
      onPrimary: Colors.white,
      primaryContainer: Color(0xFFE5F4ED),
      onPrimaryContainer: Color(0xFF075D43),
      secondary: Color(0xFF47655A),
      onSecondary: Colors.white,
      secondaryContainer: Color(0xFFEEF6F2),
      onSecondaryContainer: Color(0xFF294B3F),
      error: Color(0xFFBA1A1A),
      onError: Colors.white,
      errorContainer: Color(0xFFFFDAD6),
      onErrorContainer: Color(0xFF93000A),
      surface: Colors.white,
      onSurface: Color(0xFF18211D),
      onSurfaceVariant: Color(0xFF69756F),
      outline: Color(0xFFC8D3CD),
      outlineVariant: Color(0xFFDEE6E1),
      shadow: Color(0xFF16251E),
      surfaceContainerLowest: Colors.white,
      surfaceContainerLow: Color(0xFFF8FBF9),
      surfaceContainer: Color(0xFFF3F7F5),
      surfaceContainerHigh: Color(0xFFEDF3F0),
      surfaceContainerHighest: Color(0xFFE6EDE9),
    );
    final base = ThemeData(colorScheme: scheme, useMaterial3: true);
    return MaterialApp(
      title: 'DavDeck',
      debugShowCheckedModeBanner: false,
      locale: widget.locale,
      supportedLocales: const [Locale('en'), Locale('zh', 'CN')],
      localizationsDelegates: GlobalMaterialLocalizations.delegates,
      theme: base.copyWith(
        scaffoldBackgroundColor: const Color(0xFFFAFCFB),
        canvasColor: const Color(0xFFFAFCFB),
        dividerColor: scheme.outlineVariant,
        textTheme: base.textTheme.copyWith(
          headlineLarge: base.textTheme.headlineLarge?.copyWith(
            fontSize: 34,
            height: 1.18,
            fontWeight: FontWeight.w700,
            letterSpacing: -0.8,
          ),
          headlineMedium: base.textTheme.headlineMedium?.copyWith(
            fontSize: 34,
            height: 1.18,
            fontWeight: FontWeight.w700,
            letterSpacing: -0.7,
          ),
          titleLarge: base.textTheme.titleLarge?.copyWith(
            fontSize: 22,
            height: 1.3,
            fontWeight: FontWeight.w700,
          ),
          titleMedium: base.textTheme.titleMedium?.copyWith(
            fontSize: 17,
            height: 1.35,
            fontWeight: FontWeight.w600,
          ),
          bodyLarge: base.textTheme.bodyLarge?.copyWith(
            fontSize: 16,
            height: 1.5,
          ),
          bodyMedium: base.textTheme.bodyMedium?.copyWith(
            fontSize: 14,
            height: 1.45,
          ),
          bodySmall: base.textTheme.bodySmall?.copyWith(
            fontSize: 12,
            height: 1.4,
          ),
        ),
        cardTheme: const CardThemeData(
          color: Colors.white,
          surfaceTintColor: Colors.transparent,
          elevation: 0,
          margin: EdgeInsets.zero,
        ),
        filledButtonTheme: FilledButtonThemeData(
          style: FilledButton.styleFrom(
            minimumSize: const Size(0, 48),
            padding: const EdgeInsets.symmetric(horizontal: 20),
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(10),
            ),
            textStyle: const TextStyle(fontWeight: FontWeight.w600),
          ),
        ),
        outlinedButtonTheme: OutlinedButtonThemeData(
          style: OutlinedButton.styleFrom(
            minimumSize: const Size(0, 48),
            padding: const EdgeInsets.symmetric(horizontal: 20),
            side: const BorderSide(color: Color(0xFFC8D3CD)),
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(10),
            ),
            textStyle: const TextStyle(fontWeight: FontWeight.w600),
          ),
        ),
        inputDecorationTheme: InputDecorationTheme(
          filled: true,
          fillColor: Colors.white,
          contentPadding: const EdgeInsets.symmetric(
            horizontal: 16,
            vertical: 15,
          ),
          enabledBorder: OutlineInputBorder(
            borderRadius: BorderRadius.circular(12),
            borderSide: const BorderSide(color: Color(0xFFC8D3CD)),
          ),
          focusedBorder: OutlineInputBorder(
            borderRadius: BorderRadius.circular(12),
            borderSide: const BorderSide(color: Color(0xFF087A55), width: 1.5),
          ),
          border: OutlineInputBorder(borderRadius: BorderRadius.circular(12)),
        ),
        switchTheme: SwitchThemeData(
          thumbColor: WidgetStateProperty.resolveWith(
            (states) => states.contains(WidgetState.selected)
                ? Colors.white
                : const Color(0xFF87918C),
          ),
          trackColor: WidgetStateProperty.resolveWith(
            (states) => states.contains(WidgetState.selected)
                ? scheme.primary
                : const Color(0xFFDCE3DF),
          ),
          trackOutlineColor: const WidgetStatePropertyAll(Colors.transparent),
        ),
        tooltipTheme: TooltipThemeData(
          decoration: BoxDecoration(
            color: const Color(0xFF414844),
            borderRadius: BorderRadius.circular(6),
          ),
          textStyle: const TextStyle(color: Colors.white, fontSize: 12),
        ),
      ),
      home: _AppShell(
        status: controller,
        users: usersController,
        shares: sharesController,
        tls: tlsController,
        diagnostics: diagnosticsController,
        logs: logsController,
        revisions: revisionController,
        backup: backupController,
        onConfigurationImported: _refreshAfterConfigurationImport,
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
    required this.revisions,
    required this.backup,
    required this.onConfigurationImported,
  });
  final StatusController status;
  final UsersController users;
  final SharesController shares;
  final TlsController tls;
  final DiagnosticsController diagnostics;
  final LogsController logs;
  final RevisionController? revisions;
  final BackupController? backup;
  final Future<void> Function() onConfigurationImported;
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
          _Sidebar(
            selected: selected,
            strings: strings,
            status: widget.status,
            hasRevisions: widget.revisions != null,
            onSelected: (value) => setState(() => selected = value),
          ),
          const VerticalDivider(width: 1),
          Expanded(
            child: IndexedStack(
              index: selected,
              children: [
                DashboardPage(controller: widget.status),
                UsersPage(controller: widget.users),
                SharesPage(controller: widget.shares),
                TlsPage(controller: widget.tls, status: widget.status),
                LogsPage(
                  controller: widget.logs,
                  onOpenDiagnostics: () => setState(() => selected = 5),
                ),
                DiagnosticsPage(controller: widget.diagnostics),
                AboutPage(controller: widget.status),
                if (widget.revisions != null)
                  RevisionsPage(controller: widget.revisions!),
                SettingsPage(
                  controller: widget.backup,
                  onConfigurationImported: widget.onConfigurationImported,
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _Sidebar extends StatelessWidget {
  const _Sidebar({
    required this.selected,
    required this.strings,
    required this.status,
    required this.hasRevisions,
    required this.onSelected,
  });

  final int selected;
  final AppStrings strings;
  final StatusController status;
  final bool hasRevisions;
  final ValueChanged<int> onSelected;

  @override
  Widget build(BuildContext context) {
    final destinations = [
      _SidebarDestination(
        Icons.dashboard_outlined,
        Icons.dashboard,
        strings.dashboard,
      ),
      _SidebarDestination(Icons.people_outline, Icons.people, strings.users),
      _SidebarDestination(
        Icons.folder_shared_outlined,
        Icons.folder_shared,
        strings.shares,
      ),
      _SidebarDestination(Icons.lock_outline, Icons.lock, strings.https),
      _SidebarDestination(
        Icons.receipt_long_outlined,
        Icons.receipt_long,
        strings.logs,
      ),
      _SidebarDestination(
        Icons.health_and_safety_outlined,
        Icons.health_and_safety,
        strings.diagnostics,
      ),
      _SidebarDestination(Icons.info_outline, Icons.info, strings.about),
      if (hasRevisions)
        _SidebarDestination(
          Icons.history_outlined,
          Icons.history,
          strings.revisions,
        ),
      _SidebarDestination(
        Icons.settings_outlined,
        Icons.settings,
        strings.settings,
      ),
    ];
    return SizedBox(
      width: 250,
      child: Container(
        color: const Color(0xFFFCFDFC),
        child: Column(
          children: [
            Expanded(
              child: ListView(
                padding: const EdgeInsets.fromLTRB(16, 32, 16, 18),
                children: [
                  Padding(
                    padding: const EdgeInsets.fromLTRB(2, 0, 2, 28),
                    child: Row(
                      children: [
                        Image.asset(
                          davDeckLogoAsset,
                          width: 42,
                          height: 42,
                          fit: BoxFit.contain,
                        ),
                        const SizedBox(width: 12),
                        Text(
                          'DavDeck',
                          style: Theme.of(context).textTheme.titleLarge
                              ?.copyWith(
                                fontWeight: FontWeight.w800,
                                letterSpacing: -0.4,
                              ),
                        ),
                      ],
                    ),
                  ),
                  for (var i = 0; i < destinations.length; i++)
                    _SidebarItem(
                      destination: destinations[i],
                      selected: selected == i,
                      onTap: () => onSelected(i),
                    ),
                ],
              ),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 0, 16, 18),
              child: AnimatedBuilder(
                animation: status,
                builder: (context, _) =>
                    _SidebarStatus(status: status, strings: strings),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _SidebarDestination {
  const _SidebarDestination(this.icon, this.selectedIcon, this.label);

  final IconData icon;
  final IconData selectedIcon;
  final String label;
}

class _SidebarItem extends StatelessWidget {
  const _SidebarItem({
    required this.destination,
    required this.selected,
    required this.onTap,
  });

  final _SidebarDestination destination;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final primary = Theme.of(context).colorScheme.primary;
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Material(
        color: selected ? const Color(0xFFE7F5EF) : Colors.transparent,
        borderRadius: BorderRadius.circular(13),
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(13),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 16),
            child: Row(
              children: [
                Icon(
                  selected ? destination.selectedIcon : destination.icon,
                  size: 24,
                  color: selected ? primary : const Color(0xFF4E5955),
                ),
                const SizedBox(width: 18),
                Expanded(
                  child: Text(
                    destination.label,
                    overflow: TextOverflow.ellipsis,
                    style: Theme.of(context).textTheme.titleMedium?.copyWith(
                      color: selected ? primary : const Color(0xFF313A37),
                      fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _SidebarStatus extends StatelessWidget {
  const _SidebarStatus({required this.status, required this.strings});

  final StatusController status;
  final AppStrings strings;

  @override
  Widget build(BuildContext context) {
    final snapshot = status.status;
    final endpoints = status.endpoints;
    final overallState = snapshot == null
        ? null
        : dashboardOverallState(snapshot, endpoints);
    final healthy = overallState == 'RUNNING';
    final dotColor = switch (overallState) {
      'RUNNING' => const Color(0xFF39B864),
      'FAILED' => Theme.of(context).colorScheme.error,
      'DEGRADED' => const Color(0xFFE1A928),
      _ => const Color(0xFF9BA19F),
    };
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: const Color(0xFFFCFDFC),
        border: Border.all(color: const Color(0xFFE1E8E4)),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 9,
                height: 9,
                decoration: BoxDecoration(
                  color: dotColor,
                  shape: BoxShape.circle,
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  snapshot == null
                      ? strings.loading
                      : healthy
                      ? strings.systemHealthy
                      : _sidebarStateLabel(overallState!, strings),
                  overflow: TextOverflow.ellipsis,
                  style: Theme.of(
                    context,
                  ).textTheme.labelLarge?.copyWith(fontWeight: FontWeight.w700),
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text(
            snapshot == null
                ? strings.localApiConnected
                : healthy
                ? strings.allComponentsHealthy
                : switch (overallState) {
                    'STOPPED' => strings.runtimeStoppedHint,
                    'STARTING' => strings.runtimeStartingHint,
                    'STOPPING' => strings.runtimeStoppingHint,
                    _ => strings.checkDashboard,
                  },
            style: Theme.of(
              context,
            ).textTheme.bodySmall?.copyWith(color: const Color(0xFF7A8580)),
          ),
        ],
      ),
    );
  }
}

String _sidebarStateLabel(String state, AppStrings strings) =>
    switch (state.toUpperCase()) {
      'STOPPED' => strings.dashboardRuntimeStopped,
      'STARTING' => strings.dashboardRuntimeStarting,
      'STOPPING' => strings.dashboardRuntimeStopping,
      _ => strings.systemNeedsAttention,
    };
