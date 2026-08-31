#!/bin/sh
set -eu

install_root=/opt/davdeck
data_dir=/var/lib/davdeck
config_dir=/etc/davdeck
service_unit=/etc/systemd/system/davdeck.service
cli_link=/usr/local/bin/davctl

fail() {
    echo "DavDeck uninstaller: $*" >&2
    exit 1
}

if [ "$(uname -s)" != "Linux" ]; then
    fail "this uninstaller only supports Linux"
fi
if [ "$(id -u)" -ne 0 ]; then
    fail "administrator privileges are required; run: sudo ./uninstall.sh"
fi
command -v systemctl >/dev/null 2>&1 || fail "systemctl was not found"

if [ -f "$service_unit" ]; then
    systemctl disable --now davdeck.service || fail "could not stop and disable davdeck.service"
    rm -f "$service_unit"
    systemctl daemon-reload
fi

if [ -L "$cli_link" ]; then
    if [ "$(readlink "$cli_link")" = "$install_root/bin/davctl" ]; then
        rm -f "$cli_link"
    else
        fail "$cli_link is not the DavDeck CLI link; it was preserved"
    fi
elif [ -e "$cli_link" ]; then
    fail "$cli_link is not a symlink; it was preserved"
fi

if [ -L "$install_root" ]; then
    fail "$install_root is a symlink; refusing to remove it"
fi
if [ -e "$install_root" ] && [ ! -d "$install_root" ]; then
    fail "$install_root is not a directory; refusing to remove it"
fi
if [ -d "$install_root" ]; then
    rm -rf "$install_root"
fi

echo "DavDeck has been removed."
echo
echo "Configuration and data were preserved:"
echo
echo "  $config_dir"
echo "  $data_dir"
