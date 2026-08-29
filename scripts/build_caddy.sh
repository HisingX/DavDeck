#!/bin/sh
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
. "$repository_root/caddy/versions.env"

for value in "$CADDY_VERSION" "$XCADDY_VERSION" "$CADDY_WEBDAV_VERSION"; do
    case "$value" in
        ""|latest|master|main) echo "Caddy dependencies must use exact versions" >&2; exit 1 ;;
    esac
done

output=${1:-"$repository_root/core/bin/caddy"}
host_goos=$(env -u GOOS go env GOOS)
host_goarch=$(env -u GOARCH go env GOARCH)
target_goos=${DAVDECK_CADDY_GOOS:-${GOOS:-$host_goos}}
target_goarch=${DAVDECK_CADDY_GOARCH:-${GOARCH:-$host_goarch}}
if [ "$target_goos" = windows ] && [ "${output##*.}" = "$output" ]; then
    output="$output.exe"
fi
mkdir -p "$(dirname -- "$output")"

xcaddy_directory=$(mktemp -d)
cleanup() {
    rm -rf "$xcaddy_directory"
}
trap cleanup EXIT HUP INT TERM

GOBIN="$xcaddy_directory" GOOS="$host_goos" GOARCH="$host_goarch" \
    go install "github.com/caddyserver/xcaddy/cmd/xcaddy@$XCADDY_VERSION"

GOOS="$target_goos" GOARCH="$target_goarch" \
    "$xcaddy_directory/xcaddy" build "$CADDY_VERSION" \
    --with "$CADDY_WEBDAV_PACKAGE@$CADDY_WEBDAV_VERSION=$repository_root/caddy/caddy-webdav" \
    --output "$output"

"$repository_root/scripts/verify_caddy_buildinfo.sh" "$output"
if [ "${DAVDECK_SKIP_CADDY_EXEC_VERIFY:-0}" != "1" ]; then
    "$repository_root/scripts/verify_caddy.sh" "$output"
fi
