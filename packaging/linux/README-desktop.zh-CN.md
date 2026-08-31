# DavDeck Linux Desktop

这是 Linux x64 Desktop 版本。请保持压缩包目录结构不变，在根目录运行：

```bash
./davdeck
```

GUI 包含匹配的 `davd` 和 Caddy。若没有可连接的本机 daemon，GUI 会自动启动自己
携带的 portable daemon，并在退出时只关闭这个由自己启动的 daemon。若连接的是
已有的 systemd daemon，则会将其视为外部 daemon，不会在 GUI 退出时停止它。

运行前请核对压缩包校验和，并阅读 `manifest.json`。
