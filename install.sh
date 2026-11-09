#!/bin/sh
# Download a published rgd release. No Go, sudo, or agent configuration changes.
set -eu

main() {
  version=${RGD_VERSION:-latest}
  install_dir=${RGD_INSTALL_DIR:-${HOME:+$HOME/.local/bin}}
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --version|--bin-dir)
        [ "$#" -ge 2 ] || die "$1 requires a value"
        case "$1" in
          --version) version=$2 ;;
          --bin-dir) install_dir=$2 ;;
        esac
        shift 2 ;;
      -h|--help)
        printf 'Usage: sh install.sh [--version vX.Y.Z] [--bin-dir /absolute/path]\n'
        printf 'Defaults: latest stable release, ~/.local/bin\n'
        return ;;
      *) die "Unknown option: $1" ;;
    esac
  done
  case "$install_dir" in /*) ;; *) die '--bin-dir must be an absolute path' ;; esac
  for tool in curl tar awk mktemp; do
    command -v "$tool" >/dev/null 2>&1 || die "Required command not found: $tool"
  done
  if command -v sha256sum >/dev/null 2>&1; then
    checksum_tool=sha256sum
  elif command -v shasum >/dev/null 2>&1; then
    checksum_tool=shasum
  else
    die 'A SHA-256 tool is required (sha256sum or shasum)'
  fi
  case "$(uname -s)" in
    Darwin) os=darwin ;;
    Linux) os=linux ;;
    *) die 'Supported systems: macOS and Linux' ;;
  esac
  case "$(uname -m)" in
    x86_64|amd64) arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *) die 'Supported architectures: amd64 and arm64' ;;
  esac

  base=https://github.com/ncphillips/rgd/releases
  if [ "$version" = latest ]; then
    resolved=$(curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 \
      --output /dev/null --write-out '%{url_effective}' "$base/latest") || die 'Could not resolve the latest release; has a release been published?'
    case "$resolved" in
      "$base/tag/"*) version=${resolved#"$base/tag/"} ;;
      *) die 'Unexpected latest-release redirect' ;;
    esac
  fi
  case "$version" in
    v[0-9]*.[0-9]*.[0-9]*) ;;
    *) die 'Version must be a release tag such as v0.1.0' ;;
  esac
  case "$version" in *[!a-zA-Z0-9._-]*) die 'Invalid release tag' ;; esac

  work=$(mktemp -d "${TMPDIR:-/tmp}/rgd-install.XXXXXX")
  staged=
  trap 'rm -rf "$work"; if [ -n "$staged" ]; then rm -f "$staged"; fi' 0
  trap 'exit 1' HUP INT TERM
  asset="rgd_${version}_${os}_${arch}.tar.gz"
  printf 'Downloading rgd %s for %s/%s\n' "$version" "$os" "$arch"
  for file in "$asset" SHA256SUMS; do
    curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 \
      "$base/download/$version/$file" --output "$work/$file" || die "Could not download $file"
  done
  expected=$(awk -v asset="$asset" '$2 == asset {print $1}' "$work/SHA256SUMS")
  [ "${#expected}" -eq 64 ] || die 'Missing or invalid archive checksum'
  case "$expected" in *[!0-9a-fA-F]*) die 'Invalid archive checksum' ;; esac
  if [ "$checksum_tool" = sha256sum ]; then
    actual=$(sha256sum "$work/$asset" | awk '{print $1}')
  else
    actual=$(shasum -a 256 "$work/$asset" | awk '{print $1}')
  fi
  [ "$actual" = "$expected" ] || die 'Checksum mismatch; nothing was installed'

  # Stream only the binary, rather than trusting archive paths or symlinks.
  tar -xzOf "$work/$asset" rgd > "$work/rgd" || die 'Could not unpack rgd'
  [ -s "$work/rgd" ] || die 'Release archive contains an empty binary'
  mkdir -p "$install_dir"
  [ ! -L "$install_dir/rgd" ] || die 'Refusing to replace a symlink at the installation path'
  if [ -e "$install_dir/rgd" ] && [ ! -f "$install_dir/rgd" ]; then
    die 'Installation path exists and is not a regular file'
  fi
  staged=$(mktemp "$install_dir/.rgd.XXXXXX")
  cp "$work/rgd" "$staged"
  chmod 755 "$staged"
  mv -f "$staged" "$install_dir/rgd"
  staged=
  printf 'Installed rgd %s to %s/rgd\n' "$version" "$install_dir"
  case ":${PATH:-}:" in
    *":$install_dir:"*) ;;
    *) printf 'Add this directory to your PATH: %s\n' "$install_dir" ;;
  esac
  if ! command -v jj >/dev/null 2>&1; then
    printf 'Install Jujutsu (jj) before reviewing a repository.\n'
  fi
  printf 'Optional: run rgd skill setup in a terminal to configure your coding agents.\n'
}

die() {
  printf 'rgd install: %s\n' "$*" >&2
  exit 1
}

# Read all definitions before executing when this script is piped into sh.
main "$@"
