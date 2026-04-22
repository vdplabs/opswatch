// swift-tools-version: 5.9

import PackageDescription

let package = Package(
    name: "OpsWatchBar",
    platforms: [.macOS(.v13)],
    products: [
        .executable(name: "OpsWatchBar", targets: ["OpsWatchBar"])
    ],
    targets: [
        .executableTarget(name: "OpsWatchBar")
    ]
)
