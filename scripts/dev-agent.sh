#!/bin/sh
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
if [ ! -x "$project_dir/bin/pierops-agent" ]; then
    echo 'Build first: make all' >&2
    exit 1
fi
if [ -z "${AGENT_TOKEN:-}" ]; then
    echo 'Set AGENT_TOKEN to a token created in the Hub' >&2
    exit 1
fi
export AGENT_ENDPOINT="${AGENT_ENDPOINT:-http://127.0.0.1:25774}"
export AGENT_DISABLE_AUTO_UPDATE=1
export AGENT_DISABLE_WEB_SSH="${AGENT_DISABLE_WEB_SSH:-1}"
cd "$project_dir/agent"
exec "$project_dir/bin/pierops-agent"

