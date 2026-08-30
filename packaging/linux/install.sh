#!/bin/sh
set -eu

install_root=/opt/davdeck
data_dir=/var/lib/davdeck
config_dir=/etc/davdeck
runtime_dir=/run/davdeck
service_unit=/etc/systemd/system/davdeck.service
cli_link=/usr/local/bin/davctl

fail() {
    echo "DavDeck installer: $*" >&2
    exit 1
}

if [ "$(uname -s)" != "Linux" ]; then
    fail "this installer only supports Linux"
fi
if [ "$(id -u)" -ne 0 ]; then
    fail "administrator privileges are required; run: sudo ./install.sh"
fi
command -v systemctl >/dev/null 2>&1 || fail "systemctl was not found; a systemd host is required"

for directory in "$install_root" "$data_dir" "$config_dir"; do
    if [ -L "$directory" ]; then
        fail "$directory is a symlink; refusing to follow it"
    fi
    if [ -e "$directory" ] && [ ! -d "$directory" ]; then
        fail "$directory is not a directory"
    fi
done
if [ -L "$service_unit" ]; then
    fail "$service_unit is a symlink; refusing to replace it"
fi

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
manifest="$script_dir/manifest.json"
template="$script_dir/systemd/davdeck.service.in"
[ -f "$manifest" ] || fail "manifest.json is missing"
[ -f "$template" ] || fail "systemd service template is missing"
grep -Fq '"target_os": "linux"' "$manifest" || fail "this archive is not a Linux release"
grep -Fq '"flavor": "server"' "$manifest" || fail "this archive is not a Linux Server release"

case "$(uname -m)" in
    x86_64|amd64) expected_arch=amd64 ;;
    aarch64|arm64) expected_arch=arm64 ;;
    *) fail "unsupported Linux architecture: $(uname -m)" ;;
esac
grep -Fq "\"target_arch\": \"$expected_arch\"" "$manifest" || fail "release architecture does not match this host (expected $expected_arch)"

for required in \
    "$script_dir/bin/davd" \
    "$script_dir/bin/davctl" \
    "$script_dir/libexec/caddy"; do
    [ -f "$required" ] || fail "release file is missing: $required"
    [ -x "$required" ] || fail "release file is not executable: $required"
done

install_user=root
if [ -n "${SUDO_USER:-}" ] && [ "$SUDO_USER" != root ] && id "$SUDO_USER" >/dev/null 2>&1; then
    install_user=$SUDO_USER
fi
install_group=$(id -gn "$install_user")

if [ -e "$cli_link" ] || [ -L "$cli_link" ]; then
    if [ ! -L "$cli_link" ] || [ "$(readlink "$cli_link")" != "$install_root/bin/davctl" ]; then
        fail "$cli_link already exists and is not the DavDeck CLI link"
    fi
fi

temporary_directory=$(mktemp -d "${TMPDIR:-/tmp}/davdeck-install.XXXXXX")
cleanup() {
    rm -rf "$temporary_directory"
}
trap cleanup EXIT HUP INT TERM

if [ -f "$service_unit" ]; then
    systemctl stop davdeck.service || fail "could not stop the existing davdeck service"
fi

install -d -m 0755 "$install_root/bin" "$install_root/libexec"
install -d -m 0750 "$data_dir" "$config_dir"
install -m 0755 "$script_dir/bin/davd" "$install_root/bin/davd"
install -m 0755 "$script_dir/bin/davctl" "$install_root/bin/davctl"
install -m 0755 "$script_dir/libexec/caddy" "$install_root/libexec/caddy"

chown "$install_user:$install_group" "$data_dir" "$config_dir"
chmod 0750 "$data_dir" "$config_dir"
if [ -e "$data_dir/davdeck.db" ]; then
    chown "$install_user:$install_group" "$data_dir/davdeck.db"
    chmod 0600 "$data_dir/davdeck.db"
fi
if [ -e "$config_dir/management.token" ]; then
    chown "$install_user:$install_group" "$config_dir/management.token"
    chmod 0600 "$config_dir/management.token"
fi

user_directive=
if [ "$install_user" != root ]; then
    user_directive="User=$install_user"
fi
sed "s|^@USER_DIRECTIVE@$|$user_directive|" "$template" > "$temporary_directory/davdeck.service"
install -m 0644 "$temporary_directory/davdeck.service" "$service_unit"
if [ ! -L "$cli_link" ]; then
    ln -s "$install_root/bin/davctl" "$cli_link"
fi

systemctl daemon-reload
systemctl enable --now davdeck.service
systemctl is-active --quiet davdeck.service || fail "davdeck.service did not become active; inspect: journalctl -u davdeck.service"

ready=0
for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
    if [ -s "$runtime_dir/management.endpoint" ]; then
        ready=1
        break
    fi
    sleep 1
done
[ "$ready" -eq 1 ] || fail "davd did not publish its management endpoint; inspect: journalctl -u davdeck.service"

if ! status_output=$("$cli_link" status); then
    fail "davd is running but the management API smoke check failed"
fi
printf '%s\n' "$status_output"
printf '%s\n' "$status_output" | grep -Fq 'Caddy:    RUNNING' || fail "davd is running but managed Caddy is not healthy"
printf '%s\n' "$status_output" | grep -Fq 'WebDAV:   RUNNING' || fail "davd is running but WebDAV is not healthy"

echo
echo "DavDeck installed successfully."
echo
echo "Service: Running"
echo "Daemon:  Running"
echo "Caddy:   managed by davd"
echo
echo "Manage DavDeck with:"
echo
echo "  davctl"
