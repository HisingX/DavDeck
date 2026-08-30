#!/bin/sh
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
. "$repository_root/caddy/versions.env"
test_directory=$(mktemp -d)
cleanup() {
    rm -rf "$test_directory"
}
trap cleanup EXIT HUP INT TERM

fake_caddy="$test_directory/caddy"
sed \
    -e "s|@CADDY_VERSION@|$CADDY_VERSION|g" \
    -e "s|@MODULE@|$CADDY_WEBDAV_MODULE|g" \
    -e "s|@DISCOVERY_MODULE@|$CADDY_DISCOVERY_MODULE|g" \
    -e "s|@PACKAGE@|$CADDY_WEBDAV_PACKAGE|g" \
    -e "s|@WEBDAV_VERSION@|$CADDY_WEBDAV_VERSION|g" \
    "$repository_root/scripts/testdata/fake_caddy.sh" > "$fake_caddy"
chmod +x "$fake_caddy"

host_target="$(go env GOOS)-$(go env GOARCH)"
case "$host_target" in
    darwin-arm64|windows-amd64|linux-amd64|linux-arm64) ;;
    *) echo "Release packaging test does not support host target: $host_target" >&2; exit 1 ;;
esac
case "$host_target" in
    windows-*) archive_suffix=zip; executable_suffix=.exe ;;
    *) archive_suffix=tar.gz; executable_suffix= ;;
esac

package_target=$host_target
case "$host_target" in
    linux-amd64) package_target=linux-amd64-desktop ;;
    linux-arm64) package_target=linux-arm64-server ;;
esac

gui_bundle=""
case "$host_target" in
    darwin-arm64)
        gui_bundle="$test_directory/DavDeck.app"
        mkdir -p "$gui_bundle/Contents/Resources"
        printf '%s\n' fake-gui > "$gui_bundle/Contents/Resources/fake"
        ;;
    windows-amd64)
        gui_bundle="$test_directory/windows-gui/Release"
        ;;
    linux-amd64)
        gui_bundle="$test_directory/linux-gui/bundle"
        mkdir -p "$gui_bundle/data/flutter_assets" "$gui_bundle/lib"
        printf '%s\n' fake-gui > "$gui_bundle/davdeck"
        printf '%s\n' fake-flutter > "$gui_bundle/lib/libflutter_linux_gtk.so"
        printf '%s\n' fake-assets > "$gui_bundle/data/flutter_assets/asset"
        chmod +x "$gui_bundle/davdeck"
        ;;
esac

windows_gui_bundle="$test_directory/windows-gui/Release"
mkdir -p "$windows_gui_bundle/data/flutter_assets"
printf '%s\n' fake-gui > "$windows_gui_bundle/DavDeck.exe"
printf '%s\n' fake-flutter > "$windows_gui_bundle/flutter_windows.dll"
printf '%s\n' fake-aot > "$windows_gui_bundle/data/app.so"
printf '%s\n' fake-icu > "$windows_gui_bundle/data/icudtl.dat"
printf '%s\n' fake-assets > "$windows_gui_bundle/data/flutter_assets/asset"

(
    cd "$test_directory"
    SOURCE_DATE_EPOCH=1700000000 DAVDECK_GIT_COMMIT=0123456789abcdef DAVDECK_CADDY_BINARY="$fake_caddy" DAVDECK_ALLOW_TEST_CADDY=1 DAVDECK_GUI_BUNDLE="$gui_bundle" \
        "$repository_root/scripts/package_release.sh" 1.0.0 "$package_target" output
)

package_name="DavDeck-1.0.0-$package_target"
archive="$test_directory/output/$package_name.$archive_suffix"
test -f "$archive"
test -f "$archive.sha256"
if [ "$archive_suffix" = zip ]; then
    unzip -Z1 "$archive" > "$test_directory/contents"
    unzip -q "$archive" -d "$test_directory/extracted"
else
    tar -tzf "$archive" > "$test_directory/contents"
    mkdir -p "$test_directory/extracted"
    tar -xzf "$archive" -C "$test_directory/extracted"
fi
for expected in \
    "$package_name/bin/davd$executable_suffix" \
    "$package_name/bin/davctl$executable_suffix" \
    "$package_name/libexec/caddy$executable_suffix" \
    "$package_name/manifest.json" \
    "$package_name/README.md" \
    "$package_name/README.zh-CN.md" \
    "$package_name/LICENSE" \
    "$package_name/NOTICE" \
    "$package_name/THIRD_PARTY_NOTICES.md" \
    "$package_name/third_party/license-reports/go-core.csv" \
    "$package_name/third_party/license-reports/go-caddy-webdav.csv" \
    "$package_name/SECURITY.md"; do
    grep -Fxq "$expected" "$test_directory/contents"
done
if [ "$host_target" = darwin-arm64 ]; then
    for expected in \
        "$package_name/desktop/DavDeck.app/Contents/Resources/DavDeck/bin/davd" \
        "$package_name/desktop/DavDeck.app/Contents/Resources/DavDeck/bin/davctl" \
        "$package_name/desktop/DavDeck.app/Contents/Resources/DavDeck/bin/caddy"; do
        grep -Fxq "$expected" "$test_directory/contents"
    done
elif [ "$host_target" = windows-amd64 ]; then
    for expected in \
        "$package_name/DavDeck.exe" \
        "$package_name/flutter_windows.dll" \
        "$package_name/data/app.so" \
        "$package_name/data/icudtl.dat" \
        "$package_name/data/flutter_assets/asset"; do
        grep -Fxq "$expected" "$test_directory/contents"
    done
    test ! -e "$test_directory/extracted/$package_name/desktop"
elif [ "$host_target" = linux-amd64 ]; then
    for expected in \
        "$package_name/davdeck" \
        "$package_name/app/davdeck" \
        "$package_name/app/data/flutter_assets/asset"; do
        grep -Fxq "$expected" "$test_directory/contents"
    done
    if grep -Fq "$package_name/install.sh" "$test_directory/contents"; then
        echo "Linux desktop archive contains server installer" >&2
        exit 1
    fi
elif [ "$host_target" = linux-arm64 ]; then
    for expected in \
        "$package_name/install.sh" \
        "$package_name/uninstall.sh" \
        "$package_name/systemd/davdeck.service.in"; do
        grep -Fxq "$expected" "$test_directory/contents"
    done
fi
manifest="$test_directory/extracted/$package_name/manifest.json"
if [ "$package_target" = linux-amd64-desktop ] || [ "$package_target" = darwin-arm64 ] || [ "$package_target" = windows-amd64 ]; then
    grep -Fq '"flavor": "desktop"' "$manifest"
else
    grep -Fq '"flavor": "server"' "$manifest"
fi
version_output=$("$test_directory/extracted/$package_name/bin/davctl$executable_suffix" version --json)
printf '%s\n' "$version_output" | grep -Fq '"version":"1.0.0"'
printf '%s\n' "$version_output" | grep -Fq '"caddy_version":"v2.11.4"'
(
    cd "$test_directory/output"
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum -c "$(basename "$archive").sha256"
    else
        shasum -a 256 -c "$(basename "$archive").sha256"
    fi
)

if [ "$host_target" = linux-amd64 ]; then
    server_package_name="DavDeck-1.0.0-linux-amd64-server"
    (
        cd "$test_directory"
        SOURCE_DATE_EPOCH=1700000000 DAVDECK_GIT_COMMIT=0123456789abcdef DAVDECK_CADDY_BINARY="$fake_caddy" DAVDECK_ALLOW_TEST_CADDY=1 \
            "$repository_root/scripts/package_release.sh" 1.0.0 linux-amd64-server server-output
    )
    server_archive="$test_directory/server-output/$server_package_name.tar.gz"
    test -f "$server_archive"
    tar -tzf "$server_archive" > "$test_directory/server-contents"
    for expected in \
        "$server_package_name/install.sh" \
        "$server_package_name/uninstall.sh" \
        "$server_package_name/systemd/davdeck.service.in" \
        "$server_package_name/bin/davd" \
        "$server_package_name/bin/davctl" \
        "$server_package_name/libexec/caddy"; do
        grep -Fxq "$expected" "$test_directory/server-contents"
    done
    tar -xOf "$server_archive" "$server_package_name/manifest.json" | grep -Fq '"flavor": "server"'
    if (
        cd "$test_directory"
        DAVDECK_CADDY_BINARY="$fake_caddy" DAVDECK_ALLOW_TEST_CADDY=1 DAVDECK_GUI_BUNDLE="$gui_bundle" \
            "$repository_root/scripts/package_release.sh" 1.0.0 linux-amd64-server invalid-server-output
    ); then
        echo "Linux server packaging accepted a GUI bundle" >&2
        exit 1
    fi
fi

windows_package_name="DavDeck-0.1.0-rc.3-windows-amd64"
(
    cd "$test_directory"
    SOURCE_DATE_EPOCH=1700000000 DAVDECK_GIT_COMMIT=0123456789abcdef DAVDECK_CADDY_BINARY="$fake_caddy" DAVDECK_ALLOW_TEST_CADDY=1 DAVDECK_GUI_BUNDLE="$windows_gui_bundle" \
        "$repository_root/scripts/package_release.sh" 0.1.0-rc.3 windows-amd64 windows-output
)
windows_archive="$test_directory/windows-output/$windows_package_name.zip"
test -f "$windows_archive"
unzip -Z1 "$windows_archive" > "$test_directory/windows-contents"
for expected in \
    "$windows_package_name/DavDeck.exe" \
    "$windows_package_name/flutter_windows.dll" \
    "$windows_package_name/data/app.so" \
    "$windows_package_name/data/icudtl.dat" \
    "$windows_package_name/data/flutter_assets/asset" \
    "$windows_package_name/bin/davd.exe" \
    "$windows_package_name/bin/davctl.exe" \
    "$windows_package_name/libexec/caddy.exe"; do
    grep -Fxq "$expected" "$test_directory/windows-contents"
done
if grep -Fq "$windows_package_name/desktop/" "$test_directory/windows-contents"; then
    echo "Windows GUI bundle was not flattened" >&2
    exit 1
fi

cross_target=linux-arm64-server
if [ "$host_target" = linux-arm64 ]; then
    cross_target=linux-amd64-server
fi
cross_package_name="DavDeck-0.1.0-rc.2-$cross_target"
(
    cd "$test_directory"
    SOURCE_DATE_EPOCH=1700000000 DAVDECK_GIT_COMMIT=0123456789abcdef DAVDECK_CADDY_BINARY="$fake_caddy" DAVDECK_ALLOW_TEST_CADDY=1 \
        "$repository_root/scripts/package_release.sh" 0.1.0-rc.2 "$cross_target" cross-output
)
cross_archive="$test_directory/cross-output/$cross_package_name.tar.gz"
test -f "$cross_archive"
tar -tzf "$cross_archive" > "$test_directory/cross-contents"
for expected in \
    "$cross_package_name/bin/davd" \
    "$cross_package_name/bin/davctl" \
    "$cross_package_name/libexec/caddy" \
    "$cross_package_name/manifest.json" \
    "$cross_package_name/README.md" \
    "$cross_package_name/README.zh-CN.md" \
    "$cross_package_name/LICENSE" \
    "$cross_package_name/NOTICE" \
    "$cross_package_name/THIRD_PARTY_NOTICES.md" \
    "$cross_package_name/third_party/license-reports/go-core.csv" \
    "$cross_package_name/third_party/license-reports/go-caddy-webdav.csv" \
    "$cross_package_name/SECURITY.md"; do
    grep -Fxq "$expected" "$test_directory/cross-contents"
done
printf 'Release packaging tests passed\n'
