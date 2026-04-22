import Foundation

enum AppProfile: String, CaseIterable {
    case fast
    case balanced
    case accurate

    var label: String {
        rawValue.capitalized
    }
}

struct AppSettings {
    var profile: String
    var root: String
    var visionProvider: String
    var model: String
    var interval: String
    var maxImageDimension: String
    var ollamaNumPredict: String
    var minAnalysisInterval: String
    var alertCooldown: String
    var environment: String
    var contextDir: String
    var intent: String
    var expectedAction: String
    var protectedDomain: String

    static let defaults = AppSettings(
        profile: AppProfile.balanced.rawValue,
        root: "/Users/vishal/go/src/github.com/vdplabs/opswatch",
        visionProvider: "ollama",
        model: "qwen2.5vl:3b-q4_K_M",
        interval: "2s",
        maxImageDimension: "1000",
        ollamaNumPredict: "128",
        minAnalysisInterval: "5s",
        alertCooldown: "2m",
        environment: "prod",
        contextDir: "\(NSHomeDirectory())/.opswatch/context",
        intent: "",
        expectedAction: "",
        protectedDomain: ""
    )

    static func load() -> AppSettings {
        let defaultsStore = UserDefaults.standard
        let fallback = AppSettings.defaults
        var settings = AppSettings(
            profile: value("profile", env: "OPSWATCH_PROFILE", fallback: fallback.profile, defaultsStore: defaultsStore),
            root: value("root", env: "OPSWATCH_ROOT", fallback: fallback.root, defaultsStore: defaultsStore),
            visionProvider: value("visionProvider", env: "OPSWATCH_VISION_PROVIDER", fallback: fallback.visionProvider, defaultsStore: defaultsStore),
            model: value("model", env: "OPSWATCH_MODEL", fallback: fallback.model, defaultsStore: defaultsStore),
            interval: value("interval", env: "OPSWATCH_INTERVAL", fallback: fallback.interval, defaultsStore: defaultsStore),
            maxImageDimension: value("maxImageDimension", env: "OPSWATCH_MAX_IMAGE_DIMENSION", fallback: fallback.maxImageDimension, defaultsStore: defaultsStore),
            ollamaNumPredict: value("ollamaNumPredict", env: "OPSWATCH_OLLAMA_NUM_PREDICT", fallback: fallback.ollamaNumPredict, defaultsStore: defaultsStore),
            minAnalysisInterval: value("minAnalysisInterval", env: "OPSWATCH_MIN_ANALYSIS_INTERVAL", fallback: fallback.minAnalysisInterval, defaultsStore: defaultsStore),
            alertCooldown: value("alertCooldown", env: "OPSWATCH_ALERT_COOLDOWN", fallback: fallback.alertCooldown, defaultsStore: defaultsStore),
            environment: value("environment", env: "OPSWATCH_ENVIRONMENT", fallback: fallback.environment, defaultsStore: defaultsStore),
            contextDir: value("contextDir", env: "OPSWATCH_CONTEXT_DIR", fallback: fallback.contextDir, defaultsStore: defaultsStore),
            intent: value("intent", env: "OPSWATCH_INTENT", fallback: fallback.intent, defaultsStore: defaultsStore),
            expectedAction: value("expectedAction", env: "OPSWATCH_EXPECTED_ACTION", fallback: fallback.expectedAction, defaultsStore: defaultsStore),
            protectedDomain: value("protectedDomain", env: "OPSWATCH_PROTECTED_DOMAIN", fallback: fallback.protectedDomain, defaultsStore: defaultsStore)
        )
        settings = settings.normalizedProfile()
        return settings
    }

    func save() {
        let defaultsStore = UserDefaults.standard
        defaultsStore.set(profile, forKey: "profile")
        defaultsStore.set(root, forKey: "root")
        defaultsStore.set(visionProvider, forKey: "visionProvider")
        defaultsStore.set(model, forKey: "model")
        defaultsStore.set(interval, forKey: "interval")
        defaultsStore.set(maxImageDimension, forKey: "maxImageDimension")
        defaultsStore.set(ollamaNumPredict, forKey: "ollamaNumPredict")
        defaultsStore.set(minAnalysisInterval, forKey: "minAnalysisInterval")
        defaultsStore.set(alertCooldown, forKey: "alertCooldown")
        defaultsStore.set(environment, forKey: "environment")
        defaultsStore.set(contextDir, forKey: "contextDir")
        defaultsStore.set(intent, forKey: "intent")
        defaultsStore.set(expectedAction, forKey: "expectedAction")
        defaultsStore.set(protectedDomain, forKey: "protectedDomain")
    }

    private static func value(_ key: String, env: String, fallback: String, defaultsStore: UserDefaults) -> String {
        if let saved = defaultsStore.string(forKey: key), !saved.isEmpty {
            return saved
        }
        let envValue = ProcessInfo.processInfo.environment[env] ?? ""
        return envValue.isEmpty ? fallback : envValue
    }

    func normalizedProfile() -> AppSettings {
        guard let preset = AppProfile(rawValue: profile.lowercased()) else {
            return applying(profile: .balanced)
        }
        return applying(profile: preset)
    }

    func applying(profile preset: AppProfile) -> AppSettings {
        var updated = self
        updated.profile = preset.rawValue
        updated.visionProvider = "ollama"
        switch preset {
        case .fast:
            updated.model = "granite3.2-vision"
            updated.maxImageDimension = "768"
            updated.ollamaNumPredict = "96"
            updated.interval = "2s"
            updated.minAnalysisInterval = "4s"
        case .balanced:
            updated.model = "qwen2.5vl:3b-q4_K_M"
            updated.maxImageDimension = "1000"
            updated.ollamaNumPredict = "128"
            updated.interval = "2s"
            updated.minAnalysisInterval = "5s"
        case .accurate:
            updated.model = "llama3.2-vision"
            updated.maxImageDimension = "1200"
            updated.ollamaNumPredict = "192"
            updated.interval = "4s"
            updated.minAnalysisInterval = "10s"
        }
        return updated
    }
}
