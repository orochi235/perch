// swift-tools-version:5.9
import PackageDescription

// PerchKit draws app.tint on a status item an app builds itself, from the same
// Icon.swift perch copies into every generated app.
let package = Package(
    name: "perch",
    platforms: [.macOS(.v14)],
    products: [
        .library(name: "PerchKit", targets: ["PerchKit"]),
    ],
    targets: [
        .target(
            name: "PerchKit",
            path: "internal/backend/swiftappkit/runtime",
            exclude: ["Runtime.swift", "Preview.swift", "Window.swift"],
            sources: ["Icon.swift", "StatusItem.swift"]
        ),
    ]
)
