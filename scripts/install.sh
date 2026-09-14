#!/bin/bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
APP_DIR=${APP_DIR:-"$HOME/Applications"}
BIN_DIR=${BIN_DIR:-"$HOME/.local/bin"}
SOURCE_APP="$ROOT/build/KING OF LUNCH.app"

if [[ ! -d "$SOURCE_APP" || ! -x "$ROOT/build/kol" ]]; then
    printf 'Run make build before installing.\n' >&2
    exit 1
fi
mkdir -p "$APP_DIR" "$BIN_DIR"
APP_DIR=$(cd "$APP_DIR" && pwd -P)
BIN_DIR=$(cd "$BIN_DIR" && pwd -P)
DEST_APP="$APP_DIR/KING OF LUNCH.app"
if [[ "$DEST_APP" == "$SOURCE_APP" ]]; then
    printf 'APP_DIR must differ from the build directory.\n' >&2
    exit 1
fi
for target in "$BIN_DIR/kol" "$BIN_DIR/kol.app-path"; do
    if [[ -d "$target" ]]; then
        printf 'Cannot install over a directory: %s\n' "$target" >&2
        exit 1
    fi
done

APP_STAGE=$(mktemp -d "$APP_DIR/.king-of-lunch.XXXXXX")
BIN_STAGE=$(mktemp -d "$BIN_DIR/.king-of-lunch.XXXXXX")
trap 'rm -rf "$APP_STAGE" "$BIN_STAGE"' EXIT
ditto "$SOURCE_APP" "$APP_STAGE/KING OF LUNCH.app"
cp "$ROOT/build/kol" "$BIN_STAGE/kol"
chmod 755 "$BIN_STAGE/kol"
printf '%s\n' "$DEST_APP" > "$BIN_STAGE/kol.app-path"
codesign --verify --strict "$APP_STAGE/KING OF LUNCH.app"
codesign --verify --strict "$BIN_STAGE/kol"

# Move a previous installation aside until the replacement succeeds.
if [[ -e "$DEST_APP" || -L "$DEST_APP" ]]; then
    mv "$DEST_APP" "$APP_STAGE/previous.app"
fi
if ! mv "$APP_STAGE/KING OF LUNCH.app" "$DEST_APP"; then
    if [[ -e "$APP_STAGE/previous.app" || -L "$APP_STAGE/previous.app" ]]; then
        mv "$APP_STAGE/previous.app" "$DEST_APP"
    fi
    exit 1
fi
mv -f "$BIN_STAGE/kol.app-path" "$BIN_DIR/kol.app-path"
mv -f "$BIN_STAGE/kol" "$BIN_DIR/kol"

# Register eligibility with Finder without changing any default application.
LSREGISTER=/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister
if [[ -x "$LSREGISTER" ]]; then
    "$LSREGISTER" -f "$DEST_APP"
fi
printf 'Installed %s\nInstalled %s\n' "$DEST_APP" "$BIN_DIR/kol"
case ":$PATH:" in
    *":$BIN_DIR:"*) ;;
    *) printf 'Add %s to PATH to run kol by name. No shell files were modified.\n' "$BIN_DIR" ;;
esac
