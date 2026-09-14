#!/bin/bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"
VERSION=${VERSION:-0.1.0}
if [[ ! $VERSION =~ ^[0-9]+(\.[0-9]+){0,2}$ ]]; then
    printf 'VERSION must contain one to three numeric components.\n' >&2
    exit 1
fi
if [[ ! -d "build/KING OF LUNCH.app" || ! -x build/kol ]]; then
    printf 'Run make build before packaging.\n' >&2
    exit 1
fi

mkdir -p dist
STAGING=$(mktemp -d "$ROOT/dist/.release.XXXXXX")
trap 'rm -rf "$STAGING"' EXIT
NAME="king-of-lunch-$VERSION-darwin-$(go env GOARCH)"
RELEASE="$STAGING/$NAME"
mkdir "$RELEASE"
ditto "build/KING OF LUNCH.app" "$RELEASE/KING OF LUNCH.app"
cp build/kol "$RELEASE/kol"
cp README.md "$RELEASE/README.md"
cp plan.md "$RELEASE/plan.md"
mkdir "$RELEASE/docs"
cp docs/validation.md "$RELEASE/docs/validation.md"
mkdir "$RELEASE/assets"
cp assets/king-of-lunch-logo.png "$RELEASE/assets/"
if [[ -d licenses ]]; then cp -R licenses "$RELEASE/"; fi
if [[ -f THIRD_PARTY_NOTICES.md ]]; then cp THIRD_PARTY_NOTICES.md "$RELEASE/"; fi
ditto -c -k --sequesterRsrc --keepParent "$RELEASE" "$STAGING/$NAME.zip"
mv -f "$STAGING/$NAME.zip" "dist/$NAME.zip"
printf 'Packaged dist/%s.zip\n' "$NAME"
