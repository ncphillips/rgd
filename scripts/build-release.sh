#!/usr/bin/env bash
set -euo pipefail

version=${1:-}
if [[ $# -gt 2 || ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  printf 'Usage: bash scripts/build-release.sh vMAJOR.MINOR.PATCH[-PRERELEASE] [OUTPUT_DIR]\n' >&2
  exit 1
fi
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
output=${2:-"$root/dist"}
mkdir -p "$output"
output=$(cd "$output" && pwd)
if [[ -e "$output/SHA256SUMS" ]] || compgen -G "$output/rgd_*.tar.gz" >/dev/null; then
  printf 'Use a fresh output directory: %s\n' "$output" >&2
  exit 1
fi
work=$(mktemp -d "${TMPDIR:-/tmp}/rgd-release.XXXXXX")
trap 'rm -rf "$work"' EXIT

cd "$root"
for os in darwin linux; do
  for arch in amd64 arm64; do
    package="$work/$os-$arch"
    mkdir "$package"
    printf 'Building %s/%s\n' "$os" "$arch"
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build \
      -trimpath -buildvcs=false -ldflags='-s -w' -o "$package/rgd" ./cmd/rgd
    cp README.md "$package/README.md"
    printf '%s\n' "$version" > "$package/VERSION"
    archive="rgd_${version}_${os}_${arch}.tar.gz"
    COPYFILE_DISABLE=1 tar -czf "$output/$archive" -C "$package" rgd README.md VERSION
  done
done

cd "$output"
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum rgd_*.tar.gz > SHA256SUMS
else
  shasum -a 256 rgd_*.tar.gz > SHA256SUMS
fi
printf 'Release artifacts: %s\n' "$output"
