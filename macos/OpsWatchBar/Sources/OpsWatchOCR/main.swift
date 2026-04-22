import AppKit
import Foundation
import Vision

struct Args {
    var image = ""
    var environment = ""
    var windowOwner = ""
    var windowTitle = ""
}

struct OCRResult: Encodable {
    var source: String
    var text: String
    var context: [String: String]
}

@main
struct OpsWatchOCR {
    static func main() {
        do {
            let args = try parseArgs(CommandLine.arguments)
            guard !args.image.isEmpty else {
                throw NSError(domain: "OpsWatchOCR", code: 1, userInfo: [NSLocalizedDescriptionKey: "--image is required"])
            }
            let lines = try recognizeText(at: args.image)
            let result = buildResult(lines: lines, args: args)
            let data = try JSONEncoder().encode(result)
            FileHandle.standardOutput.write(data)
        } catch {
            fputs("opswatch-ocr: \(error.localizedDescription)\n", stderr)
            exit(1)
        }
    }
}

private func parseArgs(_ argv: [String]) throws -> Args {
    var parsed = Args()
    var i = 1
    while i < argv.count {
        let arg = argv[i]
        guard i + 1 < argv.count else { break }
        let value = argv[i + 1]
        switch arg {
        case "--image":
            parsed.image = value
        case "--environment":
            parsed.environment = value
        case "--window-owner":
            parsed.windowOwner = value
        case "--window-title":
            parsed.windowTitle = value
        default:
            break
        }
        i += 2
    }
    return parsed
}

private func recognizeText(at path: String) throws -> [String] {
    let url = URL(fileURLWithPath: path)
    guard let image = NSImage(contentsOf: url) else {
        throw NSError(domain: "OpsWatchOCR", code: 2, userInfo: [NSLocalizedDescriptionKey: "could not load image"])
    }
    var rect = NSRect(origin: .zero, size: image.size)
    guard let cgImage = image.cgImage(forProposedRect: &rect, context: nil, hints: nil) else {
        throw NSError(domain: "OpsWatchOCR", code: 3, userInfo: [NSLocalizedDescriptionKey: "could not decode cgimage"])
    }

    let request = VNRecognizeTextRequest()
    request.recognitionLevel = .accurate
    request.usesLanguageCorrection = false
    request.minimumTextHeight = 0.008

    let handler = VNImageRequestHandler(cgImage: cgImage, options: [:])
    try handler.perform([request])

    let observations = request.results ?? []
    return observations.compactMap { $0.topCandidates(1).first?.string.trimmingCharacters(in: .whitespacesAndNewlines) }
        .filter { !$0.isEmpty }
}

private func buildResult(lines: [String], args: Args) -> OCRResult {
    let joined = lines.joined(separator: "\n")
    let lower = joined.lowercased()

    if let command = extractCommand(from: lines, joined: joined) {
        return OCRResult(
            source: "terminal",
            text: command,
            context: compact([
                "app": emptyToNil(args.windowOwner),
                "command": command,
                "environment": emptyToNil(args.environment),
                "resource_type": inferTerminalResourceType(command),
                "action": inferAction(command),
                "risk_hint": "high",
            ])
        )
    }

    if isHostedZoneListScreen(lower) {
        return OCRResult(
            source: "screen",
            text: "Route53 hosted zones list",
            context: compact([
                "app": emptyToNil(args.windowOwner),
                "resource_type": "dns",
                "environment": emptyToNil(args.environment),
                "risk_hint": "low",
            ])
        )
    }

    if isHostedZoneCreateScreen(lower) {
        let domain = extractHostedZoneDomain(from: lines) ?? extractDomain(from: joined)
        let text = domain.map { "Create hosted zone for \($0)" } ?? "Create hosted zone"
        return OCRResult(
            source: "screen",
            text: text,
            context: compact([
                "app": emptyToNil(args.windowOwner),
                "action": "create",
                "resource_type": "hosted_zone",
                "domain": domain,
                "environment": emptyToNil(args.environment),
                "risk_hint": "high",
            ])
        )
    }

    if isLikelyRoute53Screen(lower) {
        let domain = extractHostedZoneDomain(from: lines) ?? extractDomain(from: joined)
        return OCRResult(
            source: "screen",
            text: domain.map { "Route53 change for \($0)" } ?? "Route53 change screen",
            context: compact([
                "app": emptyToNil(args.windowOwner),
                "resource_type": "dns",
                "domain": domain,
                "environment": emptyToNil(args.environment),
                "risk_hint": "medium",
            ])
        )
    }

    let summary = lines.prefix(4).joined(separator: " | ")
    return OCRResult(
        source: "screen",
        text: summary.isEmpty ? "Operational screen" : summary,
        context: compact([
            "app": emptyToNil(args.windowOwner),
            "environment": emptyToNil(args.environment),
            "risk_hint": "low",
        ])
    )
}

private func extractCommand(from lines: [String]) -> String? {
    return extractCommand(from: lines, joined: lines.joined(separator: "\n"))
}

private func extractCommand(from lines: [String], joined: String) -> String? {
    let patterns = ["kubectl ", "terraform ", "helm ", "aws ", "gcloud ", "az ", "vault ", "nomad ", "consul "]
    for line in lines.reversed() {
        let cleaned = line.trimmingCharacters(in: .whitespacesAndNewlines)
        let lower = cleaned.lowercased()
        for pattern in patterns {
            if let range = lower.range(of: pattern) {
                return String(cleaned[range.lowerBound...])
            }
        }
    }
    let lowerJoined = joined.lowercased()
    if let range = lowerJoined.range(of: "kubernetes delete deployment") {
        return normalizeInlineCommand(String(joined[range.lowerBound...]))
    }
    if let range = lowerJoined.range(of: "kubectl delete deployment") {
        return normalizeInlineCommand(String(joined[range.lowerBound...]))
    }
    if lowerJoined.contains("delete deployment") && (lowerJoined.contains("kubectl") || lowerJoined.contains("kubernetes")) {
        return "kubectl delete deployment"
    }
    if lowerJoined.contains("terraform apply") {
        return "terraform apply"
    }
    if lowerJoined.contains("helm uninstall") {
        return "helm uninstall"
    }
    if lowerJoined.contains("aws route53") && (lowerJoined.contains("delete") || lowerJoined.contains("change")) {
        return "aws route53 change-resource-record-sets"
    }
    return nil
}

private func isLikelyRoute53Screen(_ lower: String) -> Bool {
    (lower.contains("route 53") || lower.contains("route53") || lower.contains("hosted zone")) &&
    (lower.contains("create") || lower.contains("record") || lower.contains("zone"))
}

private func isHostedZoneListScreen(_ lower: String) -> Bool {
    lower.contains("hosted zones") &&
        (lower.contains("view details") || lower.contains("edit") || lower.contains("delete")) &&
        lower.contains("create hosted zone")
}

private func isHostedZoneCreateScreen(_ lower: String) -> Bool {
    lower.contains("create hosted zone") &&
        (lower.contains("hosted zone configuration") ||
            lower.contains("domain name") ||
            lower.contains("public hosted zone") ||
            lower.contains("private hosted zone"))
}

private func normalizeInlineCommand(_ command: String) -> String {
    command
        .replacingOccurrences(of: "\n", with: " ")
        .split(separator: " ")
        .map(String.init)
        .joined(separator: " ")
}

private func inferAction(_ command: String) -> String {
    let lower = command.lowercased()
    for action in ["delete", "create", "apply", "deploy", "destroy", "update", "edit", "rollback", "release"] {
        if lower.contains(action) {
            return action
        }
    }
    return ""
}

private func inferTerminalResourceType(_ command: String) -> String {
    let lower = command.lowercased()
    for resource in ["deployment", "hosted_zone", "zone", "record", "pod", "service", "security group", "role", "policy"] {
        if lower.contains(resource.replacingOccurrences(of: "_", with: " ")) || lower.contains(resource) {
            return resource
        }
    }
    if lower.contains("kubectl") || lower.contains("kubernetes") {
        return "kubernetes"
    }
    return ""
}

private func extractDomain(from text: String) -> String? {
    extractDomains(from: text).first
}

private func extractHostedZoneDomain(from lines: [String]) -> String? {
    for (index, line) in lines.enumerated() {
        let lower = normalizedOCRLine(line).lowercased()
        if lower.contains("domain name") {
            for candidate in domainCandidates(near: index, in: lines) {
                return candidate
            }
        }
    }

    for line in lines {
        let lower = normalizedOCRLine(line).lowercased()
        if lower.contains("hosted zone for "),
           let domain = extractDomains(from: lower).first {
            return domain
        }
    }
    return nil
}

private func domainCandidates(near index: Int, in lines: [String]) -> [String] {
    let upperBound = min(lines.count-1, index+4)
    guard upperBound >= index else { return [] }
    var matches: [String] = []
    for offset in index...upperBound {
        let line = normalizedOCRLine(lines[offset])
        if shouldIgnoreDomainCandidateLine(line) {
            continue
        }
        matches.append(contentsOf: extractDomains(from: line))
    }
    return dedupe(matches)
}

private func shouldIgnoreDomainCandidateLine(_ line: String) -> Bool {
    let lower = line.lowercased()
    if lower.isEmpty {
        return true
    }
    let ignoredFragments = [
        "this is the name",
        "valid characters",
        "hosted zone configuration",
        "description",
        "optional",
        "route traffic",
        "the hosted zone is used for",
    ]
    return ignoredFragments.contains { lower.contains($0) }
}

private func extractDomains(from text: String) -> [String] {
    let pattern = #"\b([a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+)\b"#
    guard let regex = try? NSRegularExpression(pattern: pattern) else {
        return []
    }
    let normalized = normalizedOCRLine(text).lowercased()
    let nsText = normalized as NSString
    let matches = regex.matches(in: normalized, range: NSRange(location: 0, length: nsText.length))
    var domains: [String] = []
    for match in matches {
        let value = nsText.substring(with: match.range(at: 1)).lowercased()
        if isPlausibleObservedDomain(value) {
            domains.append(value)
        }
    }
    return dedupe(domains)
}

private func isPlausibleObservedDomain(_ value: String) -> Bool {
    guard value.contains(".") else {
        return false
    }
    if value.hasPrefix("aws.") || value.hasSuffix(".amazon.com") || value.hasSuffix(".amazonaws.com") {
        return false
    }
    let ignoredSuffixes = [".png", ".jpg", ".jpeg", ".json", ".yaml", ".yml", ".swift", ".go"]
    if ignoredSuffixes.contains(where: { value.hasSuffix($0) }) {
        return false
    }
    return true
}

private func normalizedOCRLine(_ line: String) -> String {
    line
        .replacingOccurrences(of: "|", with: " ")
        .replacingOccurrences(of: "•", with: " ")
        .replacingOccurrences(of: "—", with: "-")
        .replacingOccurrences(of: "’", with: "'")
        .trimmingCharacters(in: .whitespacesAndNewlines)
}

private func dedupe(_ values: [String]) -> [String] {
    var seen: Set<String> = []
    var output: [String] = []
    for value in values {
        if seen.insert(value).inserted {
            output.append(value)
        }
    }
    return output
}

private func compact(_ values: [String: String?]) -> [String: String] {
    var output: [String: String] = [:]
    for (key, value) in values {
        if let value, !value.isEmpty {
            output[key] = value
        }
    }
    return output
}

private func emptyToNil(_ value: String) -> String? {
    value.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ? nil : value
}
