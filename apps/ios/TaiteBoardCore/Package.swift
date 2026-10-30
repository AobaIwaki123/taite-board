// swift-tools-version: 6.4

import PackageDescription

let package = Package(
    name: "TaiteBoardCore",
    products: [
        .library(
            name: "TaiteBoardCore",
            targets: ["TaiteBoardCore"]
        )
    ],
    targets: [
        .target(name: "TaiteBoardCore"),
        .testTarget(
            name: "TaiteBoardCoreTests",
            dependencies: ["TaiteBoardCore"]
        )
    ]
)
