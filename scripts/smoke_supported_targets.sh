#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "$0")/.." && pwd)"
smoke_directory="$(mktemp -d)"

cleanup() {
  rm -rf "${smoke_directory}"
}
trap cleanup EXIT

cd "${repository_root}/core"
for target in darwin/arm64 windows/amd64 linux/amd64 linux/arm64; do
  target_os="${target%/*}"
  target_arch="${target#*/}"
  suffix=""
  if [[ "${target_os}" == windows ]]; then
    suffix=".exe"
  fi
  daemon="${smoke_directory}/davd-${target_os}-${target_arch}${suffix}"
  client="${smoke_directory}/davctl-${target_os}-${target_arch}${suffix}"
  CGO_ENABLED=0 GOOS="${target_os}" GOARCH="${target_arch}" go build -trimpath -buildvcs=false -o "${daemon}" ./cmd/davd
  CGO_ENABLED=0 GOOS="${target_os}" GOARCH="${target_arch}" go build -trimpath -buildvcs=false -o "${client}" ./cmd/davctl
  go version -m "${daemon}" | awk -v os="GOOS=${target_os}" -v arch="GOARCH=${target_arch}" '
    $1 == "build" && $2 == os { found_os = 1 }
    $1 == "build" && $2 == arch { found_arch = 1 }
    END { exit !(found_os && found_arch) }
  '
  go version -m "${client}" | awk -v os="GOOS=${target_os}" -v arch="GOARCH=${target_arch}" '
    $1 == "build" && $2 == os { found_os = 1 }
    $1 == "build" && $2 == arch { found_arch = 1 }
    END { exit !(found_os && found_arch) }
  '
done

host_os="$(go env GOOS)"
host_arch="$(go env GOARCH)"
host_suffix=""
if [[ "${host_os}" == windows ]]; then
  host_suffix=".exe"
fi
DAVDECK_DAVD_BINARY="${smoke_directory}/davd-${host_os}-${host_arch}${host_suffix}" \
DAVDECK_DAVCTL_BINARY="${smoke_directory}/davctl-${host_os}-${host_arch}${host_suffix}" \
  "${repository_root}/scripts/smoke_phase0.sh"

echo "Supported-target build metadata and native daemon/CLI smoke passed"
