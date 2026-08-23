#!/bin/sh
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
. "$repository_root/caddy/versions.env"

binary=${1:-"$repository_root/core/bin/caddy"}
if [ ! -x "$binary" ]; then
    echo "Caddy binary is missing or not executable: $binary" >&2
    exit 1
fi

version_output=$($binary version)
case "$version_output" in
    *"$CADDY_VERSION"*) ;;
    *) echo "Expected Caddy $CADDY_VERSION, got: $version_output" >&2; exit 1 ;;
esac

modules_output=$($binary list-modules --packages --versions)
if ! printf '%s\n' "$modules_output" | grep -Eq "^${CADDY_WEBDAV_MODULE}([[:space:]]|$)"; then
    echo "Required Caddy module is missing: $CADDY_WEBDAV_MODULE" >&2
    exit 1
fi
if ! printf '%s\n' "$modules_output" | grep -Fq "$CADDY_WEBDAV_PACKAGE"; then
    echo "Required Caddy package is missing: $CADDY_WEBDAV_PACKAGE" >&2
    exit 1
fi
if ! printf '%s\n' "$modules_output" | grep -Fq "$CADDY_WEBDAV_VERSION"; then
    echo "Expected caddy-webdav version is missing: $CADDY_WEBDAV_VERSION" >&2
    exit 1
fi

printf 'Verified Caddy %s with %s at %s\n' "$CADDY_VERSION" "$CADDY_WEBDAV_MODULE" "$CADDY_WEBDAV_VERSION"
