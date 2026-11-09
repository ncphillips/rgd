#!/usr/bin/env bash
# Pin the integration-test dependency and verify upstream release bytes.
set -euo pipefail

version=v0.43.0
case "$(uname -s)-$(uname -m)" in
  Linux-x86_64)
    target=x86_64-unknown-linux-musl
    checksum=59e5588583ac82b623239929368c65b90735931c0f26b5a16c1f04d5bb97643d ;;
  Linux-aarch64|Linux-arm64)
    target=aarch64-unknown-linux-musl
    checksum=289197b6bec60b4e57d47260624b617716f737eb02cdfd9155791b2576aa5862 ;;
  Darwin-arm64)
    target=aarch64-apple-darwin
    checksum=84336bbe5673a36ccc6395c494021ba632794da078eb8c8c513a60f8e1cc3083 ;;
  Darwin-x86_64)
    target=x86_64-apple-darwin
    checksum=f1a7fec046b816132318c07a9c096680f7aae78b008709c7166a57efd9c579ec ;;
  *) printf 'Unsupported CI platform\n' >&2; exit 1 ;;
esac

work=$(mktemp -d "${TMPDIR:-/tmp}/rgd-jj.XXXXXX")
trap 'rm -rf "$work"' EXIT
curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 \
  "https://github.com/jj-vcs/jj/releases/download/$version/jj-$version-$target.tar.gz" \
  --output "$work/jj.tar.gz"
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$work/jj.tar.gz" | awk '{print $1}')
else
  actual=$(shasum -a 256 "$work/jj.tar.gz" | awk '{print $1}')
fi
[[ "$actual" == "$checksum" ]] || { printf 'Jujutsu checksum mismatch\n' >&2; exit 1; }
mkdir "$work/unpacked"
tar -xzf "$work/jj.tar.gz" -C "$work/unpacked"
binary=$(find "$work/unpacked" -type f -name jj -print -quit)
[[ -n "$binary" ]] || { printf 'Jujutsu archive has no jj binary\n' >&2; exit 1; }
bin="${RUNNER_TEMP:-$work}/rgd-ci-bin"
mkdir -p "$bin"
install -m 755 "$binary" "$bin/jj"
printf '%s\n' "$bin" >> "${GITHUB_PATH:?This script runs inside GitHub Actions}"
"$bin/jj" --version
