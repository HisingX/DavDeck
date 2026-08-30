# DavDeck 用户手册

本文适用于已发布构建，覆盖 macOS ARM64 桌面版、Windows x64 桌面目标、Linux x64
Desktop 版本，以及 Linux x64/ARM64 Server 版本。具体构建版本请查看压缩包中的
`manifest.json`，或执行 `davctl version --json`。平台差异会在对应章节中明确说明。

## 1. 选择运行方式

DavDeck 由一个后端和两个管理客户端组成：

- `davd` 负责数据库、业务规则、Caddy 配置生成、运行时生命周期和本机
  Management API。
- 原生 GUI 和 `davctl` 都是这个 API 的客户端。
- 两个客户端都不会直接访问 SQLite 或 Caddy Admin API。

桌面环境适合使用 GUI 完成可视化配置和诊断；服务器、SSH 或没有桌面会话的环境
使用守护进程和 CLI。WebDAV 服务可以通过局域网或反向代理对外提供，但 Management
API 和 Caddy Admin API 始终只监听本机回环地址。

## 2. 安装或构建

### 发布压缩包

每个压缩包包含以下通用文件：

```text
bin/davd
bin/davctl
libexec/caddy
manifest.json
README.md
README.zh-CN.md
LICENSE
NOTICE
SECURITY.md
```

Linux 下载包明确分为 flavor：无头 systemd 主机请选择
`linux-amd64-server` 或 `linux-arm64-server`，Linux GUI 请选择
`linux-amd64-desktop`。Server 包还包含 `install.sh`、`uninstall.sh` 和
`systemd/` 模板；Desktop 包根目录提供可直接运行的 `davdeck` 启动入口。

Windows 文件名会带 `.exe` 后缀。Windows 桌面压缩包会把 `DavDeck.exe`、
`flutter_windows.dll` 和 Flutter 的 `data/` 目录放在压缩包根目录。macOS 桌面压缩包的
原生应用位于 `desktop/`；Linux Desktop 压缩包的启动入口为根目录的 `davdeck`，Flutter
文件位于 `app/`。

运行前：

1. 使用单独发布的校验文件核对 SHA-256。
2. 阅读 `manifest.json`，确认操作系统、架构和版本。
3. 保持压缩包目录结构，以便 `davd` 找到固定版本的 Caddy。

当前发布压缩包暂未签名。macOS 可能显示 Gatekeeper 警告，Windows 可能显示未签名
二进制警告。

### 从源码构建

安装项目文档中列出的版本后执行：

```bash
make core-build
```

构建 GUI：

```bash
make gui-build-macos
```

当前 GUI 构建目标是 macOS ARM64、Windows x64 和 Linux x64。当前发布目标已完成
Windows 原生 GUI 和 ACL 验证；Windows reparse-point/junction 隔离仍是单独的安全发布门槛。

## 3. 启动守护进程

### Linux Server 安装

在 Linux Server 压缩包根目录执行：

```bash
sudo ./install.sh
davctl
```

安装脚本会检查操作系统和架构，把内置程序安装到 `/opt/davdeck`，创建
`/var/lib/davdeck`、`/etc/davdeck` 和由 systemd 管理的 `/run/davdeck`，然后
执行 `systemctl enable --now davdeck` 及 `davctl status` 冒烟检查。CLI 会自动
发现安装后的 endpoint 和令牌。卸载程序但保留数据和配置时执行
`sudo ./uninstall.sh`。

发布压缩包应使用其中的 Caddy。在解压目录根部执行：

```bash
./bin/davd \
  --caddy-binary ./libexec/caddy \
  --data-dir ./data \
  --config-dir ./config \
  --runtime-dir ./run
```

Management API 只绑定本机回环地址。默认 `--listen` 为 `127.0.0.1:0`，由操作系统
分配空闲端口。守护进程把 endpoint 写入 `run/management.endpoint`，把管理令牌写入
`config/management.token`。

正常安装的桌面应用或系统服务应省略 portable 目录参数，使用平台默认应用目录。不要
把管理令牌复制到 shell 脚本、工单或共享文件系统中。

前台运行时按 `Ctrl-C` 停止。守护进程会关闭它管理的 Caddy，并删除临时 endpoint 文件。

## 4. 使用 GUI

GUI 是管理客户端，不需要直接访问数据库。

首次运行通常按以下顺序：

1. 为当前用户启动或安装守护进程。
2. 打开 DavDeck，等待守护进程连接变为健康。
3. 创建用户并设置密码。
4. 添加一个绝对路径的共享目录。
5. 为用户设置 `READ` 或 `READ_WRITE` 权限。
6. 选择 HTTPS 模式并确认监听端口。
7. 应用配置并确认 Dashboard 状态。
8. 使用显示的 WebDAV 地址和凭据连接客户端。

当一个用户被授权多个共享目录时，推荐使用 Dashboard 显示的统一入口（默认
为 `/dav/`）。WebDAV 客户端会通过目录发现列出该用户有权限访问的共享目录；每个
共享目录仍然可以使用对应的 `/dav/<slug>/` 地址直接连接。统一入口本身只读，
不支持在共享目录之间移动或复制文件。

Dashboard、用户、共享、TLS、日志、诊断、服务和修订页面都使用守护进程维护的
同一份状态。如果应用失败，先查看结构化错误和运行时状态再重试；在可能的情况下，
程序会保留最后一次正常运行的配置。

“设置”页面提供升级和卸载的数据安全提示，以及“导出配置备份”和“导入配置备份”
操作。导出使用系统文件保存对话框；导入前会要求确认，并以事务方式合并期望状态。
导入不会删除共享目录或其中的物理文件。备份不包含密码、Management API 令牌和 TLS
私钥；导入新用户后，需要到“用户”页面重新设置密码，并到仪表盘应用待处理配置。

## 5. 使用 `davctl`

`davctl` 从平台默认目录发现 endpoint 和令牌，也支持显式传入：

```bash
./bin/davctl --endpoint http://127.0.0.1:12345 \
  --token-file ./config/management.token status
```

机器处理时，在命令前加入 `--json`。`version` 是本地命令，不需要运行中的守护进程。

### 查看状态和诊断

```bash
./bin/davctl version --json
./bin/davctl status
./bin/davctl --json status
./bin/davctl doctor
./bin/davctl --json doctor
./bin/davctl logs --limit 50
./bin/davctl logs --level ERROR --component caddy
```

在真实终端中不带命令运行 `davctl`，会进入交互菜单，可管理状态、用户、共享、
权限、HTTPS/TLS、配置、日志、诊断、备份和服务。全新安装还会提供简短的首次设置
向导。管道、脚本和 CI 场景保持非交互，只输出 usage。

`doctor` 在总体检查失败时返回非零退出码。日志是有界且经过清理的；当前不支持
`davctl logs --follow`。

### 管理服务和端口

```bash
./bin/davctl server status
./bin/davctl server start
./bin/davctl server stop
./bin/davctl server restart
./bin/davctl server settings
./bin/davctl server ports --http 8080 --https 8443
```

这些命令管理守护进程拥有的 Caddy 运行时，不负责安装或直接控制操作系统服务。

### 管理用户

密码优先通过标准输入或交互式无回显输入：

```bash
printf '%s\n' 'use-a-secret-from-a-secure-input' | \
  ./bin/davctl user add alice --password-stdin
./bin/davctl user list
./bin/davctl user passwd alice
./bin/davctl user enable alice
./bin/davctl user disable alice
./bin/davctl user delete alice
```

不要把密码写入命令行参数。禁用或删除用户不会删除共享目录中的文件。

### 管理共享和 ACL

共享路径必须是绝对路径。元数据操作不会删除物理文件。

```bash
./bin/davctl share add Documents /srv/davdeck/documents
./bin/davctl share list
./bin/davctl acl set Documents alice read-write
./bin/davctl acl set Documents bob read
./bin/davctl acl list Documents
./bin/davctl share update Documents --disable
./bin/davctl share remove Documents
```

权限含义：

- `none`：用户不能使用共享；
- `read`：允许列目录和读取，拒绝修改；
- `read-write`：允许读取和预期的写操作。

### 配置 HTTPS

先查看当前配置并验证证书文件：

```bash
./bin/davctl tls show
./bin/davctl tls check
```

局域网环境可以使用内部 HTTPS，但每个客户端都需要信任生成的本地 CA：

```bash
./bin/davctl tls internal dav.local
```

如需恢复为仅 HTTP：

```bash
./bin/davctl tls disable
./bin/davctl config apply
```

Dashboard 中的 HTTPS 地址只有在配置已应用且本机端点探测成功后才可复制。

使用组织或证书机构提供的证书和私钥：

```bash
./bin/davctl tls custom \
  --hostname dav.example.test \
  --cert /etc/davdeck/tls/fullchain.pem \
  --key /etc/davdeck/tls/privatekey.pem
```

自动/公共 HTTPS 交给 Caddy：

```bash
./bin/davctl tls automatic dav.example.com
```

公共 ACME 申请需要可从公网访问的挑战路径，或者受支持的 DNS challenge provider。
DavDeck 当前没有集成 DNS provider 凭据。局域网部署应使用内部/自定义证书，或者让
外部反向代理、端口映射环境负责公共证书申请。

### 应用和恢复配置

```bash
./bin/davctl config status
./bin/davctl config validate
./bin/davctl config apply
./bin/davctl revision list
./bin/davctl revision restore REVISION_ID
```

用户、共享、ACL 和 TLS 流程通常会自动触发相应的 apply；导入或运维变更后可以使用
显式命令。

从本版本开始，每个新应用的版本都会同时保存生成的 Caddy 配置和完整的应用状态。
恢复完整版本时，也会恢复用户（包括启用/禁用状态）、共享、权限、服务器设置和 TLS
意图。旧版 DavDeck 创建的版本可能只有运行时配置，无法安全恢复；迁移这类配置时，
请使用安全 YAML 导出/导入。

导出安全备份时不会覆盖已有文件：

```bash
./bin/davctl config export --output davdeck-backup.yaml
```

导入是事务性的，只修改期望状态：

```bash
./bin/davctl config import davdeck-backup.yaml
./bin/davctl config validate
./bin/davctl config apply
```

导出不包含明文密码、Management API 令牌、TLS 私钥或 DNS 凭据。导入新用户后，可能
还需要使用 `davctl user passwd` 设置密码。

### 管理系统服务

系统服务功能当前只面向 Linux 无头部署，由 `davd` 转发给 systemd 适配器：

```bash
./bin/davctl service status
./bin/davctl service install
./bin/davctl service start
./bin/davctl service stop
./bin/davctl service uninstall
```

可能需要管理员权限。不要让 GUI 长期以 root 或 Administrator 运行。Windows 和 macOS
桌面版当前不提供系统服务安装入口。

## 6. 平台说明

### macOS ARM64

原生 GUI 是当前主要桌面验证目标。当前应用未签名，可能需要在“隐私与安全性”中
手动允许。点击窗口关闭按钮不会退出 DavDeck，应用会保留在状态栏；从状态栏菜单选择
“退出 DavDeck”才会真正停止 GUI 及其便携守护进程。服务器只在本机使用时，建议使用
自定义证书或内部 HTTPS。窗口隐藏后 Dock 图标也会隐藏，状态栏菜单中的“显示 DavDeck”
会恢复窗口。

### Windows x64

守护进程、CLI 和 GUI 都是发布目标。当前目标已完成 Windows 原生 GUI 和 ACL 验证，
但 Windows reparse-point/junction 隔离仍是单独的安全发布门槛。在重要数据上使用前，
应在目标 Windows 版本上测试实际共享路径。点击窗口关闭按钮会最小化到通知区域托盘；
从托盘菜单选择“退出 DavDeck”才会真正停止 GUI 及其便携守护进程。

### Linux x64 和 ARM64

Linux x64 Server 压缩包可通过 SSH 使用，不需要 Flutter 或桌面会话；x64 Desktop 压缩包
提供原生 GUI。Linux ARM64 目前仅提供 Server 版本。请将数据、配置和运行时目录放在合适
的本地存储中，并在确认权限要求后再通过 `davctl service` 使用 systemd 服务适配器。

## 7. 数据、备份和升级

SQLite 数据库是应用状态的权威来源。升级前：

1. 正常停止守护进程。
2. 使用 GUI“设置”页面导出一份安全 YAML 备份；重要迁移仍建议同时备份 data 目录、config 目录及自定义 TLS 材料。
3. 保留发布压缩包及其校验文件。
4. 启动新版本，运行 `davctl doctor` 和 `davctl config status`。
5. 使用非生产 WebDAV 客户端确认读写操作。

删除用户、共享、服务注册或应用元数据不会删除用户文件。物理数据删除必须由管理员
单独、明确地执行。正常升级或卸载程序默认保留 DavDeck 数据和配置；只有明确选择
删除应用数据时才允许移除它们。当前仓库尚未提供原生图形化卸载器，未来安装器必须
遵循这一保留规则。

## 8. 故障排查

### `DAEMON_DISCOVERY_FAILED`

确认 `davd` 正在运行，`davctl` 使用了相同的平台配置，并且 endpoint 文件和令牌文件
属于同一个实例。portable 实例请显式传入 `--endpoint` 和 `--token-file`。

### `PRIVILEGE_REQUIRED`

请求的服务或受保护文件系统操作需要管理员权限。请通过平台正常的提权流程执行这一
个操作，不要让整个 GUI 以管理员/root 运行。

### 配置应用失败

执行：

```bash
./bin/davctl config validate
./bin/davctl doctor
./bin/davctl logs --level ERROR
```

确认共享路径是绝对路径且可读、监听端口未被占用、自定义证书文件可读取。不要手工编辑
生成的 Caddy JSON，应修正 DavDeck 状态后重新应用。

### WebDAV 客户端拒绝 HTTPS

检查主机名、证书链、客户端信任库和 HTTPS 端口。内部证书需要在每个客户端安装信任。
自定义证书需要确认主机名匹配，且私钥和证书相互匹配。

## 9. 安全报告

不要在公开 issue 中发布尚未修复的漏洞细节。当前报告状态见
[`SECURITY.md`](../SECURITY.md)。公开仓库投入常规安全披露前，必须先配置私密漏洞
报告渠道。
