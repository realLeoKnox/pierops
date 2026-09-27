#!/usr/bin/env python3
"""Fail on common credential patterns or local-only files in publishable Git refs."""

from __future__ import annotations

import pathlib
import re
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
FORBIDDEN_PATHS = (
    re.compile(r"(^|/)(\.env(\.[^/]+)?|id_rsa|id_ed25519)$", re.I),
    re.compile(r"^(frontend|bin|\.local)/"),
    re.compile(r"(^|/)(data|cache|node_modules)/"),
    re.compile(r"\.(db|sqlite|pem|p12|pfx|key)$", re.I),
)
SECRET_PATTERNS = {
    "private-key": re.compile(rb"-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----"),
    "github-token": re.compile(rb"(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{50,})"),
    "aws-access-key": re.compile(rb"(?:AKIA|ASIA)[A-Z0-9]{16}"),
    "slack-token": re.compile(rb"xox[baprs]-[A-Za-z0-9-]{20,}"),
    "api-secret": re.compile(rb"sk-[A-Za-z0-9_-]{30,}"),
}


def git(*args: str) -> bytes:
    return subprocess.check_output(["git", *args], cwd=ROOT)


def inspect(path: str, data: bytes, source: str, findings: set[str]) -> None:
    if any(rule.search(path) for rule in FORBIDDEN_PATHS):
        findings.add(f"{source}: forbidden path: {path}")
    if data.startswith(b"SQLite format 3\x00"):
        findings.add(f"{source}: SQLite database: {path}")
    for name, pattern in SECRET_PATTERNS.items():
        if pattern.search(data):
            findings.add(f"{source}: possible {name}: {path}")


def main() -> int:
    findings: set[str] = set()
    for raw_path in git("ls-files", "-z").split(b"\x00"):
        if not raw_path:
            continue
        path = raw_path.decode("utf-8", "surrogateescape")
        file_path = ROOT / path
        if file_path.is_file():
            inspect(path, file_path.read_bytes(), "working tree", findings)

    for raw_ref in git("rev-list", "--all").splitlines():
        ref = raw_ref.decode("ascii")
        for entry in git("ls-tree", "-r", "-z", ref).split(b"\x00"):
            if not entry:
                continue
            metadata, raw_path = entry.split(b"\t", 1)
            _, kind, raw_oid = metadata.split(b" ")
            if kind != b"blob":
                continue
            oid = raw_oid.decode("ascii")
            path = raw_path.decode("utf-8", "surrogateescape")
            inspect(path, git("cat-file", "blob", oid), f"commit {ref[:10]}", findings)

    for finding in sorted(findings):
        print(finding, file=sys.stderr)
    if findings:
        print(f"Public repository check failed: {len(findings)} finding(s)", file=sys.stderr)
        return 1
    print("Public repository check passed (tracked tree and reachable commits).")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
