#!/bin/sh
set -eu

# Flutter's macOS build only knows about the GUI. This build phase makes every
# locally built App bundle self-contained as well, including `flutter run` and
# `flutter build macos --debug`.
repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

if [ "$(uname -s)" != "Darwin" ] || [ "$(uname -m)" != "arm64" ]; then
    echo "DavDeck macOS runtime embedding requires an Apple Silicon macOS host" >&2
    exit 1
fi

if [ -z "${TARGET_BUILD_DIR:-}" ] || [ -z "${CONTENTS_FOLDER_PATH:-}" ]; then
    echo "DavDeck runtime embedding must run as an Xcode build phase" >&2
    exit 1
fi

go_command=$(command -v go || true)
if [ -z "$go_command" ]; then
    echo "Go is required to build the bundled davd runtime" >&2
    exit 1
fi

caddy_binary=${DAVDECK_CADDY_BINARY:-$repository_root/core/bin/caddy}
if [ ! -x "$caddy_binary" ]; then
    echo "Bundled Caddy is missing: $caddy_binary" >&2
    echo "Run 'make caddy-build' before building the macOS GUI" >&2
    exit 1
fi
if ! lipo -archs "$caddy_binary" | tr ' ' '\n' | grep -Fxq arm64; then
    echo "Bundled Caddy does not contain the arm64 architecture: $caddy_binary" >&2
    exit 1
fi

runtime_directory="$TARGET_BUILD_DIR/$CONTENTS_FOLDER_PATH/Resources/DavDeck/bin"
temporary_directory=$(mktemp -d "${TMPDIR:-/tmp}/davdeck-macos-runtime.XXXXXX")
cleanup() {
    rm -rf "$temporary_directory"
}
trap cleanup EXIT HUP INT TERM

(
    cd "$repository_root/core"
    CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
        "$go_command" build -trimpath -buildvcs=false -o "$temporary_directory/davd" ./cmd/davd
    CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
        "$go_command" build -trimpath -buildvcs=false -o "$temporary_directory/davctl" ./cmd/davctl
)

mkdir -p "$runtime_directory"
cp "$temporary_directory/davd" "$runtime_directory/davd"
cp "$temporary_directory/davctl" "$runtime_directory/davctl"
cp "$caddy_binary" "$runtime_directory/caddy"
chmod 0755 "$runtime_directory/davd" "$runtime_directory/davctl" "$runtime_directory/caddy"

# Sign nested executables before Xcode signs the outer application. Ad-hoc
# signing is the fallback for local debug builds; release packaging replaces
# it with the configured identity.
if command -v codesign >/dev/null 2>&1; then
    signing_identity=${EXPANDED_CODE_SIGN_IDENTITY:--}
    codesign --force --sign "$signing_identity" \
        "$runtime_directory/davd" "$runtime_directory/davctl" "$runtime_directory/caddy"
fi
