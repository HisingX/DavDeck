#!/bin/sh
case "$1" in
    version) echo "@CADDY_VERSION@ test" ;;
    list-modules) echo "@MODULE@ @WEBDAV_VERSION@ @PACKAGE@"; echo "@DISCOVERY_MODULE@"; echo "@CF_MODULE@ @CLOUDFLARE_VERSION@ @CF_PACKAGE@"; echo "@TENCENT_MODULE@ @TENCENT_VERSION@ @TENCENT_PACKAGE@"; echo "@DNSPOD_MODULE@ @DNSPOD_VERSION@ @DNSPOD_PACKAGE@"; echo "@ALI_MODULE@ @ALI_VERSION@ @ALI_PACKAGE@" ;;
    *) exit 2 ;;
esac
