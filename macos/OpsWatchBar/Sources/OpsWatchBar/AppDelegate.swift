import AppKit
import CoreGraphics
import Foundation

struct WatchWindow {
    let id: UInt32
    let owner: String
    let title: String

    var label: String {
        if title.isEmpty {
            return "\(owner) (#\(id))"
        }
        return "\(owner): \(title)"
    }
}

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    private struct LaunchCommand {
        let executableURL: URL
        let currentDirectoryURL: URL?
        let argumentsPrefix: [String]
        let usesRepoCheckout: Bool
    }

    private let statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
    private let menu = NSMenu()
    private let windowsMenu = NSMenu()
    private var selectedWindow: WatchWindow?
    private var watcher: Process?
    private var verifier: Process?
    private var selectedItem = NSMenuItem(title: "Selected: none", action: nil, keyEquivalent: "")
    private var startItem = NSMenuItem(title: "Start Watching", action: #selector(startWatching), keyEquivalent: "s")
    private var stopItem = NSMenuItem(title: "Stop Watching", action: #selector(stopWatching), keyEquivalent: "x")
    private var verifyItem = NSMenuItem(title: "Verify Current", action: #selector(verifyCurrent), keyEquivalent: "v")
    private var settingsItem = NSMenuItem(title: "Settings...", action: #selector(openSettings), keyEquivalent: ",")
    private var checkSetupItem = NSMenuItem(title: "Check Setup", action: #selector(checkSetup), keyEquivalent: "d")
    private var logItem = NSMenuItem(title: "Open Log", action: #selector(openLog), keyEquivalent: "l")
    private var statusItemRow = NSMenuItem(title: "Status: idle", action: nil, keyEquivalent: "")
    private let logURL = URL(fileURLWithPath: NSTemporaryDirectory()).appendingPathComponent("opswatch-menubar.log")
    private var logHandle: FileHandle?
    private var settings = AppSettings.load()
    private var settingsWindowController: SettingsWindowController?

    func applicationDidFinishLaunching(_ notification: Notification) {
        NSApp.setActivationPolicy(.accessory)
        setStatus(.idle)
        configureMenu()
        syncMenuState()
        refreshWindows()
    }

    private func configureMenu() {
        statusItemRow.isEnabled = false
        menu.addItem(statusItemRow)

        selectedItem.isEnabled = false
        menu.addItem(selectedItem)

        let chooseItem = NSMenuItem(title: "Windows", action: nil, keyEquivalent: "")
        chooseItem.submenu = windowsMenu
        menu.addItem(chooseItem)

        menu.addItem(NSMenuItem(title: "Refresh Windows", action: #selector(refreshWindows), keyEquivalent: "r"))
        menu.addItem(.separator())

        startItem.target = self
        stopItem.target = self
        verifyItem.target = self
        stopItem.isEnabled = false
        verifyItem.isEnabled = false
        settingsItem.target = self
        checkSetupItem.target = self
        logItem.target = self
        menu.addItem(startItem)
        menu.addItem(stopItem)
        menu.addItem(verifyItem)
        menu.addItem(settingsItem)
        menu.addItem(checkSetupItem)
        menu.addItem(logItem)

        menu.addItem(.separator())
        let quitItem = NSMenuItem(title: "Quit", action: #selector(quit), keyEquivalent: "q")
        quitItem.target = self
        menu.addItem(quitItem)

        statusItem.menu = menu
    }

    @objc private func refreshWindows() {
        windowsMenu.removeAllItems()
        let windows = listWindows()
        if windows.isEmpty {
            let item = NSMenuItem(title: "No capturable windows found", action: nil, keyEquivalent: "")
            item.isEnabled = false
            windowsMenu.addItem(item)
            return
        }

        for window in windows.prefix(40) {
            let item = NSMenuItem(title: window.label, action: #selector(selectWindow(_:)), keyEquivalent: "")
            item.target = self
            item.representedObject = window
            windowsMenu.addItem(item)
        }
    }

    @objc private func selectWindow(_ sender: NSMenuItem) {
        guard let window = sender.representedObject as? WatchWindow else {
            return
        }
        selectedWindow = window
        selectedItem.title = "Selected: \(window.label)"
        if watcher == nil {
            setStatus(.selected)
        }
        syncMenuState()
    }

    @objc private func startWatching() {
        guard watcher == nil else {
            return
        }
        guard let selectedWindow else {
            selectedItem.title = "Select a window first"
            setStatus(.needsWindow)
            return
        }
        setStatus(.starting)

        let command = opswatchCommand()
        var arguments = command.argumentsPrefix + [
            "watch",
            "--vision-provider", settings.visionProvider,
            "--model", settings.model,
            "--interval", settings.interval,
            "--window-id", "\(selectedWindow.id)",
            "--window-owner", selectedWindow.owner,
            "--window-title", selectedWindow.title,
            "--max-image-dimension", settings.maxImageDimension,
            "--ollama-num-predict", settings.ollamaNumPredict,
            "--alert-cooldown", settings.alertCooldown,
            "--min-analysis-interval", settings.minAnalysisInterval,
            "--environment", settings.environment,
            "--context-dir", settings.contextDir,
            "--notify",
            "--verbose"
        ]
        appendOptionalFlag("--intent", settings.intent, to: &arguments)
        appendOptionalFlag("--expected-action", settings.expectedAction, to: &arguments)
        appendOptionalFlag("--protected-domain", settings.protectedDomain, to: &arguments)
        do {
            let process = try makeLoggedProcess(command: command, arguments: arguments)
            NotificationCenter.default.addObserver(
                self,
                selector: #selector(watcherDidTerminate(_:)),
                name: Process.didTerminateNotification,
                object: process
            )
            try process.run()
            watcher = process
            setStatus(.watching)
            syncMenuState()
            NSWorkspace.shared.open(logURL)
        } catch {
            selectedItem.title = "Start failed: \(error.localizedDescription)"
            setStatus(.error)
            closeLogHandle()
        }
    }

    @objc private func stopWatching() {
        let runningWatcher = watcher
        runningWatcher?.terminate()
        if let runningWatcher {
            NotificationCenter.default.removeObserver(
                self,
                name: Process.didTerminateNotification,
                object: runningWatcher
            )
        }
        watcher = nil
        setStatus(selectedWindow == nil ? .idle : .selected)
        syncMenuState()
        closeLogHandle()
    }

    @objc private func watcherDidTerminate(_ notification: Notification) {
        guard let terminatedProcess = notification.object as? Process,
              watcher === terminatedProcess else {
            return
        }
        NotificationCenter.default.removeObserver(
            self,
            name: Process.didTerminateNotification,
            object: terminatedProcess
        )
        watcher = nil
        setStatus(terminatedProcess.terminationStatus == 0 ? .selected : .stoppedUnexpectedly)
        syncMenuState()
        closeLogHandle()
    }

    @objc private func verifyCurrent() {
        guard watcher == nil else {
            selectedItem.title = "Stop watching before verification"
            setStatus(.watching)
            return
        }
        guard verifier == nil else {
            return
        }
        guard let selectedWindow else {
            selectedItem.title = "Select a window first"
            setStatus(.needsWindow)
            return
        }
        setStatus(.starting)

        let command = opswatchCommand()
        var arguments = command.argumentsPrefix + [
            "watch",
            "--vision-provider", settings.visionProvider,
            "--model", settings.model,
            "--window-id", "\(selectedWindow.id)",
            "--window-owner", selectedWindow.owner,
            "--window-title", selectedWindow.title,
            "--max-image-dimension", settings.maxImageDimension,
            "--ollama-num-predict", settings.ollamaNumPredict,
            "--environment", settings.environment,
            "--context-dir", settings.contextDir,
            "--notify",
            "--verbose",
            "--once"
        ]
        appendOptionalFlag("--intent", settings.intent, to: &arguments)
        appendOptionalFlag("--expected-action", settings.expectedAction, to: &arguments)
        appendOptionalFlag("--protected-domain", settings.protectedDomain, to: &arguments)

        do {
            let process = try makeLoggedProcess(command: command, arguments: arguments)
            process.terminationHandler = { [weak self] _ in
                Task { @MainActor in
                    self?.verifier = nil
                    if self?.watcher == nil {
                        self?.setStatus(self?.selectedWindow == nil ? .idle : .selected)
                    }
                    self?.syncMenuState()
                    self?.closeLogHandle()
                }
            }
            try process.run()
            verifier = process
            selectedItem.title = "Selected: \(selectedWindow.label) (verifying)"
            syncMenuState()
            NSWorkspace.shared.open(logURL)
        } catch {
            selectedItem.title = "Verify failed: \(error.localizedDescription)"
            setStatus(.error)
            syncMenuState()
            closeLogHandle()
        }
    }

    @objc private func openLog() {
        NSWorkspace.shared.open(logURL)
    }

    @objc private func openSettings() {
        let controller = SettingsWindowController(settings: settings) { [weak self] newSettings in
            self?.settings = newSettings
        }
        settingsWindowController = controller
        controller.showWindow(nil)
        NSApp.activate(ignoringOtherApps: true)
    }

    @objc private func checkSetup() {
        let command = opswatchCommand()
        let process = Process()
        process.executableURL = command.executableURL
        process.currentDirectoryURL = command.currentDirectoryURL
        process.environment = appEnvironment()
        var arguments = command.argumentsPrefix + [
            "doctor",
            "--vision-provider", settings.visionProvider,
            "--model", settings.model
        ]
        if command.usesRepoCheckout {
            arguments += ["--repo-root", settings.root]
        }
        process.arguments = arguments

        FileManager.default.createFile(atPath: logURL.path, contents: nil)
        do {
            let handle = try FileHandle(forWritingTo: logURL)
            try handle.seekToEnd()
            process.standardOutput = handle
            process.standardError = handle
            process.terminationHandler = { _ in
                try? handle.close()
            }
            try process.run()
            NSWorkspace.shared.open(logURL)
        } catch {
            selectedItem.title = "Doctor failed: \(error.localizedDescription)"
            setStatus(.error)
        }
    }

    private func opswatchCommand() -> LaunchCommand {
        if let bundledCLI = Bundle.main.url(forResource: "opswatch", withExtension: nil),
           FileManager.default.isExecutableFile(atPath: bundledCLI.path) {
            return LaunchCommand(
                executableURL: bundledCLI,
                currentDirectoryURL: nil,
                argumentsPrefix: [],
                usesRepoCheckout: false
            )
        }

        return LaunchCommand(
            executableURL: URL(fileURLWithPath: "/usr/bin/env"),
            currentDirectoryURL: URL(fileURLWithPath: settings.root),
            argumentsPrefix: ["go", "run", "./cmd/opswatch"],
            usesRepoCheckout: true
        )
    }

    private func appEnvironment() -> [String: String] {
        var environment = ProcessInfo.processInfo.environment
        let defaultPath = "/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
        if let path = environment["PATH"], !path.isEmpty {
            environment["PATH"] = "\(defaultPath):\(path)"
        } else {
            environment["PATH"] = defaultPath
        }
        if let bundledOCR = Bundle.main.url(forResource: "OpsWatchOCR", withExtension: nil),
           FileManager.default.isExecutableFile(atPath: bundledOCR.path) {
            environment["OPSWATCH_OCR_HELPER"] = bundledOCR.path
        } else {
            let localCandidates = [
                "\(settings.root)/macos/OpsWatchBar/.build/debug/OpsWatchOCR",
                "\(settings.root)/macos/OpsWatchBar/.build/release/OpsWatchOCR",
            ]
            if let helper = localCandidates.first(where: { FileManager.default.isExecutableFile(atPath: $0) }) {
                environment["OPSWATCH_OCR_HELPER"] = helper
            }
        }
        return environment
    }

    @objc private func quit() {
        stopWatching()
        NSApp.terminate(nil)
    }

    private func listWindows() -> [WatchWindow] {
        let options: CGWindowListOption = [.optionOnScreenOnly, .excludeDesktopElements]
        guard let rawWindows = CGWindowListCopyWindowInfo(options, kCGNullWindowID) as? [[String: Any]] else {
            return []
        }

        return rawWindows.compactMap { info in
            guard let id = info[kCGWindowNumber as String] as? UInt32,
                  let owner = info[kCGWindowOwnerName as String] as? String else {
                return nil
            }
            let layer = info[kCGWindowLayer as String] as? Int ?? 0
            let alpha = info[kCGWindowAlpha as String] as? Double ?? 1
            if layer != 0 || alpha <= 0 {
                return nil
            }
            let title = info[kCGWindowName as String] as? String ?? ""
            if owner == "OpsWatchBar" || owner == "Window Server" {
                return nil
            }
            return WatchWindow(id: id, owner: owner, title: title)
        }
    }

    private func appendOptionalFlag(_ flag: String, _ value: String, to arguments: inout [String]) {
        if value.isEmpty {
            return
        }
        arguments.append(flag)
        arguments.append(value)
    }

    private func makeLoggedProcess(command: LaunchCommand, arguments: [String]) throws -> Process {
        FileManager.default.createFile(atPath: logURL.path, contents: nil)
        let handle = try FileHandle(forWritingTo: logURL)
        try handle.seekToEnd()

        let process = Process()
        process.executableURL = command.executableURL
        process.currentDirectoryURL = command.currentDirectoryURL
        process.environment = appEnvironment()
        process.arguments = arguments
        process.standardOutput = handle
        process.standardError = handle
        logHandle = handle
        return process
    }

    private func closeLogHandle() {
        try? logHandle?.close()
        logHandle = nil
    }

    private func syncMenuState() {
        let hasSelection = selectedWindow != nil
        startItem.isEnabled = hasSelection && watcher == nil && verifier == nil
        stopItem.isEnabled = watcher != nil
        verifyItem.isEnabled = hasSelection && watcher == nil && verifier == nil
    }

    private func setStatus(_ status: WatchStatus) {
        if let button = statusItem.button {
            button.image = makeStatusIcon()
            button.imagePosition = .imageLeading
            button.toolTip = "OpsWatch: \(status.description)"
        }
        statusItem.button?.title = status.menuTitle
        statusItemRow.title = "Status: \(status.description)"
    }

    private func makeStatusIcon() -> NSImage {
        let size = NSSize(width: 18, height: 18)
        let image = NSImage(size: size)
        image.lockFocus()

        let shield = NSBezierPath()
        shield.move(to: NSPoint(x: 9, y: 16))
        shield.curve(to: NSPoint(x: 15, y: 13), controlPoint1: NSPoint(x: 11, y: 15.5), controlPoint2: NSPoint(x: 13, y: 14.5))
        shield.curve(to: NSPoint(x: 9, y: 2), controlPoint1: NSPoint(x: 14.7, y: 7.8), controlPoint2: NSPoint(x: 12.4, y: 4.2))
        shield.curve(to: NSPoint(x: 3, y: 13), controlPoint1: NSPoint(x: 5.6, y: 4.2), controlPoint2: NSPoint(x: 3.3, y: 7.8))
        shield.curve(to: NSPoint(x: 9, y: 16), controlPoint1: NSPoint(x: 5, y: 14.5), controlPoint2: NSPoint(x: 7, y: 15.5))
        shield.close()
        shield.lineWidth = 1.6
        NSColor.labelColor.setStroke()
        shield.stroke()

        let eye = NSBezierPath(ovalIn: NSRect(x: 6, y: 7, width: 6, height: 4))
        eye.lineWidth = 1.3
        eye.stroke()

        let pupil = NSBezierPath(ovalIn: NSRect(x: 8.2, y: 8.2, width: 1.6, height: 1.6))
        NSColor.labelColor.setFill()
        pupil.fill()

        image.unlockFocus()
        image.isTemplate = true
        return image
    }

}

private enum WatchStatus {
    case idle
    case selected
    case needsWindow
    case starting
    case watching
    case stoppedUnexpectedly
    case error

    var menuTitle: String {
        switch self {
        case .idle:
            return "OpsWatch"
        case .selected:
            return "OpsWatch ◦"
        case .needsWindow:
            return "OpsWatch !"
        case .starting:
            return "OpsWatch …"
        case .watching:
            return "OpsWatch ●"
        case .stoppedUnexpectedly:
            return "OpsWatch !"
        case .error:
            return "OpsWatch !"
        }
    }

    var description: String {
        switch self {
        case .idle:
            return "idle"
        case .selected:
            return "window selected"
        case .needsWindow:
            return "select a window first"
        case .starting:
            return "starting watcher"
        case .watching:
            return "watching"
        case .stoppedUnexpectedly:
            return "watcher stopped"
        case .error:
            return "error"
        }
    }
}
