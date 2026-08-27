import Cocoa
import FlutterMacOS

@main
class AppDelegate: FlutterAppDelegate {
  private var daemonProcess: Process?

  override func applicationDidFinishLaunching(_ notification: Notification) {
    super.applicationDidFinishLaunching(notification)
    startBundledDaemonIfNeeded()
  }

  override func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
    // The Flutter window manager intercepts the close event and the desktop
    // lifecycle keeps DavDeck in the menu bar. Keep this native fallback
    // non-terminating as well.
    return false
  }

  override func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
    // This is a fallback for a Dock click while the window is hidden. The
    // normal path is the status-bar menu's Show DavDeck action.
    sender.setActivationPolicy(.regular)
    if let window = sender.windows.first(where: { $0 is MainFlutterWindow }) {
      window.makeKeyAndOrderFront(nil)
    }
    sender.activate(ignoringOtherApps: true)
    return true
  }

  override func applicationWillTerminate(_ notification: Notification) {
    stopBundledDaemonIfNeeded()
    super.applicationWillTerminate(notification)
  }

  override func applicationSupportsSecureRestorableState(_ app: NSApplication) -> Bool {
    return true
  }

  func startBundledDaemonIfNeeded() {
    guard daemonProcess == nil else { return }
    guard !localDaemonIsReachable() else {
      appendBootstrapLog("existing daemon is reachable")
      return
    }
    guard let resourceURL = Bundle.main.resourceURL else {
      appendBootstrapLog("application resources are unavailable")
      NSLog("DavDeck: application resources are unavailable")
      return
    }

    let binaryDirectory = resourceURL.appendingPathComponent("DavDeck/bin", isDirectory: true)
    let daemonURL = binaryDirectory.appendingPathComponent("davd")
    let caddyURL = binaryDirectory.appendingPathComponent("caddy")
    guard FileManager.default.isExecutableFile(atPath: daemonURL.path),
          FileManager.default.isExecutableFile(atPath: caddyURL.path) else {
      appendBootstrapLog("bundled runtime is missing or not executable")
      NSLog("DavDeck: bundled runtime is missing; rebuild with make macos-app")
      return
    }

    do {
      appendBootstrapLog("starting bundled daemon")
      let process = Process()
      process.executableURL = daemonURL
      process.arguments = ["--caddy-binary", caddyURL.path, "--portable-owner", "gui"]
      process.standardOutput = daemonLogHandle()
      process.standardError = process.standardOutput
      try process.run()
      daemonProcess = process
      appendBootstrapLog("bundled daemon started with pid \(process.processIdentifier)")
    } catch {
      appendBootstrapLog("could not start bundled daemon: \(error.localizedDescription)")
      NSLog("DavDeck: could not start bundled daemon: \(error.localizedDescription)")
    }
  }

  private func stopBundledDaemonIfNeeded() {
    guard let process = daemonProcess, process.isRunning else {
      daemonProcess = nil
      return
    }

    _ = requestBundledDaemonShutdown()
    for _ in 0..<50 {
      if !process.isRunning {
        daemonProcess = nil
        return
      }
      RunLoop.main.run(until: Date(timeIntervalSinceNow: 0.1))
    }
    if process.isRunning {
      process.terminate()
    }
    daemonProcess = nil
  }

  private func localDaemonIsReachable() -> Bool {
    guard let connection = localDaemonConnection() else {
      return false
    }

    var request = URLRequest(url: connection.endpoint.appendingPathComponent("api/v1/status"))
    request.setValue("Bearer \(connection.token)", forHTTPHeaderField: "Authorization")
    let semaphore = DispatchSemaphore(value: 0)
    var reachable = false
    URLSession.shared.dataTask(with: request) { _, response, _ in
      reachable = (response as? HTTPURLResponse)?.statusCode == 200
      semaphore.signal()
    }.resume()
    _ = semaphore.wait(timeout: .now() + 1)
    return reachable
  }

  private func requestBundledDaemonShutdown() -> Bool {
    guard let connection = localDaemonConnection() else {
      return false
    }

    var request = URLRequest(url: connection.endpoint.appendingPathComponent("api/v1/daemon/shutdown"))
    request.httpMethod = "POST"
    request.setValue("Bearer \(connection.token)", forHTTPHeaderField: "Authorization")
    let semaphore = DispatchSemaphore(value: 0)
    var succeeded = false
    URLSession.shared.dataTask(with: request) { _, response, _ in
      succeeded = (response as? HTTPURLResponse)?.statusCode == 200
      semaphore.signal()
    }.resume()
    _ = semaphore.wait(timeout: .now() + 2)
    return succeeded
  }

  private func localDaemonConnection() -> (endpoint: URL, token: String)? {
    let home = FileManager.default.homeDirectoryForCurrentUser.path
    let endpointPath = "\(home)/Library/Caches/DavDeck/run/management.endpoint"
    let tokenPath = "\(home)/Library/Application Support/DavDeck/management.token"
    guard let endpointText = try? String(contentsOfFile: endpointPath, encoding: .utf8),
      let tokenText = try? String(contentsOfFile: tokenPath, encoding: .utf8),
      let endpoint = URL(string: endpointText.trimmingCharacters(in: .whitespacesAndNewlines)) else {
      return nil
    }
    let token = tokenText.trimmingCharacters(in: .whitespacesAndNewlines)
    return token.isEmpty ? nil : (endpoint, token)
  }

  private func daemonLogHandle() -> FileHandle? {
    let logs = FileManager.default.homeDirectoryForCurrentUser
      .appendingPathComponent("Library/Logs/DavDeck", isDirectory: true)
    do {
      try FileManager.default.createDirectory(at: logs, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
      let log = logs.appendingPathComponent("davd.log")
      if !FileManager.default.fileExists(atPath: log.path) {
        FileManager.default.createFile(atPath: log.path, contents: nil, attributes: [.posixPermissions: 0o600])
      }
      try FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: log.path)
      let handle = try FileHandle(forWritingTo: log)
      handle.seekToEndOfFile()
      return handle
    } catch {
      NSLog("DavDeck: could not open daemon log: \(error.localizedDescription)")
      return nil
    }
  }

  private func appendBootstrapLog(_ message: String) {
    let log = FileManager.default.homeDirectoryForCurrentUser
      .appendingPathComponent("Library/Logs/DavDeck/bootstrap.log")
    let line = "\(ISO8601DateFormatter().string(from: Date())) \(message)\n"
    do {
      try FileManager.default.createDirectory(at: log.deletingLastPathComponent(), withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
      if !FileManager.default.fileExists(atPath: log.path) {
        FileManager.default.createFile(atPath: log.path, contents: nil, attributes: [.posixPermissions: 0o600])
      }
      let handle = try FileHandle(forWritingTo: log)
      handle.seekToEndOfFile()
      handle.write(line.data(using: .utf8)!)
      try handle.close()
    } catch {
      NSLog("DavDeck: could not write bootstrap log: \(error.localizedDescription)")
    }
  }
}
