#!/usr/bin/env bash
# Packages the binaries built by scripts/build.sh for a release: the macOS and
# Linux ones also go in zips holding a ready-to-use tordo/ folder, because a
# zip keeps the executable bit that a bare download loses (so no chmod is
# needed). Windows stays a bare .exe: running it from inside a zip would put
# the catalog in a temporary folder. The bare macOS binary is dropped so the
# release offers one obvious Mac download; the bare Linux ones stay for
# servers. Usage: scripts/package.sh [dir] (default dist/tordo).
set -euo pipefail
out=${1:-dist/tordo}
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT
for bin in tordo-macos tordo-linux-amd64 tordo-linux-arm64; do
  rm -rf "$stage/tordo"
  mkdir "$stage/tordo"
  cp "$out/$bin" "$stage/tordo/tordo"
  chmod 755 "$stage/tordo/tordo"
  rm -f "$out/$bin.zip"
  (cd "$stage" && zip -qrX - tordo) > "$out/$bin.zip"
done
rm "$out/tordo-macos"
