#!/bin/sh
set -eu
project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_dir"
# Never label modified Agent source as a committed build.
if ! git diff --quiet HEAD -- agent; then
    echo 'Commit Agent source before packaging.' >&2
    exit 1
fi
source_commit=$(git rev-parse HEAD)
source_short=$(git rev-parse --short=12 HEAD)
version="pierops-m2-$source_short"
output_dir="$project_dir/bin/releases/$version"
mkdir -p "$output_dir"
for agent_arch in amd64 arm64; do
    echo "Building Linux Agent: $agent_arch"
    GOWORK=off GOOS=linux GOARCH="$agent_arch" CGO_ENABLED=0 \
        go -C agent build -trimpath \
        -ldflags="-X github.com/komari-monitor/komari-agent/update.CurrentVersion=$version" \
        -o "$output_dir/pierops-agent-linux-$agent_arch" .
done
python3 - "$project_dir" "$output_dir" "$source_commit" "$version" <<'PY'
import hashlib, json, pathlib, shutil, sys, tarfile, tempfile
from datetime import datetime, timezone
root, out = map(pathlib.Path, sys.argv[1:3])
commit, version = sys.argv[3:5]
checksums = []
for arch in ('amd64', 'arm64'):
    name = f'{version}-linux-{arch}'
    with tempfile.TemporaryDirectory(prefix='pierops-package-') as work:
        bundle = pathlib.Path(work) / name
        bundle.mkdir()
        shutil.copy2(out / f'pierops-agent-linux-{arch}', bundle / 'pierops-agent')
        for source in (root / 'packaging/agent').iterdir():
            if source.is_file(): shutil.copy2(source, bundle / source.name)
        shutil.copy2(root / 'docs/AGENT-POLICY.md', bundle / 'AGENT-POLICY.md')
        shutil.copy2(root / 'agent/LICENSE', bundle / 'LICENSE')
        (bundle / 'BUILD.json').write_text(json.dumps({
            'project': 'PierOps', 'stage': 'M2-development', 'source_commit': commit,
            'version': version, 'os': 'linux', 'arch': arch, 'cgo_enabled': False,
            'built_at_utc': datetime.now(timezone.utc).isoformat(),
            'validation': 'cross-compiled; real Linux/VPS runtime not verified'
        }, ensure_ascii=False, indent=2) + '\n')
        archive = out / f'{name}.tar.gz'
        with tarfile.open(archive, 'w:gz') as tar: tar.add(bundle, arcname=name)
        checksums.append(f'{hashlib.sha256(archive.read_bytes()).hexdigest()}  {archive.name}')
(out / 'SHA256SUMS').write_text('\n'.join(checksums) + '\n')
print(f'Packages: {out}')
PY
