# DavDeck

DavDeck 是一个基于 Caddy 的开源跨平台 WebDAV 服务管理器。它提供
macOS 和 Windows 原生桌面应用，也提供适用于 Linux 服务器的无头守护进程
和 CLI。

当前仓库正在准备 `0.1.0-rc.1` 预览版。这不是稳定的 `1.0` 版本，预览版二进制
文件暂未签名，使用前请阅读平台限制和发布说明。

## AI 辅助开发声明

DavDeck 的开发过程中大量使用了 AI 编程工具。AI 参与了设计探索、实现、重构、
文档编写和测试开发。代码审查、安全决策、依赖与许可证审查、测试以及发布决策仍
由人工维护者负责。

## 主要功能

- 管理 WebDAV 用户、共享目录，以及按共享目录设置 `NONE`、`READ`、
  `READ_WRITE` 权限。
- 根据应用状态生成并验证 Caddy 运行时配置。
- GUI 和 CLI 共用本机回环地址上的认证 Management API。
- 配置自动 HTTPS、内部 HTTPS 或自定义证书 HTTPS。
- 管理守护进程、配置修订、诊断、日志和原生系统服务适配器。
- 导入和导出安全的版本化 YAML 配置，不导出密码、管理令牌或私钥。
- Linux 无需 Flutter 或桌面会话即可无头运行。

DavDeck 不是 Caddyfile 编辑器。用户管理的是应用状态，DavDeck 负责为其编译
和运行 Caddy 配置。

## 预览版目标平台

| 目标平台 | 当前预览状态 |
| --- | --- |
| macOS ARM64 | 已完成原生 GUI 冒烟验证；二进制未签名 |
| Windows x64 | 已提供构建目标；GUI 验证暂缓 |
| Linux x64 | 已在原生硬件完成无头守护进程/CLI 和 HTTPS 冒烟验证 |
| Linux ARM64 | 已在原生硬件完成无头守护进程/CLI 和 HTTPS 冒烟验证 |

Windows GUI 验证、安装器完善、代码签名、公证，以及 Linux 服务重启后的自动启动
验证均暂不纳入本预览版。详见[已知限制](docs/KNOWN_LIMITATIONS.md)。

## 从源码快速开始

核心程序需要 Go，桌面客户端还需要 Flutter；具体版本要求见项目文档。

```bash
make core-build caddy-build
./core/davd \
  --caddy-binary ./core/bin/caddy \
  --data-dir ./data --config-dir ./config --runtime-dir ./run
```

在另一个终端通过守护进程的本机 Management API 使用 CLI：

```bash
./core/davctl version --json
./core/davctl status
./core/davctl doctor
```

使用发布包时，请使用压缩包中的 `bin/davd` 和 `bin/davctl`。完整流程包括 GUI
初始化、系统服务、HTTPS、备份和 CLI 自动化，见[用户手册](docs/USER_GUIDE.zh-CN.md)，
也可阅读[英文版](docs/USER_GUIDE.md)。

## 文档

- [用户手册](docs/USER_GUIDE.zh-CN.md) · [English User Guide](docs/USER_GUIDE.md)
- [已知限制](docs/KNOWN_LIMITATIONS.md)
- [项目规格](docs/PROJECT_SPEC.md)
- [架构](docs/ARCHITECTURE.md)
- [CLI 参考](docs/CLI.md)
- [平台说明](docs/PLATFORM.md)
- [安全设计](docs/SECURITY.md) · [安全策略](SECURITY.md)
- [发布流程](docs/RELEASE.md)
- [变更记录](CHANGELOG.md)

内部开发流程说明和本地验证记录不属于公开用户文档。

## 开发

```bash
make check
make caddy-tooling-test
make caddy-module-test
make release-packaging-test
```

发布前应执行适用的原生平台检查。前置条件和已知限制见测试及发布文档。

## 许可证

DavDeck 使用 [Apache License 2.0](LICENSE) 发布。第三方组件仍受其各自许可证和
署名要求约束，详见 [NOTICE](NOTICE) 以及相关上游项目元数据。
