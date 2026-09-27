#!/bin/sh
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
frontend_dir="$project_dir/frontend"
upstream_commit=0321789bc1989e53df729dfc98bed2a2800c39c6

if [ -f "$frontend_dir/package.json" ]; then
    echo "Using local frontend source at $frontend_dir"
    exit 0
fi
if [ -e "$frontend_dir" ]; then
    echo "Frontend directory exists but is incomplete: $frontend_dir" >&2
    exit 1
fi

git init -q "$frontend_dir"
git -C "$frontend_dir" remote add origin https://github.com/komari-monitor/komari-web.git
git -C "$frontend_dir" fetch --depth=1 origin "$upstream_commit"
git -C "$frontend_dir" checkout -q --detach FETCH_HEAD
actual_commit=$(git -C "$frontend_dir" rev-parse HEAD)
if [ "$actual_commit" != "$upstream_commit" ]; then
    echo "Unexpected frontend commit: $actual_commit" >&2
    exit 1
fi
echo "Fetched pinned Komari Web UI $actual_commit"

