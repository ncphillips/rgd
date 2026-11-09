#!/usr/bin/env python3
"""Exercise the real installer offline with fake release downloads and platforms."""

import hashlib
import io
import os
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import unittest


INSTALLER = Path(__file__).resolve().parents[1] / "install.sh"
TAG = "v0.1.0"
BINARY = b"#!/bin/sh\nprintf 'rgd test fixture\\n'\n"


class InstallerTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="rgd-installer-test-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.home = self.root / "home"
        self.home.mkdir()
        self.mock = self.root / "mock-bin"
        self.mock.mkdir()
        self.releases = self.root / "releases"
        self.releases.mkdir()
        self.log = self.root / "requests.log"
        self.env = dict(
            os.environ,
            HOME=str(self.home),
            PATH=str(self.mock) + os.pathsep + os.environ["PATH"],
            RGD_VERSION="latest",
            RGD_INSTALL_DIR=str(self.home / ".local/bin"),
            MOCK_OS="Darwin",
            MOCK_ARCH="arm64",
            MOCK_RELEASES=str(self.releases),
            MOCK_REQUESTS=str(self.log),
        )
        self.write_mock("uname", """#!/bin/sh
case "$1" in
  -s) printf '%s\\n' "$MOCK_OS" ;;
  -m) printf '%s\\n' "$MOCK_ARCH" ;;
  *) exit 1 ;;
esac
""")
        self.write_mock("curl", f"#!{sys.executable}\n" + """
import os, pathlib, shutil, sys
args = sys.argv[1:]
url = next(arg for arg in args if arg.startswith('https://'))
with open(os.environ['MOCK_REQUESTS'], 'a') as log:
    log.write(url + '\\n')
if '--write-out' in args:
    print('https://github.com/ncphillips/rgd/releases/tag/v0.1.0', end='')
else:
    source = pathlib.Path(os.environ['MOCK_RELEASES']) / url.rsplit('/', 1)[-1]
    if not source.exists():
        sys.exit(22)
    shutil.copyfile(source, args[args.index('--output') + 1])
""")
        for platform in ("darwin", "linux"):
            for arch in ("amd64", "arm64"):
                self.archive(platform, arch)
        self.checksums()

    def write_mock(self, name, content):
        path = self.mock / name
        path.write_text(content)
        path.chmod(0o755)

    def archive(self, platform, arch, member="rgd"):
        path = self.releases / f"rgd_{TAG}_{platform}_{arch}.tar.gz"
        with tarfile.open(path, "w:gz") as archive:
            info = tarfile.TarInfo(member)
            info.size = len(BINARY)
            info.mode = 0o755
            archive.addfile(info, io.BytesIO(BINARY))

    def checksums(self):
        lines = []
        for path in sorted(self.releases.glob("*.tar.gz")):
            lines.append(f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}\n")
        (self.releases / "SHA256SUMS").write_text("".join(lines))

    def run_install(self, *args, success=True):
        result = subprocess.run(
            ["sh", str(INSTALLER), *args], env=self.env, text=True, capture_output=True
        )
        if success:
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        else:
            self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        return result

    @property
    def installed(self):
        return self.home / ".local/bin/rgd"

    def test_latest_and_platform_mapping(self):
        for os_name, machine, platform, arch in (
            ("Darwin", "arm64", "darwin", "arm64"),
            ("Darwin", "x86_64", "darwin", "amd64"),
            ("Linux", "aarch64", "linux", "arm64"),
            ("Linux", "x86_64", "linux", "amd64"),
        ):
            with self.subTest(platform=platform, arch=arch):
                self.env.update(MOCK_OS=os_name, MOCK_ARCH=machine)
                self.run_install()
                self.assertEqual(self.installed.read_bytes(), BINARY)
                self.assertEqual(self.installed.stat().st_mode & 0o777, 0o755)
                self.assertIn(f"rgd_{TAG}_{platform}_{arch}.tar.gz", self.log.read_text())
                self.assertEqual(list(self.installed.parent.glob(".rgd.*")), [])
        self.assertFalse((self.home / ".claude").exists())
        self.assertFalse((self.home / ".agents").exists())

    def test_pinned_version_and_custom_path(self):
        destination = self.root / "custom bin"
        self.run_install("--version", TAG, "--bin-dir", str(destination))
        self.assertEqual((destination / "rgd").read_bytes(), BINARY)
        self.assertNotIn("/latest", self.log.read_text())

    def test_checksum_mismatch_keeps_existing_install(self):
        self.installed.parent.mkdir(parents=True)
        self.installed.write_text("keep existing executable")
        (self.releases / f"rgd_{TAG}_darwin_arm64.tar.gz").write_bytes(b"tampered")
        result = self.run_install(success=False)
        self.assertIn("Checksum mismatch", result.stderr)
        self.assertEqual(self.installed.read_text(), "keep existing executable")

    def test_missing_checksum(self):
        (self.releases / "SHA256SUMS").write_text("")
        self.run_install(success=False)
        self.assertFalse(self.installed.exists())

    def test_missing_binary_in_valid_archive(self):
        self.archive("darwin", "arm64", member="not-rgd")
        self.checksums()
        self.run_install(success=False)
        self.assertFalse(self.installed.exists())

    def test_symlink_destination_is_not_followed(self):
        target = self.root / "user-file"
        target.write_text("leave alone")
        self.installed.parent.mkdir(parents=True)
        self.installed.symlink_to(target)
        self.run_install(success=False)
        self.assertTrue(self.installed.is_symlink())
        self.assertEqual(target.read_text(), "leave alone")

    def test_unsupported_platform_and_invalid_options(self):
        self.env["MOCK_OS"] = "Windows"
        self.run_install(success=False)
        self.assertFalse(self.log.exists())
        self.env["MOCK_OS"] = "Darwin"
        for args in (("--version", "../../bad"), ("--bin-dir", "relative"), ("--version",), ("--unknown",)):
            with self.subTest(args=args):
                self.run_install(*args, success=False)
                self.assertFalse(self.installed.exists())


if __name__ == "__main__":
    unittest.main()
