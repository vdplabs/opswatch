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
    private let statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
    private let menu = NSMenu()
    private let windowsMenu = NSMenu()
    private var selectedWindow: WatchWindow?
    private var watcher: Process?
    private var selectedItem = NSMenuItem(title: "Selected: none", action: nil, keyEquivalent: "")
    private var startItem = NSMenuItem(title: "Start Watching", action: #selector(startWatching), keyEquivalent: "s")
    private var stopItem = NSMenuItem(title: "Stop Watching", action: #selector(stopWatching), keyEquivalent: "x")
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
        stopItem.isEnabled = false
        settingsItem.target = self
        checkSetupItem.target = self
        logItem.target = self
        menu.addItem(startItem)
        menu.addItem(stopItem)
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

        let root = URL(fileURLWithPath: settings.root)
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/env")
        process.currentDirectoryURL = root
        var arguments = [
            "go", "run", "./cmd/opswatch", "watch",
            "--vision-provider", settings.visionProvider,
            "--model", settings.model,
            "--interval", settings.interval,
            "--window-id", "\(selectedWindow.id)",
            "--max-image-dimension", settings.maxImageDimension,
            "--ollama-num-predict", settings.ollamaNumPredict,
            "--alert-cooldown", settings.alertCooldown,
            "--min-analysis-interval", settings.minAnalysisInterval,
            "--environment", settings.environment,
            "--notify",
            "--verbose"
        ]
        appendOptionalFlag("--intent", settings.intent, to: &arguments)
        appendOptionalFlag("--expected-action", settings.expectedAction, to: &arguments)
        appendOptionalFlag("--protected-domain", settings.protectedDomain, to: &arguments)
        process.arguments = arguments

        FileManager.default.createFile(atPath: logURL.path, contents: nil)
        do {
            logHandle = try FileHandle(forWritingTo: logURL)
            try logHandle?.seekToEnd()
        } catch {
            selectedItem.title = "Log error: \(error.localizedDescription)"
            return
        }
        process.standardOutput = logHandle
        process.standardError = logHandle
        process.terminationHandler = { [weak self] process in
            Task { @MainActor in
                guard let self else {
                    return
                }
                if self.watcher === process {
                    self.watcher = nil
                    self.startItem.isEnabled = true
                    self.stopItem.isEnabled = false
                    self.setStatus(process.terminationStatus == 0 ? .selected : .stoppedUnexpectedly)
                    try? self.logHandle?.close()
                    self.logHandle = nil
                }
            }
        }

        do {
            try process.run()
            watcher = process
            startItem.isEnabled = false
            stopItem.isEnabled = true
            setStatus(.watching)
            NSWorkspace.shared.open(logURL)
        } catch {
            selectedItem.title = "Start failed: \(error.localizedDescription)"
            setStatus(.error)
            try? logHandle?.close()
            logHandle = nil
        }
    }

    @objc private func stopWatching() {
        watcher?.terminate()
        watcher = nil
        startItem.isEnabled = true
        stopItem.isEnabled = false
        setStatus(selectedWindow == nil ? .idle : .selected)
        try? logHandle?.close()
        logHandle = nil
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
        let root = URL(fileURLWithPath: settings.root)
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/env")
        process.currentDirectoryURL = root
        process.arguments = [
            "go", "run", "./cmd/opswatch", "doctor",
            "--repo-root", settings.root,
            "--vision-provider", settings.visionProvider,
            "--model", settings.model
        ]

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

    private func setStatus(_ status: WatchStatus) {
        statusItem.button?.title = status.menuTitle
        statusItemRow.title = "Status: \(status.description)"
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

let app = NSApplication.shared
let delegate = AppDelegate()
app.delegate = delegate
app.run()
