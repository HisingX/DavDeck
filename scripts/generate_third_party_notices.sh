#!/bin/sh
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
license_tool=${DAVDECK_GO_LICENSES_BIN:-"$(go env GOPATH)/bin/go-licenses"}
output_directory="$repository_root/third_party"

if [ ! -x "$license_tool" ]; then
    echo "go-licenses v2.0.1 is required; install it with:" >&2
    echo "  GOBIN=\"$(go env GOPATH)/bin\" go install github.com/google/go-licenses/v2@v2.0.1" >&2
    exit 1
fi
if [ -e "$output_directory" ]; then
    echo "Refusing to overwrite existing third-party notices: $output_directory" >&2
    exit 1
fi

mkdir -p \
    "$output_directory/license-reports" \
    "$output_directory/licenses"

(
    cd "$repository_root/core"
    "$license_tool" report ./cmd/davd ./cmd/davctl \
        --ignore davdeck.dev/davdeck/core \
        > "$output_directory/license-reports/go-core.csv"
    "$license_tool" save ./cmd/davd ./cmd/davctl \
        --ignore davdeck.dev/davdeck/core \
        --save_path "$output_directory/licenses/core"
)

(
    cd "$repository_root/caddy/caddy-webdav"
    "$license_tool" report . \
        --ignore github.com/mholt/caddy-webdav \
        > "$output_directory/license-reports/go-caddy-webdav.csv"
    "$license_tool" save . \
        --ignore github.com/mholt/caddy-webdav \
        --save_path "$output_directory/licenses/caddy-webdav"
)

cp "$repository_root/gui/pubspec.lock" "$output_directory/license-reports/flutter-pubspec.lock"
if flutter_binary=$(command -v flutter 2>/dev/null); then
    flutter_root=$(CDPATH= cd -- "$(dirname -- "$flutter_binary")/.." && pwd)
    if [ -f "$flutter_root/LICENSE" ]; then
        mkdir -p "$output_directory/licenses/flutter"
        cp "$flutter_root/LICENSE" "$output_directory/licenses/flutter/LICENSE"
    fi
fi

(
    cd "$repository_root/core"
    "$license_tool" check ./cmd/davd ./cmd/davctl \
        --ignore davdeck.dev/davdeck/core
)
(
    cd "$repository_root/caddy/caddy-webdav"
    "$license_tool" check . \
        --ignore github.com/mholt/caddy-webdav
)

printf '%s\n' \
    '# Third-Party License Review' \
    '' \
    'Generated with github.com/google/go-licenses/v2@v2.0.1.' \
    '' \
    '- The Core and Caddy/WebDAV dependency checks completed without unknown or forbidden license results.' \
    '- The analyzer can warn about non-Go source files such as assembly or C sources. Review those warnings on each release host.' \
    '- The reports include an MPL-2.0 dependency (github.com/go-sql-driver/mysql); its source and license are preserved in the Caddy/WebDAV license bundle.' \
    '- The exact Caddy binary and Flutter desktop bundles still require release-target review before publication.' \
    > "$output_directory/REVIEW.md"

printf '%s\n' 'Third-party notices generated. Review command warnings before publishing a release.'
