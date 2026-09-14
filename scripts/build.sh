#!/bin/bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"
VERSION=${VERSION:-0.1.0}

if [[ $(uname -s) != Darwin ]]; then
    printf 'KING OF LUNCH requires macOS and Apple Command Line Tools.\n' >&2
    exit 1
fi
if [[ ! $VERSION =~ ^[0-9]+(\.[0-9]+){0,2}$ ]]; then
    printf 'VERSION must contain one to three numeric components (for example 0.1.0).\n' >&2
    exit 1
fi
for tool in go xcrun sips iconutil codesign plutil; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        printf 'Missing build tool: %s. Install Go and Apple Command Line Tools.\n' "$tool" >&2
        exit 1
    fi
done
xcrun --find clang >/dev/null

case $(uname -m) in
    arm64) NATIVE_ARCH=arm64 ;;
    x86_64) NATIVE_ARCH=amd64 ;;
    *) printf 'Unsupported Mac architecture.\n' >&2; exit 1 ;;
esac
if [[ $(go env GOOS) != darwin || $(go env GOARCH) != "$NATIVE_ARCH" ]]; then
    printf 'Build on the target Mac with native GOOS/GOARCH; cross-compilation is not supported.\n' >&2
    exit 1
fi

mkdir -p build
STAGING=$(mktemp -d "$ROOT/build/.package.XXXXXX")
trap 'rm -rf "$STAGING"' EXIT
APP="$STAGING/KING OF LUNCH.app"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"

# The native bridge links system AppKit/WebKit; no browser engine is bundled.
export CGO_ENABLED=1
export MACOSX_DEPLOYMENT_TARGET=13.0
export CGO_CFLAGS="${CGO_CFLAGS:-} -mmacosx-version-min=13.0"
export CGO_LDFLAGS="${CGO_LDFLAGS:-} -mmacosx-version-min=13.0"
go build -trimpath -ldflags='-s -w -buildid=' -o "$APP/Contents/MacOS/king-of-lunch" ./cmd/king-of-lunch
go build -trimpath -ldflags='-s -w -buildid=' -o "$STAGING/kol" ./cmd/kol

cp packaging/Info.plist "$APP/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $VERSION" "$APP/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $VERSION" "$APP/Contents/Info.plist"
plutil -lint "$APP/Contents/Info.plist"
printf 'APPL????' > "$APP/Contents/PkgInfo"

ICONSET="$STAGING/AppIcon.iconset"
mkdir "$ICONSET"
for size in 16 32 128 256 512; do
    sips -z "$size" "$size" assets/king-of-lunch-logo.png --out "$ICONSET/icon_${size}x${size}.png" >/dev/null
    retina=$((size * 2))
    sips -z "$retina" "$retina" assets/king-of-lunch-logo.png --out "$ICONSET/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$ICONSET" -o "$APP/Contents/Resources/AppIcon.icns"

cp LICENSE "$APP/Contents/Resources/LICENSE"
if [[ -d licenses ]]; then
    cp -R licenses "$APP/Contents/Resources/"
fi
if [[ -f THIRD_PARTY_NOTICES.md ]]; then
    cp THIRD_PARTY_NOTICES.md "$APP/Contents/Resources/"
fi

codesign --force --sign - "$APP"
codesign --force --sign - "$STAGING/kol"
codesign --verify --strict "$APP"
codesign --verify --strict "$STAGING/kol"

rm -rf "build/KING OF LUNCH.app"
mv "$APP" "build/KING OF LUNCH.app"
mv -f "$STAGING/kol" build/kol
printf 'Built build/KING OF LUNCH.app and build/kol (%s, macOS 13+).\n' "$NATIVE_ARCH"
