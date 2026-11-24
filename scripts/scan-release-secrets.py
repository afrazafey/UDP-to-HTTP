#!/usr/bin/env python3
"""Review candidate files and archive members for recognizable credential formats."""
import argparse
import io
import json
from pathlib import Path
import re
import tarfile
import zipfile

RULES = {
    "private key": rb"-----BEGIN [A-Z ]*PRIVATE KEY-----\r?\n[A-Za-z0-9+/]{40,}",
    "GitHub token": rb"\b(?:gh[pousr]_[A-Za-z0-9]{36,255}|github_pat_[A-Za-z0-9_]{75,255})\b",
    "AWS access key": rb"\b(?:AKIA|ASIA)[A-Z0-9]{16}\b",
    "npm token": rb"\bnpm_[A-Za-z0-9]{36}\b",
    "Slack token": rb"\bxox[baprs]-[0-9]{8,}-[A-Za-z0-9-]{12,}\b",
}
PATTERNS = {name: re.compile(pattern) for name, pattern in RULES.items()}


def members(name, content):
    if name.endswith(".zip"):
        with zipfile.ZipFile(io.BytesIO(content)) as archive:
            for entry in archive.infolist():
                if not entry.is_dir():
                    yield entry.filename, archive.read(entry)
    elif name.endswith(".tar.gz"):
        with tarfile.open(fileobj=io.BytesIO(content), mode="r:gz") as archive:
            for entry in archive:
                if entry.isfile():
                    yield entry.name, archive.extractfile(entry).read()
    elif name.endswith(".deb"):
        if not content.startswith(b"!<arch>\n"):
            raise ValueError("invalid Debian archive")
        offset = 8
        while offset + 60 <= len(content):
            header = content[offset:offset + 60]
            size = int(header[48:58])
            start = offset + 60
            yield header[:16].decode("ascii").strip().rstrip("/"), content[start:start + size]
            offset = start + size + size % 2


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("directory", type=Path)
    parser.add_argument("--report", type=Path, required=True)
    args = parser.parse_args()
    findings = []
    examined = 0

    def inspect(name, content, namespace=""):
        nonlocal examined
        location = f"{namespace}{name}"
        examined += 1
        for rule, pattern in PATTERNS.items():
            if pattern.search(content):
                findings.append({"file": location, "rule": rule})
        for child, data in members(name, content):
            inspect(child, data, location + "!")

    for line in (args.directory / "SHA256SUMS").read_text(encoding="utf-8").splitlines():
        _, name = line.split("  ", 1)
        if Path(name).name != name:
            raise ValueError("candidate manifest must contain basenames")
        inspect(name, (args.directory / name).read_bytes())
    report = {"scanner": "release credential pattern check", "files_and_members_examined": examined,
              "rules": list(RULES), "findings": findings}
    args.report.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    print(f"Examined {examined} files and archive members, with {len(findings)} matching locations")
    return bool(findings)


if __name__ == "__main__":
    raise SystemExit(main())
