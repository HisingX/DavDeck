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

gui_bundle=""
case "$host_target" in
    darwin-arm64)
        gui_bundle="$test_directory/DavDeck.app"
        mkdir -p "$gui_bundle/Contents/Resources"
        printf '%s\n' fake-gui > "$gui_bundle/Contents/Resources/fake"
        ;;
    windows-amd64)
        gui_bundle="$test_directory/windows-gui/Release"
        mkdir -p "$gui_bundle"
        printf '%s\n' fake-gui > "$gui_bundle/davdeck.exe"
        ;;
esac

(
    cd "$test_directory"
    SOURCE_DATE_EPOCH=1700000000 DAVDECK_GIT_COMMIT=0123456789abcdef DAVDECK_CADDY_BINARY="$fake_caddy" DAVDECK_ALLOW_TEST_CADDY=1 DAVDECK_GUI_BUNDLE="$gui_bundle" \
        "$repository_root/scripts/package_release.sh" 0.1.0-rc.1 "$host_target" output
)

package_name="DavDeck-0.1.0-rc.1-$host_target"
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
    grep -Fxq "$package_name/desktop/Release/davdeck.exe" "$test_directory/contents"
fi
version_output=$("$test_directory/extracted/$package_name/bin/davctl$executable_suffix" version --json)
printf '%s\n' "$version_output" | grep -Fq '"version":"0.1.0-rc.1"'
printf '%s\n' "$version_output" | grep -Fq '"caddy_version":"v2.11.4"'
(
    cd "$test_directory/output"
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum -c "$(basename "$archive").sha256"
    else
        shasum -a 256 -c "$(basename "$archive").sha256"
    fi
)

cross_target=linux-arm64
if [ "$host_target" = linux-arm64 ]; then
    cross_target=linux-amd64
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
    "$cross_package_name/SECURITY.md"; do
    grep -Fxq "$expected" "$test_directory/cross-contents"
done
printf 'Release packaging tests passed\n'
