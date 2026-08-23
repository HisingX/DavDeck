#!/bin/sh
case "$1" in
    version) echo "@CADDY_VERSION@ test" ;;
    list-modules) echo "http.handlers.file_server" ;;
    *) exit 2 ;;
esac
