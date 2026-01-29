#!/bin/bash

set -euo pipefail

# Directory of the script file, independent of where it's called from.
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$DIR/.."

# Apply the SQLite BIGINT patch before generating. SQLite doesn't care about
# column type sizes (all integers are 64-bit internally), but sqlc generates
# int32 for INTEGER and int64 for BIGINT.
echo "Applying SQLite bigint patch..."
for file in "$REPO_ROOT"/internal/db/migrations/*.up.sql; do
    if [ -f "$file" ]; then
        echo "Patching $file"
        sed -i.bak -E 's/INTEGER PRIMARY KEY/BIGINT PRIMARY KEY/g' "$file"
    fi
done

# Restore original files on exit.
cleanup() {
    echo "Restoring original schema files..."
    for file in "$REPO_ROOT"/internal/db/migrations/*.up.sql.bak; do
        if [ -f "$file" ]; then
            mv "$file" "${file%.bak}"
        fi
    done
}
trap cleanup EXIT

echo "Generating SQL models and queries with sqlc..."

docker run \
    --rm \
    --user "$UID:$(id -g)" \
    -v "$REPO_ROOT:/build" \
    -w /build \
    sqlc/sqlc:1.29.0 generate

echo "Done!"
