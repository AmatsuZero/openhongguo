#!/bin/sh
# build.sh 构建 Android 和 iOS 绑定。
#
#   ./build.sh android   生成 android/build/outputs/aar/openhongguo-release.aar
#   ./build.sh ios       生成 ios/OpenhongguoCore.xcframework
#   ./build.sh           两者都构建
#
# 需要 gomobile（go install golang.org/x/mobile/cmd/gomobile@latest）。
# Android 还需要 ANDROID_HOME、Android NDK 和 JDK 17 以上；iOS 需要 Xcode。
set -eu

here=$(cd "$(dirname "$0")" && pwd)
core="$here/core"

android() {
	: "${ANDROID_HOME:?set ANDROID_HOME to the Android SDK}"
	if [ -z "${ANDROID_NDK_HOME:-}" ]; then
		ANDROID_NDK_HOME=$(ls -d "$ANDROID_HOME"/ndk/* | sort -V | tail -1)
		export ANDROID_NDK_HOME
	fi
	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT
	(cd "$core" && gomobile bind -target android -androidapi 21 -javapkg dev.openhongguo \
		-trimpath -ldflags="-s -w" -o "$tmp/core.aar" .)
	# Gradle 不能把本地 aar 打进另一个 aar，所以拆成 jar 和 .so 放进库工程。
	rm -rf "$here/android/libs" "$here/android/src/main/jniLibs"
	mkdir -p "$here/android/libs" "$here/android/src/main/jniLibs"
	(cd "$tmp" && unzip -q core.aar classes.jar 'jni/*')
	mv "$tmp/classes.jar" "$here/android/libs/core.jar"
	cp -R "$tmp/jni/." "$here/android/src/main/jniLibs/"
	(cd "$here/android" && ./gradlew --quiet test assembleRelease)
	echo "built $here/android/build/outputs/aar/openhongguo-release.aar"
}

ios() {
	rm -rf "$here/ios/OpenhongguoCore.xcframework"
	(cd "$core" && gomobile bind -target ios,iossimulator,macos -iosversion 13 \
		-trimpath -ldflags="-s -w" -o "$here/ios/OpenhongguoCore.xcframework" .)
	echo "built $here/ios/OpenhongguoCore.xcframework"
}

case "${1:-all}" in
android) android ;;
ios) ios ;;
all) android && ios ;;
*)
	echo "usage: $0 [android|ios|all]" >&2
	exit 2
	;;
esac
