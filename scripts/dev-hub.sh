#!/bin/sh
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
if [ ! -x "$project_dir/bin/pierops-hub" ]; then
    echo 'Build first: make all' >&2
    exit 1
fi
cd "$project_dir/hub"
exec "$project_dir/bin/pierops-hub" server --listen "${PIEROPS_LISTEN:-127.0.0.1:25774}"

