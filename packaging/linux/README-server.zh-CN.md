# DavDeck Linux Server

这是无头 Linux Server 版本，包含 `davd`、`davctl` 和固定版本的 Caddy
运行时；不需要另外安装 Go、Flutter 或 Caddy。

在解压目录执行：

```bash
sudo ./install.sh
davctl
```

安装脚本会检查主机架构，把程序安装到 `/opt/davdeck`，创建并启动 systemd
服务，然后执行本机 Management API 冒烟检查。服务使用 `/var/lib/davdeck`
保存数据、`/etc/davdeck` 保存配置、`/run/davdeck` 保存临时 endpoint；管理接口
始终只监听本机回环地址。

卸载程序但保留数据和配置：

```bash
sudo ./uninstall.sh
```

安装前请核对压缩包校验和，并阅读 `manifest.json`。
