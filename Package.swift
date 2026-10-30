// swift-tools-version:5.9
// openhongguo 的 Swift 包，供 App 通过 Swift Package Manager 引用：
//
//     .package(url: "https://github.com/AmatsuZero/openhongguo.git", from: "0.1.1")
//
// Go 编译出的 OpenhongguoCore.xcframework 从 GitHub Release 下载。本地开发和
// 测试用 bindings/mobile/ios/Package.swift，它引用 build.sh 生成的 xcframework。
import PackageDescription

let package = Package(
    name: "Openhongguo",
    platforms: [.iOS(.v13), .macOS(.v11)],
    products: [
        .library(name: "Openhongguo", targets: ["Openhongguo"]),
    ],
    targets: [
        .binaryTarget(
            name: "OpenhongguoCore",
            url: "https://github.com/AmatsuZero/openhongguo/releases/download/xcf-v0.1.1/OpenhongguoCore.xcframework.zip",
            checksum: "3f96355f5e8e7bed07b3fa8375a02dce4320f29d33ee1a02bdc280e8ca2978b8"
        ),
        .target(
            name: "Openhongguo",
            dependencies: ["OpenhongguoCore"],
            path: "bindings/mobile/ios/Sources/Openhongguo"
        ),
    ]
)
