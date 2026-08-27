#!/usr/bin/env bash
set -euo pipefail

task_runtime_dir="$(mktemp -d)"
daemon_pid=""

cleanup() {
  if [[ -n "${daemon_pid}" ]]; then
    kill "${daemon_pid}" 2>/dev/null || true
    wait "${daemon_pid}" 2>/dev/null || true
  fi
  rm -rf "${task_runtime_dir}"
}
trap cleanup EXIT

cd "$(dirname "$0")/../core"
if [[ -n "${DAVDECK_DAVD_BINARY:-}" ]]; then
  daemon_command=("${DAVDECK_DAVD_BINARY}")
else
  daemon_command=(go run ./cmd/davd)
fi
if [[ -n "${DAVDECK_DAVCTL_BINARY:-}" ]]; then
  client_command=("${DAVDECK_DAVCTL_BINARY}")
else
  client_command=(go run ./cmd/davctl)
fi

daemon_arguments=(
  --data-dir "${task_runtime_dir}/data"
  --config-dir "${task_runtime_dir}/config"
  --runtime-dir "${task_runtime_dir}/run"
)
if [[ -n "${DAVDECK_CADDY_BINARY:-}" ]]; then
  daemon_arguments+=(--caddy-binary "${DAVDECK_CADDY_BINARY}")
else
  # This smoke validates the daemon and Management API. The Caddy/WebDAV
  # integration job supplies and exercises the pinned Caddy runtime.
  daemon_arguments+=(--portable-owner gui)
fi

"${daemon_command[@]}" "${daemon_arguments[@]}" \
  >"${task_runtime_dir}/davd.stdout" 2>"${task_runtime_dir}/davd.log" &
daemon_pid="$!"

for _ in {1..100}; do
  [[ -s "${task_runtime_dir}/run/management.endpoint" ]] && break
  sleep 0.1
done

endpoint_path="${task_runtime_dir}/run/management.endpoint"
if [[ ! -s "${endpoint_path}" ]]; then
  echo "davd did not create the management endpoint" >&2
  if [[ -s "${task_runtime_dir}/davd.log" ]]; then
    echo "--- davd stderr ---" >&2
    sed -n '1,240p' "${task_runtime_dir}/davd.log" >&2
  fi
  if [[ -s "${task_runtime_dir}/davd.stdout" ]]; then
    echo "--- davd stdout ---" >&2
    sed -n '1,240p' "${task_runtime_dir}/davd.stdout" >&2
  fi
  exit 1
fi

endpoint="$(<"${task_runtime_dir}/run/management.endpoint")"
"${client_command[@]}" \
  --endpoint "${endpoint}" \
  --token-file "${task_runtime_dir}/config/management.token" \
  status
"${client_command[@]}" \
  --endpoint "${endpoint}" \
  --token-file "${task_runtime_dir}/config/management.token" \
  logs --limit 5
