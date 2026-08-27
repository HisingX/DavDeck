#!/bin/sh
case "$1" in
    version) echo "@CADDY_VERSION@ test" ;;
    list-modules) echo "@MODULE@ @WEBDAV_VERSION@ @PACKAGE@"; echo "@DISCOVERY_MODULE@" ;;
    *) exit 2 ;;
esac
