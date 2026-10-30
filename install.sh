#!/usr/bin/env bash
# Installs claude-gauges: downloads the release for this system, checks its checksum, then runs
# `claude-gauges install`, which copies the program to ~/.claude and sets the Claude Code status line.
#
#   curl -fsSL https://raw.githubusercontent.com/mountaser/claude-gauges/main/install.sh | bash
#
# Extra flags go to `claude-gauges install`, for example:  ... | bash -s -- --force
# Set CLAUDE_GAUGES_VERSION (for example v0.2.1) to install a specific release instead of the latest.
set -euo pipefail

REPO="mountaser/claude-gauges"

fail() {
  printf 'claude-gauges install: %s\n' "$*" >&2
  exit 1
}

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) fail "unsupported system $(uname -s). On Windows, use PowerShell: irm https://raw.githubusercontent.com/$REPO/main/install.ps1 | iex" ;;
esac

case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) fail "unsupported CPU $(uname -m)" ;;
esac

command -v curl >/dev/null 2>&1 || fail "curl is required"
if command -v sha256sum >/dev/null 2>&1; then
  checksum() { sha256sum "$1" | cut -d' ' -f1; }
elif command -v shasum >/dev/null 2>&1; then
  checksum() { shasum -a 256 "$1" | cut -d' ' -f1; }
else
  fail "sha256sum or shasum is required"
fi

asset="claude-gauges_${os}_${arch}"
version="${CLAUDE_GAUGES_VERSION:-latest}"
if [ "$version" = "latest" ]; then
  base="https://github.com/$REPO/releases/latest/download"
else
  base="https://github.com/$REPO/releases/download/$version"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "Downloading $asset ($version)..."
curl -fsSL "$base/$asset" -o "$tmp/claude-gauges" || fail "download failed: $base/$asset"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt" || fail "could not download $base/checksums.txt"

want="$(awk -v f="$asset" '$2 == f { print $1 }' "$tmp/checksums.txt")"
[ -n "$want" ] || fail "no checksum is listed for $asset"
got="$(checksum "$tmp/claude-gauges")"
[ "$want" = "$got" ] || fail "checksum mismatch for $asset (expected $want, got $got)"

chmod +x "$tmp/claude-gauges"
"$tmp/claude-gauges" install "$@"
