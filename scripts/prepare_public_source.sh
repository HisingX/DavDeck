#!/bin/sh
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

if [ "$#" -ne 1 ]; then
    echo "usage: prepare_public_source.sh <new-output-directory>" >&2
    exit 2
fi

output_dir=$1
case "$output_dir" in
    /*) ;;
    *)
        echo "Output directory must be an absolute path" >&2
        exit 2
        ;;
esac
case "$output_dir" in
    "/"|"$repository_root"|"$repository_root"/*)
        echo "Output directory must be outside the development repository" >&2
        exit 2
        ;;
esac
if [ -e "$output_dir" ]; then
    echo "Output directory already exists: $output_dir" >&2
    exit 2
fi
if ! command -v rg >/dev/null 2>&1; then
    echo "rg is required for the public-source scan" >&2
    exit 1
fi

staging_directory=$(mktemp -d)
cleanup() {
    rm -rf "$staging_directory"
}
trap cleanup EXIT HUP INT TERM

git -C "$repository_root" archive --format=tar HEAD | tar -xf - -C "$staging_directory"

# These paths contain private development process material and must not be
# copied into the public source snapshot. The snapshot intentionally has no
# Git history; initialize a new public repository from it after review.
for excluded_path in \
    AGENTS.md \
    docs/AI_WORKFLOW.md \
    docs/tasks \
    docs/NEXT_PHASE_PLAN.md \
    docs/ACCEPTANCE_1_0.md \
    local; do
    rm -rf "$staging_directory/$excluded_path"
done

for forbidden_pattern in \
    '-----BEGIN (RSA|EC|OPENSSH|DSA|PRIVATE) KEY-----' \
    'AKIA[0-9A-Z]{16}' \
    'gh[pousr]_[A-Za-z0-9_]{20,}' \
    'xox[baprs]-[A-Za-z0-9-]{20,}'; do
    if rg -n --hidden --glob '!.git/**' -e "$forbidden_pattern" "$staging_directory"; then
        echo "Potential secret found in public source snapshot" >&2
        exit 1
    fi
done

if find "$staging_directory" -type f \( \
    -name '*.db' -o \
    -name '*.sqlite' -o \
    -name '*.sqlite3' -o \
    -name 'management.token' \
    \) -print -quit | grep -q .; then
    echo "Runtime data or management token found in public source snapshot" >&2
    exit 1
fi

mkdir -p "$(dirname -- "$output_dir")"
mkdir "$output_dir"
cp -R "$staging_directory"/. "$output_dir"/
printf 'Prepared public source snapshot at %s\n' "$output_dir"
printf '%s\n' 'Review the snapshot, run a separate secret/license audit, then initialize a new Git repository.'
