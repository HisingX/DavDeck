#!/bin/sh
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
. "$repository_root/caddy/versions.env"
test_directory=$(mktemp -d)
trap 'rm -rf "$test_directory"' EXIT HUP INT TERM

fake="$test_directory/caddy"
sed \
    -e "s|@CADDY_VERSION@|$CADDY_VERSION|g" \
    -e "s|@MODULE@|$CADDY_WEBDAV_MODULE|g" \
    -e "s|@DISCOVERY_MODULE@|$CADDY_DISCOVERY_MODULE|g" \
    -e "s|@PACKAGE@|$CADDY_WEBDAV_PACKAGE|g" \
    -e "s|@WEBDAV_VERSION@|$CADDY_WEBDAV_VERSION|g" \
    -e "s|@CF_MODULE@|$CADDY_DNS_CLOUDFLARE_MODULE|g" \
    -e "s|@TENCENT_MODULE@|$CADDY_DNS_TENCENTCLOUD_MODULE|g" \
    -e "s|@DNSPOD_MODULE@|$CADDY_DNS_DNSPOD_MODULE|g" \
    -e "s|@ALI_MODULE@|$CADDY_DNS_ALIDNS_MODULE|g" \
    -e "s|@CF_PACKAGE@|$CADDY_DNS_CLOUDFLARE_PACKAGE|g" \
    -e "s|@TENCENT_PACKAGE@|$CADDY_DNS_TENCENTCLOUD_PACKAGE|g" \
    -e "s|@DNSPOD_PACKAGE@|$CADDY_DNS_DNSPOD_PACKAGE|g" \
    -e "s|@ALI_PACKAGE@|$CADDY_DNS_ALIDNS_PACKAGE|g" \
    -e "s|@CLOUDFLARE_VERSION@|$CADDY_DNS_CLOUDFLARE_VERSION|g" \
    -e "s|@TENCENT_VERSION@|$CADDY_DNS_TENCENTCLOUD_VERSION|g" \
    -e "s|@DNSPOD_VERSION@|$CADDY_DNS_DNSPOD_VERSION|g" \
    -e "s|@ALI_VERSION@|$CADDY_DNS_ALIDNS_VERSION|g" \
    "$repository_root/scripts/testdata/fake_caddy.sh" > "$fake"
chmod +x "$fake"
"$repository_root/scripts/verify_caddy.sh" "$fake" >/dev/null

missing="$test_directory/caddy-missing-module"
sed -e "s|@CADDY_VERSION@|$CADDY_VERSION|g" "$repository_root/scripts/testdata/fake_caddy_missing.sh" > "$missing"
chmod +x "$missing"
if "$repository_root/scripts/verify_caddy.sh" "$missing" >/dev/null 2>&1; then
    echo "verification accepted a Caddy binary without WebDAV" >&2
    exit 1
fi

echo "Caddy tooling tests passed"
