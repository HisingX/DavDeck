import 'package:flutter/widgets.dart';

class AppStrings {
  const AppStrings(this.locale);

  factory AppStrings.of(BuildContext context) =>
      AppStrings(Localizations.localeOf(context));

  final Locale locale;
  bool get _zh => locale.languageCode == 'zh';

  String get dashboard => _zh ? '仪表盘' : 'Dashboard';
  String get daemon => _zh ? '守护进程' : 'Daemon';
  String get database => _zh ? '数据库' : 'Database';
  String get schema => _zh ? '架构版本' : 'Schema';
  String get version => _zh ? '版本' : 'Version';
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
  String get addUser => _zh ? '添加用户' : 'Add user';
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
  String get addShare => _zh ? '添加共享' : 'Add share';
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
  String get runDiagnostics => _zh ? '运行诊断' : 'Run diagnostics';
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
  String get logs => _zh ? '日志' : 'Logs';
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
  String get daemonState => _zh ? '守护进程状态' : 'Daemon state';
  String get caddyState => _zh ? 'Caddy 状态' : 'Caddy state';
  String get webdavState => _zh ? 'WebDAV 状态' : 'WebDAV state';
  String get serviceState => _zh ? '系统服务状态' : 'System service state';
}
