#!/bin/sh
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
. "$repository_root/caddy/versions.env"

binary=${1:-"$repository_root/core/bin/caddy"}
metadata=$(go version -m "$binary")

if ! printf '%s\n' "$metadata" | awk -v package="github.com/caddyserver/caddy/v2" -v version="$CADDY_VERSION" '$1 == "dep" && $2 == package && $3 == version { found = 1 } END { exit !found }'; then
    echo "Pinned Caddy dependency is missing from build metadata: $CADDY_VERSION" >&2
    exit 1
fi
if ! printf '%s\n' "$metadata" | awk -v package="$CADDY_WEBDAV_PACKAGE" -v version="$CADDY_WEBDAV_VERSION" '$1 == "dep" && $2 == package && $3 == version { found = 1 } END { exit !found }'; then
    echo "Pinned caddy-webdav dependency is missing from build metadata: $CADDY_WEBDAV_VERSION" >&2
    exit 1
fi

printf 'Verified Caddy build metadata %s with %s\n' "$CADDY_VERSION" "$CADDY_WEBDAV_VERSION"
