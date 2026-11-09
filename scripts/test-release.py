#!/usr/bin/env python3
"""Validate built release archives and smoke-test the native packaged CLI."""

import hashlib
from pathlib import Path
import platform
import subprocess
import sys
import tarfile
import tempfile


def check(version, output):
    names = {
        f"rgd_{version}_{os_name}_{arch}.tar.gz"
        for os_name in ("darwin", "linux")
        for arch in ("amd64", "arm64")
    }
    if {path.name for path in output.iterdir()} != names | {"SHA256SUMS"}:
        raise ValueError("Release directory must contain exactly four archives and SHA256SUMS")
    checksums = {}
    for line in (output / "SHA256SUMS").read_text().splitlines():
        checksum, name = line.split()
        if name in checksums:
            raise ValueError(f"Duplicate checksum: {name}")
        checksums[name] = checksum
    if set(checksums) != names:
        raise ValueError("Checksum manifest does not match the expected archives")
    for name in sorted(names):
        path = output / name
        if hashlib.sha256(path.read_bytes()).hexdigest() != checksums[name]:
            raise ValueError(f"Checksum mismatch: {name}")
        with tarfile.open(path) as archive:
            members = archive.getmembers()
            if {member.name for member in members} != {"rgd", "README.md", "VERSION"} or len(members) != 3:
                raise ValueError(f"Unexpected archive contents: {name}")
            if not all(member.isfile() and member.size > 0 for member in members):
                raise ValueError(f"Non-regular or empty archive member: {name}")
            if not archive.getmember("rgd").mode & 0o111:
                raise ValueError(f"Binary is not executable: {name}")
            if archive.extractfile("VERSION").read().decode().strip() != version:
                raise ValueError(f"Incorrect packaged version: {name}")
    os_name = platform.system().lower()
    arch = {"aarch64": "arm64", "arm64": "arm64", "x86_64": "amd64"}.get(platform.machine())
    native = f"rgd_{version}_{os_name}_{arch}.tar.gz"
    if native not in names:
        raise ValueError("Run package validation on a supported release platform")
    with tempfile.TemporaryDirectory(prefix="rgd-package-test-") as root:
        binary = Path(root) / "rgd"
        with tarfile.open(output / native) as archive:
            binary.write_bytes(archive.extractfile("rgd").read())
        binary.chmod(0o755)
        help_text = subprocess.check_output([str(binary), "--help"], stderr=subprocess.STDOUT, text=True)
        if "rgd skill setup" not in help_text:
            raise ValueError("Native packaged CLI help is missing expected commands")
        guide = subprocess.check_output([str(binary), "skill", "show"], text=True)
        if "rgd guide set REVIEW_ID" not in guide:
            raise ValueError("Native packaged CLI is missing its embedded skill")
    print("Release validation passed: four archives, checksums, contents, native CLI, and embedded skill.")


if __name__ == "__main__":
    if len(sys.argv) != 3:
        sys.exit("Usage: python3 scripts/test-release.py VERSION OUTPUT_DIR")
    check(sys.argv[1], Path(sys.argv[2]))
