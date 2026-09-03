#!/bin/sh
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
. "$repository_root/caddy/versions.env"

if [ "$#" -gt 0 ]; then
    binary=$1
else
    binary="$repository_root/core/bin/caddy"
    if [ "$(go env GOOS)" = windows ]; then
        binary="$binary.exe"
    fi
fi
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
if ! printf '%s\n' "$modules_output" | grep -Eq "^${CADDY_DISCOVERY_MODULE}([[:space:]]|$)"; then
    echo "Required Caddy module is missing: $CADDY_DISCOVERY_MODULE" >&2
    exit 1
fi
if ! printf '%s\n' "$modules_output" | grep -Eq "^${CADDY_RENEWAL_MODULE}([[:space:]]|$)"; then
    echo "Required Caddy module is missing: $CADDY_RENEWAL_MODULE" >&2
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
for module in "$CADDY_DNS_CLOUDFLARE_MODULE" "$CADDY_DNS_TENCENTCLOUD_MODULE" "$CADDY_DNS_DNSPOD_MODULE" "$CADDY_DNS_ALIDNS_MODULE"; do
    if ! printf '%s\n' "$modules_output" | grep -Eq "^${module}([[:space:]]|$)"; then
        echo "Required Caddy module is missing: $module" >&2
        exit 1
    fi
done
if ! printf '%s\n' "$modules_output" | grep -Fq "$CADDY_RENEWAL_PACKAGE"; then
    echo "Required Caddy package is missing: $CADDY_RENEWAL_PACKAGE" >&2
    exit 1
fi
if ! printf '%s\n' "$modules_output" | awk -v module="$CADDY_RENEWAL_MODULE" -v version="$CADDY_RENEWAL_VERSION" -v package="$CADDY_RENEWAL_PACKAGE" '$1 == module && $2 == version && $3 == package { found = 1 } END { exit !found }'; then
    echo "Expected DavDeck renewal module version is missing: $CADDY_RENEWAL_MODULE@$CADDY_RENEWAL_VERSION" >&2
    exit 1
fi
for package in "$CADDY_DNS_CLOUDFLARE_PACKAGE" "$CADDY_DNS_TENCENTCLOUD_PACKAGE" "$CADDY_DNS_DNSPOD_PACKAGE" "$CADDY_DNS_ALIDNS_PACKAGE"; do
    if ! printf '%s\n' "$modules_output" | grep -Fq "$package"; then
        echo "Required Caddy package is missing: $package" >&2
        exit 1
    fi
done
for module_version_package in \
    "$CADDY_DNS_CLOUDFLARE_MODULE $CADDY_DNS_CLOUDFLARE_VERSION $CADDY_DNS_CLOUDFLARE_PACKAGE" \
    "$CADDY_DNS_TENCENTCLOUD_MODULE $CADDY_DNS_TENCENTCLOUD_VERSION $CADDY_DNS_TENCENTCLOUD_PACKAGE" \
    "$CADDY_DNS_DNSPOD_MODULE $CADDY_DNS_DNSPOD_VERSION $CADDY_DNS_DNSPOD_PACKAGE" \
    "$CADDY_DNS_ALIDNS_MODULE $CADDY_DNS_ALIDNS_VERSION $CADDY_DNS_ALIDNS_PACKAGE"; do
    module=${module_version_package%% *}
    remainder=${module_version_package#* }
    version=${remainder%% *}
    package=${remainder#* }
    if ! printf '%s\n' "$modules_output" | awk -v module="$module" -v version="$version" -v package="$package" '$1 == module && $2 == version && $3 == package { found = 1 } END { exit !found }'; then
        echo "Expected Caddy DNS provider version is missing: $module@$version" >&2
        exit 1
    fi
done

printf 'Verified Caddy %s with %s and %s at %s\n' "$CADDY_VERSION" "$CADDY_WEBDAV_MODULE" "$CADDY_DISCOVERY_MODULE" "$CADDY_WEBDAV_VERSION"
