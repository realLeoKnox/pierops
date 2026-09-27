#!/bin/sh
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
frontend_dir="$project_dir/frontend"
target_dir="$project_dir/hub/web/public/defaultTheme"

"$project_dir/scripts/fetch-frontend.sh"
command -v npm >/dev/null 2>&1 || { echo 'npm is required' >&2; exit 1; }
command -v zstd >/dev/null 2>&1 || { echo 'zstd is required' >&2; exit 1; }
node "$project_dir/scripts/brand-frontend.mjs"

if [ ! -d "$frontend_dir/node_modules" ]; then
    (cd "$frontend_dir" && npm ci --no-audit --no-fund)
fi
(cd "$frontend_dir" && npm run build)

mkdir -p "$target_dir"
cp "$frontend_dir/komari-theme.json" "$target_dir/komari-theme.json"
tar -cf - -C "$frontend_dir/dist" . | zstd -q -f -o "$target_dir/dist.tar.zst"
test -s "$target_dir/dist.tar.zst"
echo "Frontend packed at $target_dir"
