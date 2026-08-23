#!/bin/sh
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
. "$repository_root/caddy/versions.env"

if [ "$#" -ne 2 ]; then
    echo "usage: package_macos_app.sh <version> <output-dir>" >&2
    exit 2
fi

version=${1#v}
output_dir=$2
if ! printf '%s\n' "$version" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$'; then
    echo "Version must be a semantic version" >&2
    exit 2
fi
if [ "$(uname -s)" != "Darwin" ] || [ "$(uname -m)" != "arm64" ]; then
    echo "macOS app packaging must run on an Apple Silicon macOS host" >&2
    exit 2
fi

destination="$output_dir/DavDeck.app"
if [ -e "$destination" ]; then
    echo "Refusing to overwrite existing app bundle: $destination" >&2
    exit 1
fi

temporary_directory=$(mktemp -d)
cleanup() {
    rm -rf "$temporary_directory"
}
trap cleanup EXIT HUP INT TERM

build_package=davdeck.dev/davdeck/core/internal/buildinfo
git_commit=${DAVDECK_GIT_COMMIT:-$(git -C "$repository_root" rev-parse HEAD)}
build_date=${DAVDECK_BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}
ldflags="-s -w -X ${build_package}.Version=${version} -X ${build_package}.GitCommit=${git_commit} -X ${build_package}.BuildDate=${build_date} -X ${build_package}.FlutterVersion=${DAVDECK_FLUTTER_VERSION:-3.44.4} -X ${build_package}.CaddyVersion=${CADDY_VERSION} -X ${build_package}.WebDAVVersion=${CADDY_WEBDAV_VERSION}"

caddy_binary=${DAVDECK_CADDY_BINARY:-$repository_root/core/bin/caddy}
if [ -n "${DAVDECK_CADDY_BINARY:-}" ]; then
    "$repository_root/scripts/verify_caddy.sh" "$caddy_binary"
else
    "$repository_root/scripts/build_caddy.sh" "$caddy_binary"
fi
if ! lipo -archs "$caddy_binary" | tr ' ' '\n' | grep -Fxq arm64; then
    echo "Caddy binary does not contain the required arm64 architecture" >&2
    exit 1
fi

mkdir -p "$temporary_directory/bin"
(
    cd "$repository_root/core"
    CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -buildvcs=false -ldflags "$ldflags" -o "$temporary_directory/bin/davd" ./cmd/davd
    CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -buildvcs=false -ldflags "$ldflags" -o "$temporary_directory/bin/davctl" ./cmd/davctl
)

if [ -n "${DAVDECK_GUI_BUNDLE:-}" ]; then
    gui_bundle=$DAVDECK_GUI_BUNDLE
    if [ ! -d "$gui_bundle" ]; then
        echo "GUI bundle does not exist: $gui_bundle" >&2
        exit 1
    fi
else
    (
        cd "$repository_root/gui"
        flutter build macos --release
    )
    gui_bundle="$repository_root/gui/build/macos/Build/Products/Release/DavDeck.app"
fi

mkdir -p "$output_dir"
ditto "$gui_bundle" "$destination"
runtime_directory="$destination/Contents/Resources/DavDeck/bin"
mkdir -p "$runtime_directory"
cp "$temporary_directory/bin/davd" "$runtime_directory/davd"
cp "$temporary_directory/bin/davctl" "$runtime_directory/davctl"
cp "$caddy_binary" "$runtime_directory/caddy"
chmod 0755 "$runtime_directory/davd" "$runtime_directory/davctl" "$runtime_directory/caddy"

printf '%s\n' \
    '{' \
    '  "schema_version": 1,' \
    '  "product": "DavDeck",' \
    "  \"version\": \"$version\", " \
    "  \"git_commit\": \"$git_commit\", " \
    "  \"build_date\": \"$build_date\", " \
    "  \"caddy_version\": \"$CADDY_VERSION\", " \
    "  \"caddy_webdav_version\": \"$CADDY_WEBDAV_VERSION\", " \
    '  "target_os": "darwin",' \
    '  "target_arch": "arm64"' \
    '}' > "$destination/Contents/Resources/DavDeck/manifest.json"

signing_identity=${DAVDECK_CODESIGN_IDENTITY:--}
codesign --force --sign "$signing_identity" "$runtime_directory/davd"
codesign --force --sign "$signing_identity" "$runtime_directory/davctl"
codesign --force --sign "$signing_identity" "$runtime_directory/caddy"
codesign --force --sign "$signing_identity" "$destination"
codesign --verify --deep --strict "$destination"

printf 'Created complete macOS app: %s\n' "$destination"
