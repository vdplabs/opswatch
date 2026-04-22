import AppKit

@MainActor
final class SettingsWindowController: NSWindowController {
    private var fields: [String: NSTextField] = [:]
    private var profileControl: NSPopUpButton?
    private let onSave: (AppSettings) -> Void

    init(settings: AppSettings, onSave: @escaping (AppSettings) -> Void) {
        self.onSave = onSave

        let contentView = NSView(frame: NSRect(x: 0, y: 0, width: 640, height: 540))
        let window = NSWindow(
            contentRect: contentView.frame,
            styleMask: [.titled, .closable],
            backing: .buffered,
            defer: false
        )
        window.title = "OpsWatch Settings"
        window.contentView = contentView
        super.init(window: window)

        buildForm(in: contentView, settings: settings)
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) has not been implemented")
    }

    private func buildForm(in contentView: NSView, settings: AppSettings) {
        let stack = NSStackView()
        stack.orientation = .vertical
        stack.spacing = 10
        stack.translatesAutoresizingMaskIntoConstraints = false
        contentView.addSubview(stack)

        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: contentView.leadingAnchor, constant: 20),
            stack.trailingAnchor.constraint(equalTo: contentView.trailingAnchor, constant: -20),
            stack.topAnchor.constraint(equalTo: contentView.topAnchor, constant: 20)
        ])

        addProfilePicker(settings: settings, stack: stack)
        addField("Repo root", key: "root", value: settings.root, stack: stack)
        addField("Vision provider", key: "visionProvider", value: settings.visionProvider, stack: stack)
        addField("Model", key: "model", value: settings.model, stack: stack)
        addField("Interval", key: "interval", value: settings.interval, stack: stack)
        addField("Max image dimension", key: "maxImageDimension", value: settings.maxImageDimension, stack: stack)
        addField("Ollama num predict", key: "ollamaNumPredict", value: settings.ollamaNumPredict, stack: stack)
        addField("Min analysis interval", key: "minAnalysisInterval", value: settings.minAnalysisInterval, stack: stack)
        addField("Alert cooldown", key: "alertCooldown", value: settings.alertCooldown, stack: stack)
        addField("Environment", key: "environment", value: settings.environment, stack: stack)
        addField("Context directory", key: "contextDir", value: settings.contextDir, stack: stack)
        addField("Intent (optional)", key: "intent", value: settings.intent, stack: stack)
        addField("Expected action (optional)", key: "expectedAction", value: settings.expectedAction, stack: stack)
        addField("Protected domain (optional)", key: "protectedDomain", value: settings.protectedDomain, stack: stack)

        let buttonRow = NSStackView()
        buttonRow.orientation = .horizontal
        buttonRow.alignment = .centerY
        buttonRow.spacing = 10
        let saveButton = NSButton(title: "Save", target: self, action: #selector(save))
        let closeButton = NSButton(title: "Close", target: self, action: #selector(closeWindow))
        buttonRow.addArrangedSubview(saveButton)
        buttonRow.addArrangedSubview(closeButton)
        stack.addArrangedSubview(buttonRow)
    }

    private func addProfilePicker(settings: AppSettings, stack: NSStackView) {
        let row = NSStackView()
        row.orientation = .horizontal
        row.alignment = .centerY
        row.spacing = 12

        let labelView = NSTextField(labelWithString: "Model profile")
        labelView.widthAnchor.constraint(equalToConstant: 170).isActive = true

        let popup = NSPopUpButton()
        popup.translatesAutoresizingMaskIntoConstraints = false
        popup.widthAnchor.constraint(equalToConstant: 220).isActive = true
        AppProfile.allCases.forEach { popup.addItem(withTitle: $0.label) }
        if let profile = AppProfile(rawValue: settings.profile.lowercased()) {
            popup.selectItem(withTitle: profile.label)
        } else {
            popup.selectItem(withTitle: AppProfile.balanced.label)
        }
        popup.target = self
        popup.action = #selector(profileChanged(_:))
        profileControl = popup

        let hint = NSTextField(labelWithString: "Fast favors responsiveness. Accurate favors richer vision output.")
        hint.textColor = .secondaryLabelColor
        hint.lineBreakMode = .byWordWrapping

        row.addArrangedSubview(labelView)
        row.addArrangedSubview(popup)
        row.addArrangedSubview(hint)
        stack.addArrangedSubview(row)
    }

    private func addField(_ label: String, key: String, value: String, stack: NSStackView) {
        let row = NSStackView()
        row.orientation = .horizontal
        row.alignment = .centerY
        row.spacing = 12

        let labelView = NSTextField(labelWithString: label)
        labelView.widthAnchor.constraint(equalToConstant: 170).isActive = true

        let field = NSTextField(string: value)
        field.translatesAutoresizingMaskIntoConstraints = false
        field.widthAnchor.constraint(equalToConstant: 390).isActive = true
        fields[key] = field

        row.addArrangedSubview(labelView)
        row.addArrangedSubview(field)
        stack.addArrangedSubview(row)
    }

    @objc private func profileChanged(_ sender: NSPopUpButton) {
        guard let title = sender.selectedItem?.title,
              let profile = AppProfile.allCases.first(where: { $0.label == title }) else {
            return
        }
        apply(settings: currentSettings().applying(profile: profile))
    }

    @objc private func save() {
        let settings = currentSettings()
        settings.save()
        onSave(settings)
        window?.close()
    }

    @objc private func closeWindow() {
        window?.close()
    }

    private func field(_ key: String) -> String {
        fields[key]?.stringValue.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
    }

    private func currentSettings() -> AppSettings {
        AppSettings(
            profile: selectedProfile().rawValue,
            root: field("root"),
            visionProvider: field("visionProvider"),
            model: field("model"),
            interval: field("interval"),
            maxImageDimension: field("maxImageDimension"),
            ollamaNumPredict: field("ollamaNumPredict"),
            minAnalysisInterval: field("minAnalysisInterval"),
            alertCooldown: field("alertCooldown"),
            environment: field("environment"),
            contextDir: field("contextDir"),
            intent: field("intent"),
            expectedAction: field("expectedAction"),
            protectedDomain: field("protectedDomain")
        )
    }

    private func selectedProfile() -> AppProfile {
        guard let title = profileControl?.selectedItem?.title,
              let profile = AppProfile.allCases.first(where: { $0.label == title }) else {
            return .balanced
        }
        return profile
    }

    private func apply(settings: AppSettings) {
        profileControl?.selectItem(withTitle: AppProfile(rawValue: settings.profile)?.label ?? AppProfile.balanced.label)
        fields["visionProvider"]?.stringValue = settings.visionProvider
        fields["model"]?.stringValue = settings.model
        fields["interval"]?.stringValue = settings.interval
        fields["maxImageDimension"]?.stringValue = settings.maxImageDimension
        fields["ollamaNumPredict"]?.stringValue = settings.ollamaNumPredict
        fields["minAnalysisInterval"]?.stringValue = settings.minAnalysisInterval
    }
}
