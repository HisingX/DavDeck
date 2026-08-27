import 'package:flutter/widgets.dart';

class AppStrings {
  const AppStrings(this.locale);

  factory AppStrings.of(BuildContext context) =>
      AppStrings(Localizations.localeOf(context));

  final Locale locale;
  bool get _zh => locale.languageCode == 'zh';

  String get dashboard => _zh ? '仪表盘' : 'Dashboard';
  String get dashboardSubtitle =>
      _zh ? '系统状态与服务管理概览' : 'System status and service management overview';
  String get refreshDashboard => _zh ? '刷新仪表盘' : 'Refresh dashboard';
  String get dashboardHealthy => _zh ? '运行正常' : 'Healthy';
  String get dashboardAttention => _zh ? '需要关注' : 'Needs attention';
  String get daemon => _zh ? '守护进程' : 'Daemon';
  String get database => _zh ? '数据库' : 'Database';
  String get schema => _zh ? '架构版本' : 'Schema';
  String get version => _zh ? '版本' : 'Version';
  String get daemonHealthy => _zh ? '进程运行正常' : 'Process is healthy';
  String get databaseHealthy =>
      _zh ? 'SQLite · 连接正常' : 'SQLite · connection ready';
  String get caddyDetail => _zh ? '反向代理服务' : 'Reverse proxy service';
  String get webdavDetail => _zh ? 'WebDAV 服务' : 'WebDAV service';
  String get localApiConnected =>
      _zh ? '本机管理 API 已连接' : 'Connected to the local management API';
  String get lastError => _zh ? '最近错误' : 'Last error';
  String stateLabel(String state) {
    if (!_zh) return state;
    return switch (state.toUpperCase()) {
      'RUNNING' => '运行中',
      'READY' => '就绪',
      'STOPPED' => '已停止',
      'STARTING' => '启动中',
      'STOPPING' => '停止中',
      'FAILED' => '失败',
      'DEGRADED' => '已降级',
      'NOT_INSTALLED' => '未安装',
      'ENABLED' => '已启用',
      'DISABLED' => '已停用',
      'YES' => '是',
      'NO' => '否',
      'UNKNOWN' => '未知',
      _ => state,
    };
  }

  String get yes => _zh ? '是' : 'Yes';
  String get no => _zh ? '否' : 'No';
  String get serviceControl => _zh ? '服务控制' : 'Service control';
  String get serviceControlSubtitle =>
      _zh ? '控制 DavDeck 核心服务的运行状态' : 'Control the DavDeck core service runtime';
  String get start => _zh ? '启动' : 'Start';
  String get stop => _zh ? '停止' : 'Stop';
  String get restart => _zh ? '重启' : 'Restart';
  String get restartService => _zh ? '重启服务' : 'Restart service';
  String get accessEndpoints => _zh ? '访问端点' : 'Access endpoints';
  String get accessEndpointsSubtitle => _zh
      ? '通过以下地址访问 DavDeck 服务'
      : 'Access DavDeck services at these local addresses';
  String get endpointCopied => _zh ? '端点地址已复制。' : 'Endpoint copied.';
  String get systemInformation => _zh ? '系统信息' : 'System information';
  String get runtimeMode => _zh ? '运行模式' : 'Runtime mode';
  String get portableMode => _zh ? '便携模式' : 'Portable';
  String get managedMode => _zh ? '系统服务' : 'System service';
  String get systemHealthy => _zh ? '系统运行正常' : 'System running normally';
  String get systemNeedsAttention => _zh ? '系统需要关注' : 'System needs attention';
  String get allServicesHealthy => _zh ? '所有服务健康' : 'All services healthy';
  String get checkDashboard => _zh ? '请查看仪表盘状态' : 'Check the dashboard status';
  String get loading => _zh ? '正在连接 DavDeck…' : 'Connecting to DavDeck…';
  String get unavailable =>
      _zh ? '无法连接到本机 DavDeck 服务。' : 'The local DavDeck service is unavailable.';
  String get retry => _zh ? '重试' : 'Retry';
  String get ports => _zh ? '端口' : 'Ports';
  String get httpPort => _zh ? 'HTTP 端口' : 'HTTP port';
  String get httpsPort => _zh ? 'HTTPS 端口' : 'HTTPS port';
  String get editPorts => _zh ? '修改端口' : 'Edit ports';
  String get portRequired =>
      _zh ? '请输入 1 至 65535 的端口。' : 'Enter a port from 1 to 65535.';
  String get portsMustDiffer =>
      _zh ? 'HTTP 与 HTTPS 端口不能相同。' : 'HTTP and HTTPS ports must differ.';
  String get savePorts => _zh ? '保存并应用' : 'Save and apply';
  String get savePortsFailed => _zh
      ? '无法保存端口设置，请确认端口未被占用。'
      : 'Unable to save ports. Confirm the ports are available.';
  String get caddyActionFailed => _zh
      ? 'Caddy 操作失败，请检查 Caddy 运行时配置。'
      : 'Caddy action failed. Check the Caddy runtime configuration.';
  String get users => _zh ? '用户' : 'Users';
  String get usersSubtitle =>
      _zh ? '管理 WebDAV 登录账户' : 'Manage WebDAV login accounts';
  String get addUser => _zh ? '添加用户' : 'Add user';
  String get searchUsersHint => _zh ? '搜索用户名' : 'Search users';
  String get clearSearch => _zh ? '清除搜索' : 'Clear search';
  String usersCount(int count) => _zh ? '共 $count 个账户' : '$count accounts';
  String get usersTotal => _zh ? '用户总数' : 'Total users';
  String get usersEnabled => _zh ? '已启用' : 'Enabled';
  String get usersDisabled => _zh ? '已停用' : 'Disabled';
  String get usersInactive => _zh ? '未启用' : 'Inactive';
  String get noMatchingUsers => _zh ? '没有匹配的用户。' : 'No matching users.';
  String get accountId => _zh ? '账户 ID' : 'Account ID';
  String get accountType => _zh ? '账户类型' : 'Account type';
  String get webdavAccount => _zh ? 'WebDAV 登录' : 'WebDAV login';
  String get userActions => _zh ? '用户操作' : 'User actions';
  String get noUsers => _zh ? '尚未添加用户。' : 'No users yet.';
  String get usersUnavailable => _zh ? '无法加载用户列表。' : 'Unable to load users.';
  String get enabled => _zh ? '已启用' : 'Enabled';
  String get disabled => _zh ? '已停用' : 'Disabled';
  String get username => _zh ? '用户名' : 'Username';
  String get password => _zh ? '密码' : 'Password';
  String get usernameRequired => _zh ? '请输入用户名。' : 'Enter a username.';
  String get passwordLengthRequirement => _zh
      ? '密码必须为 8 至 72 个 UTF-8 字节。'
      : 'Password must contain 8 to 72 UTF-8 bytes.';
  String get usernameAlreadyExists =>
      _zh ? '该用户名已存在。' : 'That username already exists.';
  String get invalidUsername => _zh ? '用户名无效。' : 'The username is invalid.';
  String get createUserFailed =>
      _zh ? '无法创建用户，请重试。' : 'Unable to create user. Please try again.';
  String get newPassword => _zh ? '新密码' : 'New password';
  String get changePassword => _zh ? '修改密码' : 'Change password';
  String get delete => _zh ? '删除' : 'Delete';
  String get deleteUser => _zh ? '删除用户' : 'Delete user';
  String get cancel => _zh ? '取消' : 'Cancel';
  String get create => _zh ? '创建' : 'Create';
  String get save => _zh ? '保存' : 'Save';
  String deleteUserPreservesFiles(String username) => _zh
      ? '确定删除用户“$username”吗？共享目录中的物理文件会保留。'
      : 'Delete user “$username”? Physical files in shares will be preserved.';
  String get shares => _zh ? '共享' : 'Shares';
  String get sharesSubtitle =>
      _zh ? '管理 WebDAV 目录与访问路径' : 'Manage WebDAV directories and access paths';
  String get addShare => _zh ? '添加共享' : 'Add share';
  String get searchSharesHint => _zh ? '搜索共享名称或路径' : 'Search shares or paths';
  String get sharesUnavailable => _zh ? '无法加载共享列表。' : 'Unable to load shares.';
  String get noMatchingShares => _zh ? '没有匹配的共享。' : 'No matching shares.';
  String get sharesTotal => _zh ? '共享总数' : 'Total shares';
  String get sharesEnabled => _zh ? '启用中' : 'Enabled';
  String get sharesDisabled => _zh ? '未启用' : 'Inactive';
  String get webdavPath => _zh ? 'WebDAV 路径' : 'WebDAV path';
  String get localDirectory => _zh ? '本地目录' : 'Local directory';
  String get protocol => _zh ? '协议' : 'Protocol';
  String get status => _zh ? '状态' : 'Status';
  String get shareActions => _zh ? '共享操作' : 'Share actions';
  String get editShare => _zh ? '编辑共享' : 'Edit share';
  String get deleteShare => _zh ? '删除共享' : 'Delete share';
  String get noShares => _zh ? '尚未添加共享。' : 'No shares yet.';
  String get shareName => _zh ? '共享名称' : 'Share name';
  String get slug => _zh ? 'URL 标识' : 'URL slug';
  String get folderPath => _zh ? '文件夹路径' : 'Folder path';
  String get permissions => _zh ? '权限' : 'Permissions';
  String get noAccess => _zh ? '无权限' : 'No access';
  String get readOnly => _zh ? '只读' : 'Read only';
  String get readWrite => _zh ? '读写' : 'Read & write';
  String get edit => _zh ? '编辑' : 'Edit';
  String get close => _zh ? '关闭' : 'Close';
  String deleteSharePreservesFiles(String name) => _zh
      ? '确定删除共享“$name”吗？只会移除元数据和权限，物理文件会保留。'
      : 'Delete share “$name”? Only metadata and permissions are removed; physical files are preserved.';
  String get https => _zh ? 'HTTPS' : 'HTTPS';
  String get httpsSubtitle => _zh
      ? '配置安全连接与证书策略'
      : 'Configure secure connections and certificate strategy';
  String get refreshHttps => _zh ? '刷新 HTTPS 设置' : 'Refresh HTTPS settings';
  String get httpsSettings => _zh ? 'HTTPS 设置' : 'HTTPS settings';
  String get certificateStatus => _zh ? '证书状态' : 'Certificate status';
  String get certificatePathShort => _zh ? '证书文件' : 'Certificate file';
  String get privateKeyPathShort => _zh ? '私钥文件' : 'Private-key file';
  String get configured => _zh ? '已配置' : 'Configured';
  String get notConfigured => _zh ? '未配置' : 'Not configured';
  String get tlsAutomaticTitle => _zh ? '公网自动证书' : 'Automatic certificate';
  String get tlsInternalTitle => _zh ? '内网证书模式' : 'Internal certificate mode';
  String get tlsCustomTitle => _zh ? '自定义证书模式' : 'Custom certificate mode';
  String get httpsWizardTitle =>
      _zh ? '配置安全连接' : 'Configure a secure connection';
  String get httpsWizardIntro => _zh
      ? '选择适合部署环境的证书方式。DavDeck 会生成并验证 Caddy 配置。'
      : 'Choose the certificate strategy for this deployment. DavDeck generates and validates the Caddy configuration.';
  String get tlsAutomatic => _zh ? '公网自动证书' : 'Automatic';
  String get tlsInternal => _zh ? '内网证书' : 'Internal';
  String get tlsCustom => _zh ? '自定义证书' : 'Custom';
  String get automaticTlsDescription => _zh
      ? '适用于已正确解析到本机的公网域名，由 Caddy 申请并续期证书。'
      : 'For a public hostname that resolves to this server. Caddy obtains and renews the certificate.';
  String get internalTlsDescription => _zh
      ? '适用于局域网或本地环境，由 Caddy 内部 CA 签发证书。'
      : 'For LAN or local environments. Caddy issues the certificate from its internal CA.';
  String get customTlsDescription => _zh
      ? '引用已有证书和私钥文件。DavDeck 不会复制私钥内容。'
      : 'Reference an existing certificate and private-key file. DavDeck never copies private-key contents.';
  String get internalTrustWarning => _zh
      ? '客户端必须信任 Caddy 的内部根证书，否则会显示证书警告。'
      : 'Clients must trust Caddy’s internal root certificate or they will show a certificate warning.';
  String get hostname => _zh ? '主机名' : 'Hostname';
  String get certificatePath => _zh ? '证书文件绝对路径' : 'Certificate absolute path';
  String get privateKeyPath => _zh ? '私钥文件绝对路径' : 'Private-key absolute path';
  String get privateKeyPathSafety => _zh
      ? '界面和诊断信息只保存路径，不读取或显示私钥内容。'
      : 'The UI and diagnostics retain only the path and never display private-key contents.';
  String get saveTlsSettings => _zh ? '保存 HTTPS 设置' : 'Save HTTPS settings';
  String get runPreflight => _zh ? '运行预检' : 'Run preflight';
  String get applyConfiguration => _zh ? '应用配置' : 'Apply configuration';
  String get pendingTlsApply => _zh
      ? 'HTTPS 设置已保存为期望状态，应用后才会改变运行中的 Caddy。'
      : 'HTTPS settings are saved as desired state. Apply them to change the running Caddy instance.';
  String get preflightReady => _zh ? '预检通过' : 'Preflight passed';
  String get preflightFailed => _zh ? '预检未通过' : 'Preflight failed';
  String get diagnostics => _zh ? '诊断' : 'Diagnostics';
  String get diagnosticsSubtitle => _zh
      ? '运行系统检查并查看问题建议'
      : 'Run system checks and review remediation suggestions';
  String get runDiagnostics => _zh ? '运行诊断' : 'Run diagnostics';
  String get diagnosticWarnings => _zh ? '个警告' : 'warnings';
  String get diagnosticPassed => _zh ? '项通过' : 'passed';
  String get runningDiagnostics =>
      _zh ? '正在运行安全诊断…' : 'Running safe diagnostics…';
  String get diagnosticsUnavailable =>
      _zh ? '无法运行诊断。' : 'Unable to run diagnostics.';
  String get openDiagnostics => _zh ? '打开诊断' : 'Open diagnostics';
  String get suggestedAction => _zh ? '建议操作' : 'Suggested action';
  String diagnosticRemediation(String code) {
    if (_zh) {
      return switch (code) {
        'CADDY_START_FAILED' ||
        'CADDY_RELOAD_FAILED' ||
        'CADDY_VALIDATE_FAILED' => '检查 Caddy 运行时和配置校验结果。',
        'RUNTIME_UNHEALTHY' || 'RUNTIME_STOPPED' => '检查 Caddy 状态并重新应用配置。',
        'PRIVILEGE_REQUIRED' => '使用管理员权限执行系统服务操作。',
        'TLS_CERTIFICATE_NOT_FOUND' ||
        'TLS_PRIVATE_KEY_NOT_FOUND' => '确认配置的证书和私钥路径仍然存在且可读。',
        'TLS_CONFIGURATION_ERROR' => '运行 HTTPS 预检并修正证书配置。',
        'SHARE_PATH_UNAVAILABLE' => '确认共享目录存在且 DavDeck 可以访问。',
        'DATABASE_UNAVAILABLE' || 'DATABASE_ERROR' => '检查 DavDeck 数据目录和文件权限。',
        _ => '',
      };
    }
    return switch (code) {
      'CADDY_START_FAILED' ||
      'CADDY_RELOAD_FAILED' ||
      'CADDY_VALIDATE_FAILED' =>
        'Check the Caddy runtime and configuration validation result.',
      'RUNTIME_UNHEALTHY' || 'RUNTIME_STOPPED' =>
        'Check Caddy status and apply the configuration again.',
      'PRIVILEGE_REQUIRED' =>
        'Run the service operation with administrator privileges.',
      'TLS_CERTIFICATE_NOT_FOUND' || 'TLS_PRIVATE_KEY_NOT_FOUND' =>
        'Confirm the configured certificate and private-key paths still exist and are readable.',
      'TLS_CONFIGURATION_ERROR' =>
        'Run HTTPS preflight and correct the certificate settings.',
      'SHARE_PATH_UNAVAILABLE' =>
        'Confirm the share directory exists and DavDeck can access it.',
      'DATABASE_UNAVAILABLE' || 'DATABASE_ERROR' =>
        'Check the DavDeck data directory and file permissions.',
      _ => '',
    };
  }

  String get diagnosticsNotRun =>
      _zh ? '尚未运行诊断。' : 'Diagnostics have not been run yet.';
  String get sanitizedReportNotice => _zh
      ? '此报告已脱敏，不包含密码、管理令牌、密码哈希、私钥内容或完整文件路径。'
      : 'This report is sanitized and excludes passwords, management tokens, password hashes, private-key contents, and full filesystem paths.';
  String diagnosticOverall(String status) =>
      _zh ? '总体状态：$status' : 'Overall status: $status';
  String get generatedAt => _zh ? '生成时间' : 'Generated';
  String get pendingConfiguration =>
      _zh ? '配置变更待应用。' : 'Configuration changes are pending.';
  String configurationAppliedRevision(int number) =>
      _zh ? '已应用配置版本 $number。' : 'Configuration revision $number applied.';
  String get revisions => _zh ? '版本' : 'Revisions';
  String get revisionsSubtitle =>
      _zh ? '查看与恢复配置版本' : 'Review and restore configuration revisions';
  String get revisionHistory => _zh ? '版本历史' : 'Revision history';
  String revisionsCount(int count) => _zh ? '共 $count 个版本' : '$count revisions';
  String get currentRevision => _zh ? '当前版本' : 'Current version';
  String get refreshRevisions => _zh ? '刷新版本' : 'Refresh revisions';
  String get revisionsLoading =>
      _zh ? '正在加载配置版本…' : 'Loading configuration revisions…';
  String get revisionsUnavailable =>
      _zh ? '无法加载配置版本。' : 'Unable to load configuration revisions.';
  String get noRevisions => _zh ? '暂无配置版本。' : 'No configuration revisions.';
  String get configurationState => _zh ? '配置状态' : 'Configuration state';
  String get desiredRevision => _zh ? '期望版本' : 'Desired';
  String get activeRevision => _zh ? '活动版本' : 'Active';
  String get none => _zh ? '无' : 'none';
  String get pending => _zh ? '待应用' : 'Pending';
  String get applied => _zh ? '已应用' : 'Applied';
  String get revision => _zh ? '版本' : 'Revision';
  String get validation => _zh ? '校验' : 'Validation';
  String get created => _zh ? '创建时间' : 'Created';
  String get configHash => _zh ? '配置哈希' : 'Config hash';
  String get restoreRevision =>
      _zh ? '恢复配置版本' : 'Restore configuration revision';
  String confirmRestoreRevision(int number) => _zh
      ? '确定恢复配置版本 $number 吗？这会重新校验并切换运行中的 Caddy 配置。'
      : 'Restore configuration revision $number? DavDeck will revalidate it and switch the running Caddy configuration.';
  String get restore => _zh ? '恢复' : 'Restore';
  String get restoring => _zh ? '正在恢复…' : 'Restoring…';
  String get deleteRevision => _zh ? '删除配置版本' : 'Delete configuration revision';
  String confirmDeleteRevision(int number) => _zh
      ? '确定删除配置版本 $number 吗？这只会删除版本快照，不会删除共享目录中的文件。'
      : 'Delete configuration revision $number? This removes only the revision snapshot and never shared files.';
  String get deleting => _zh ? '正在删除…' : 'Deleting…';
  String get logs => _zh ? '日志' : 'Logs';
  String get logsSubtitle =>
      _zh ? '查看系统与服务运行日志' : 'Review system and service runtime logs';
  String get searchLogsHint => _zh ? '搜索日志内容…' : 'Search log messages…';
  String get noMatchingLogs => _zh ? '没有匹配的日志。' : 'No matching logs.';
  String logsCount(int count) => _zh ? '共 $count 条日志' : '$count log entries';
  String get logLevel => _zh ? '级别' : 'Level';
  String get logComponent => _zh ? '组件' : 'Component';
  String get allLevels => _zh ? '全部级别' : 'All levels';
  String get componentFilter => _zh ? '组件筛选' : 'Component filter';
  String get applyFilter => _zh ? '应用筛选' : 'Apply filter';
  String get refreshLogs => _zh ? '刷新日志' : 'Refresh logs';
  String get autoRefresh => _zh ? '自动刷新（30 秒）' : 'Auto-refresh (30s)';
  String get pauseRefresh => _zh ? '暂停自动刷新' : 'Pause auto-refresh';
  String get copyLogs => _zh ? '复制日志' : 'Copy logs';
  String get exportLogs => _zh ? '导出日志' : 'Export logs';
  String get logsLoading => _zh ? '正在加载日志…' : 'Loading logs…';
  String get logsUnavailable => _zh ? '无法加载日志。' : 'Unable to load logs.';
  String get noLogs => _zh ? '暂无日志。' : 'No recent logs.';
  String get loadMoreLogs => _zh ? '加载更多' : 'Load more';
  String get logsCopied => _zh ? '已复制已脱敏日志。' : 'Sanitized logs copied.';
  String logsExportedTo(String path) =>
      _zh ? '已导出已脱敏日志：$path' : 'Sanitized logs exported to $path';
  String get logsExportFailed => _zh ? '导出日志失败。' : 'Unable to export logs.';
  String get logDetails => _zh ? '结构化字段' : 'Structured fields';
  String get service => _zh ? '服务' : 'Service';
  String get serviceSubtitle => _zh
      ? '管理守护进程、Caddy 与系统服务'
      : 'Manage the daemon, Caddy, and system service';
  String get serviceManagement => _zh ? '服务管理' : 'Service management';
  String get serviceLoading => _zh ? '正在加载服务状态…' : 'Loading service status…';
  String get serviceUnavailable =>
      _zh ? '无法读取系统服务状态。' : 'Unable to read system service status.';
  String get refreshService => _zh ? '刷新服务状态' : 'Refresh service status';
  String get installService => _zh ? '安装服务' : 'Install service';
  String get uninstallService => _zh ? '卸载服务' : 'Uninstall service';
  String get startService => _zh ? '启动服务' : 'Start service';
  String get stopService => _zh ? '停止服务' : 'Stop service';
  String get serviceInstalled => _zh ? '已安装' : 'Installed';
  String get serviceNotInstalled => _zh ? '未安装' : 'Not installed';
  String get startsAtBoot => _zh ? '登录/启动时运行' : 'Starts at boot/login';
  String get serviceActionFailed => _zh
      ? '服务操作失败，请检查权限和系统服务状态。'
      : 'Service action failed. Check permissions and service state.';
  String confirmServiceAction(String action) => _zh
      ? '确定要执行“$action”吗？这会改变 DavDeck 的系统服务状态。'
      : 'Run “$action”? This changes DavDeck’s system service state.';
  String get portableDaemonNote => _zh
      ? '当前守护进程由 GUI 以便携模式启动。GUI 只管理自己启动的进程，不会停止独立系统服务。'
      : 'The daemon is owned by the GUI in portable mode. The GUI only manages the process it launched and will not stop an independent system service.';
  String get openService => _zh ? '打开服务管理' : 'Open service management';
  String get openLogs => _zh ? '查看日志' : 'View logs';
  String get startServiceDescription => _zh ? '启动服务' : 'Start the service';
  String get stopServiceDescription => _zh ? '停止服务' : 'Stop the service';
  String get restartServiceDescription =>
      _zh ? '重新启动服务' : 'Restart the service';
  String get installServiceDescription =>
      _zh ? '安装为系统服务' : 'Install as a system service';
  String get openLogsDescription => _zh ? '查看服务日志' : 'Review service logs';
  String get systemServiceTitle =>
      _zh ? '系统服务（开机启动）' : 'System service (start at boot)';
  String get systemServiceSubtitle => _zh
      ? '管理 DavDeck 系统服务，支持开机自启动'
      : 'Manage the DavDeck system service and boot behavior';
  String get portableModeLabel =>
      _zh ? '当前模式：GUI 便携模式' : 'Current mode: GUI portable mode';
  String get serviceInstalledDescription =>
      _zh ? 'DavDeck 系统服务已安装。' : 'The DavDeck system service is installed.';
  String get serviceNotInstalledDescription =>
      _zh ? '未检测到 DavDeck 系统服务。' : 'No DavDeck system service was detected.';
  String get serviceExplanationTitle => _zh ? '说明' : 'About service management';
  String get daemonState => _zh ? '守护进程状态' : 'Daemon state';
  String get caddyState => _zh ? 'Caddy 状态' : 'Caddy state';
  String get webdavState => _zh ? 'WebDAV 状态' : 'WebDAV state';
  String get serviceState => _zh ? '系统服务状态' : 'System service state';
}
