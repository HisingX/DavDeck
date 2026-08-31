#!/bin/sh
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
. "$repository_root/caddy/versions.env"

if [ "$#" -ne 3 ]; then
    echo "usage: package_release.sh <version> <darwin-arm64|windows-amd64|linux-amd64-server|linux-arm64-server|linux-amd64-desktop> <output-dir>" >&2
    exit 2
fi

version=${1#v}
target=$2
output_dir=$3
if ! printf '%s\n' "$version" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-rc\.[0-9]+)?$'; then
    echo "Version must be semantic version or release candidate" >&2
    exit 2
fi
case "$target" in
    darwin-arm64) target_os=darwin; target_arch=arm64; target_flavor=desktop; archive_format=tar.gz; executable_suffix= ;;
    windows-amd64) target_os=windows; target_arch=amd64; target_flavor=desktop; archive_format=zip; executable_suffix=.exe ;;
    linux-amd64-server) target_os=linux; target_arch=amd64; target_flavor=server; archive_format=tar.gz; executable_suffix= ;;
    linux-arm64-server) target_os=linux; target_arch=arm64; target_flavor=server; archive_format=tar.gz; executable_suffix= ;;
    linux-amd64-desktop) target_os=linux; target_arch=amd64; target_flavor=desktop; archive_format=tar.gz; executable_suffix= ;;
    # Keep the old target names as local-build compatibility aliases. Release
    # CI uses the explicit flavor names above and archives are named with the
    # caller's target for backward compatibility.
    linux-amd64) target_os=linux; target_arch=amd64; target_flavor=server; archive_format=tar.gz; executable_suffix= ;;
    linux-arm64) target_os=linux; target_arch=arm64; target_flavor=server; archive_format=tar.gz; executable_suffix= ;;
    *) echo "Unsupported release target: $target" >&2; exit 2 ;;
esac
if [ "$target" = linux-amd64 ] && [ -n "${DAVDECK_GUI_BUNDLE:-}" ]; then
    target_flavor=desktop
fi
if [ "$target_flavor" = server ] && [ -n "${DAVDECK_GUI_BUNDLE:-}" ]; then
    echo "A Linux Server release cannot include a GUI bundle: $target" >&2
    exit 2
fi

host_target="$(env -u GOOS -u GOARCH go env GOOS)-$(env -u GOOS -u GOARCH go env GOARCH)"
target_executable_target="$target_os-$target_arch"

case "${SOURCE_DATE_EPOCH:-}" in
    "") source_epoch=$(git -C "$repository_root" log -1 --format=%ct) ;;
    *[!0-9]*) echo "SOURCE_DATE_EPOCH must be a Unix timestamp" >&2; exit 2 ;;
    *) source_epoch=$SOURCE_DATE_EPOCH ;;
esac
git_commit=${DAVDECK_GIT_COMMIT:-$(git -C "$repository_root" rev-parse HEAD)}
if ! printf '%s\n' "$git_commit" | grep -Eq '^[0-9a-fA-F]{7,40}$'; then
    echo "DAVDECK_GIT_COMMIT must be a 7-40 character hexadecimal Git commit" >&2
    exit 2
fi

if build_date=$(date -u -r "$source_epoch" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null); then
    :
else
    build_date=$(date -u -d "@$source_epoch" +%Y-%m-%dT%H:%M:%SZ)
fi

temporary_directory=$(mktemp -d)
cleanup() {
    rm -rf "$temporary_directory"
}
trap cleanup EXIT HUP INT TERM

package_name="DavDeck-${version}-${target}"
stage="$temporary_directory/$package_name"
mkdir -p "$stage/bin" "$stage/libexec"

build_package=davdeck.dev/davdeck/core/internal/buildinfo
ldflags="-s -w -X ${build_package}.Version=${version} -X ${build_package}.GitCommit=${git_commit} -X ${build_package}.BuildDate=${build_date} -X ${build_package}.FlutterVersion=${DAVDECK_FLUTTER_VERSION:-3.44.4} -X ${build_package}.CaddyVersion=${CADDY_VERSION} -X ${build_package}.WebDAVVersion=${CADDY_WEBDAV_VERSION}"
(
    cd "$repository_root/core"
    CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" go build -trimpath -buildvcs=false -ldflags "$ldflags" -o "$stage/bin/davd${executable_suffix}" ./cmd/davd
    CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" go build -trimpath -buildvcs=false -ldflags "$ldflags" -o "$stage/bin/davctl${executable_suffix}" ./cmd/davctl
)

caddy_destination="$stage/libexec/caddy${executable_suffix}"
if [ -n "${DAVDECK_CADDY_BINARY:-}" ]; then
    if [ "$host_target" = "$target_executable_target" ]; then
        "$repository_root/scripts/verify_caddy.sh" "$DAVDECK_CADDY_BINARY"
    fi
    if caddy_metadata=$(go version -m "$DAVDECK_CADDY_BINARY" 2>/dev/null); then
        "$repository_root/scripts/verify_caddy_buildinfo.sh" "$DAVDECK_CADDY_BINARY"
        if ! printf '%s\n' "$caddy_metadata" | awk -v value="GOOS=$target_os" '$1 == "build" && $2 == value { found = 1 } END { exit !found }' || ! printf '%s\n' "$caddy_metadata" | awk -v value="GOARCH=$target_arch" '$1 == "build" && $2 == value { found = 1 } END { exit !found }'; then
            echo "Caddy binary target does not match $target_os/$target_arch" >&2
            exit 1
        fi
    elif [ "${DAVDECK_ALLOW_TEST_CADDY:-0}" != "1" ]; then
        echo "Caddy binary does not expose Go build metadata" >&2
        exit 1
    fi
    cp "$DAVDECK_CADDY_BINARY" "$caddy_destination"
else
    skip_exec=0
    if [ "$host_target" != "$target_executable_target" ]; then
        skip_exec=1
    fi
    GOOS="$target_os" GOARCH="$target_arch" DAVDECK_SKIP_CADDY_EXEC_VERIFY="$skip_exec" "$repository_root/scripts/build_caddy.sh" "$caddy_destination"
fi
chmod 0755 "$stage/bin/davd${executable_suffix}" "$stage/bin/davctl${executable_suffix}" "$caddy_destination"

desktop_included=false
if [ -n "${DAVDECK_GUI_BUNDLE:-}" ]; then
    if [ ! -e "$DAVDECK_GUI_BUNDLE" ]; then
        echo "GUI bundle does not exist: $DAVDECK_GUI_BUNDLE" >&2
        exit 1
    fi
    if [ "$target" = "windows-amd64" ]; then
        # A Windows Flutter release is already a runnable directory bundle.
        # Flatten it into the archive root so users can launch DavDeck.exe
        # without navigating through the build-system's Release directory.
        if [ ! -f "$DAVDECK_GUI_BUNDLE/DavDeck.exe" ]; then
            echo "Windows GUI bundle must contain DavDeck.exe" >&2
            exit 1
        fi
        cp -R "$DAVDECK_GUI_BUNDLE"/. "$stage/"
    elif [ "$target_flavor" = desktop ] && [ "$target_os" = linux ]; then
        mkdir -p "$stage/app"
        cp -R "$DAVDECK_GUI_BUNDLE"/. "$stage/app/"
        cp "$repository_root/packaging/linux/davdeck-launcher.sh" "$stage/davdeck"
        chmod 0755 "$stage/davdeck"
    else
        mkdir -p "$stage/desktop"
        cp -R "$DAVDECK_GUI_BUNDLE" "$stage/desktop/"
    fi
    desktop_included=true
fi

if [ "$target_flavor" = server ] && [ "$target_os" = linux ]; then
    cp "$repository_root/packaging/linux/install.sh" "$stage/install.sh"
    cp "$repository_root/packaging/linux/uninstall.sh" "$stage/uninstall.sh"
    mkdir -p "$stage/systemd"
    cp "$repository_root/packaging/linux/systemd/davdeck.service.in" "$stage/systemd/davdeck.service.in"
    chmod 0755 "$stage/install.sh" "$stage/uninstall.sh"
fi

# The macOS runner starts the bundled daemon from inside the app bundle. The
# generic release archive keeps headless binaries at the common archive root,
# so mirror the runtime into the location expected by AppDelegate.swift when a
# native macOS GUI is included.
if [ "$target" = "darwin-arm64" ] && [ "$desktop_included" = true ]; then
    macos_app="$stage/desktop/$(basename "$DAVDECK_GUI_BUNDLE")"
    macos_runtime="$macos_app/Contents/Resources/DavDeck/bin"
    mkdir -p "$macos_runtime"
    cp "$stage/bin/davd" "$macos_runtime/davd"
    cp "$stage/bin/davctl" "$macos_runtime/davctl"
    cp "$stage/libexec/caddy" "$macos_runtime/caddy"
    chmod 0755 "$macos_runtime/davd" "$macos_runtime/davctl" "$macos_runtime/caddy"
    if command -v codesign >/dev/null 2>&1 &&
        codesign --verify --deep --strict "$macos_app" >/dev/null 2>&1; then
        codesign --force --sign - "$macos_runtime/davd" "$macos_runtime/davctl" "$macos_runtime/caddy"
        codesign --force --sign - "$macos_app"
        codesign --verify --deep --strict "$macos_app"
    fi
fi

cp \
    "$repository_root/README.md" \
    "$repository_root/README.zh-CN.md" \
    "$repository_root/LICENSE" \
    "$repository_root/NOTICE" \
    "$repository_root/THIRD_PARTY_NOTICES.md" \
    "$repository_root/SECURITY.md" \
    "$stage/"
cp -R "$repository_root/third_party" "$stage/"
if [ "$target_os" = linux ]; then
    if [ "$target_flavor" = server ]; then
        cp "$repository_root/packaging/linux/README-server.md" "$stage/README.md"
        cp "$repository_root/packaging/linux/README-server.zh-CN.md" "$stage/README.zh-CN.md"
    elif [ "$target_flavor" = desktop ]; then
        cp "$repository_root/packaging/linux/README-desktop.md" "$stage/README.md"
        cp "$repository_root/packaging/linux/README-desktop.zh-CN.md" "$stage/README.zh-CN.md"
    fi
fi
go_version=$(go version | awk '{print $3}')
printf '%s\n' \
    '{' \
    '  "schema_version": 1,' \
    '  "product": "DavDeck",' \
    "  \"version\": \"$version\"," \
    "  \"git_commit\": \"$git_commit\"," \
    "  \"build_date\": \"$build_date\"," \
    "  \"go_version\": \"$go_version\"," \
    "  \"flutter_version\": \"${DAVDECK_FLUTTER_VERSION:-3.44.4}\"," \
    "  \"caddy_version\": \"$CADDY_VERSION\"," \
    "  \"caddy_webdav_version\": \"$CADDY_WEBDAV_VERSION\"," \
    "  \"target_os\": \"$target_os\"," \
    "  \"target_arch\": \"$target_arch\"," \
    "  \"flavor\": \"$target_flavor\"," \
    "  \"desktop_included\": $desktop_included," \
    '  "signed": false' \
    '}' > "$stage/manifest.json"

mkdir -p "$output_dir"
output_dir=$(CDPATH= cd -- "$output_dir" && pwd)
archive="$output_dir/$package_name.$archive_format"
if [ -e "$archive" ] || [ -e "$archive.sha256" ]; then
    echo "Release artifact already exists: $archive" >&2
    exit 1
fi
(
    cd "$repository_root/core"
    go run ./tools/releasepack -root "$stage" -output "$archive" -format "$archive_format" -epoch "$source_epoch"
)
printf 'Created %s\n' "$archive"
