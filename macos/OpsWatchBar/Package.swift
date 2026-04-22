// swift-tools-version: 5.9

import PackageDescription

let package = Package(
    name: "OpsWatchBar",
    platforms: [.macOS(.v13)],
    products: [
        .executable(name: "OpsWatchBar", targets: ["OpsWatchBar"]),
        .executable(name: "OpsWatchOCR", targets: ["OpsWatchOCR"])
    ],
    targets: [
        .executableTarget(name: "OpsWatchBar"),
        .executableTarget(name: "OpsWatchOCR")
    ]
)
