#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
    echo "usage: smoke_linux_release.sh <linux-server-tar.gz>" >&2
    exit 2
fi
archive=$1
[ -f "$archive" ] || { echo "Archive not found: $archive" >&2; exit 2; }
command -v curl >/dev/null 2>&1 || { echo "curl is required" >&2; exit 2; }

test_directory=$(mktemp -d "${TMPDIR:-/tmp}/davdeck-release-smoke.XXXXXX")
cleanup() {
    if [ -n "${daemon_pid:-}" ]; then
        kill -TERM "$daemon_pid" 2>/dev/null || true
        wait "$daemon_pid" 2>/dev/null || true
    fi
    rm -rf "$test_directory"
}
trap cleanup EXIT HUP INT TERM

tar -xzf "$archive" -C "$test_directory"
package_name=$(tar -tzf "$archive" | sed -n '1s#/.*##p')
package="$test_directory/$package_name"
[ -n "$package_name" ] && [ -d "$package" ] || { echo "Archive root is invalid" >&2; exit 1; }
grep -Fq '"target_os": "linux"' "$package/manifest.json"
grep -Fq '"flavor": "server"' "$package/manifest.json"
case "$(uname -m)" in
    x86_64|amd64) expected_arch=amd64 ;;
    aarch64|arm64) expected_arch=arm64 ;;
    *) echo "Unsupported Linux architecture: $(uname -m)" >&2; exit 1 ;;
esac
grep -Fq "\"target_arch\": \"$expected_arch\"" "$package/manifest.json"
for required in "$package/bin/davd" "$package/bin/davctl" "$package/libexec/caddy" "$package/install.sh" "$package/uninstall.sh" "$package/systemd/davdeck.service.in"; do
    [ -f "$required" ] || { echo "Missing release file: $required" >&2; exit 1; }
    if [ "${required##*/}" != davdeck.service.in ] && [ ! -x "$required" ]; then
        echo "Release file is not executable: $required" >&2
        exit 1
    fi
done

"$package/bin/davctl" version --json | grep -Fq '"target_os":"linux"'
task_directory="$test_directory/task"
mkdir -p "$task_directory/data" "$task_directory/config" "$task_directory/run" "$task_directory/share"
"$package/bin/davd" \
    --caddy-binary "$package/libexec/caddy" \
    --portable-owner gui \
    --data-dir "$task_directory/data" \
    --config-dir "$task_directory/config" \
    --runtime-dir "$task_directory/run" \
    >"$task_directory/davd.log" 2>&1 &
daemon_pid=$!

ready=0
for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
    if [ -s "$task_directory/run/management.endpoint" ]; then
        ready=1
        break
    fi
    kill -0 "$daemon_pid" 2>/dev/null || break
    sleep 1
done
[ "$ready" -eq 1 ] || { cat "$task_directory/davd.log" >&2; exit 1; }

endpoint=$(tr -d '[:space:]' < "$task_directory/run/management.endpoint")
export DAVDECK_ENDPOINT="$endpoint"
export DAVDECK_TOKEN_FILE="$task_directory/config/management.token"
"$package/bin/davctl" status
"$package/bin/davctl" doctor || true
port_offset=$(($$ % 1000))
http_port=$((18080 + port_offset))
https_port=$((18443 + port_offset))
"$package/bin/davctl" server ports --http "$http_port" --https "$https_port"
password=$(od -An -N18 -tu1 /dev/urandom | tr -d '[:space:]')
printf '%s\n' "$password" | "$package/bin/davctl" user add smoke --password-stdin
"$package/bin/davctl" share add Smoke "$task_directory/share"
"$package/bin/davctl" acl set smoke smoke read-write
"$package/bin/davctl" config apply
printf 'linux-release-smoke\n' > "$task_directory/payload"
curl --fail --silent --show-error --user "smoke:$password" --upload-file "$task_directory/payload" "http://127.0.0.1:$http_port/dav/smoke/upload.txt"
curl --fail --silent --show-error --user "smoke:$password" "http://127.0.0.1:$http_port/dav/smoke/upload.txt" > "$task_directory/download"
cmp "$task_directory/payload" "$task_directory/download"
echo "Linux Server release smoke passed: $package_name"
