#!/usr/bin/env bash
# Cross-compiles Tordo for Windows, macOS (universal) and Linux (amd64,
# arm64) into dist/tordo/.
set -euo pipefail
cd "$(dirname "$0")/.."
out=dist/tordo
mkdir -p "$out"
export CGO_ENABLED=0
build() { GOOS=$1 GOARCH=$2 go build -trimpath -ldflags "-s -w" -o "$out/$3" ./cmd/tordo; }
build windows amd64 tordo-windows.exe
build linux   amd64 tordo-linux-amd64
build linux   arm64 tordo-linux-arm64
build darwin  amd64 tordo-macos-amd64
build darwin  arm64 tordo-macos-arm64
go run github.com/randall77/makefat@latest "$out/tordo-macos" "$out/tordo-macos-amd64" "$out/tordo-macos-arm64"
rm "$out/tordo-macos-amd64" "$out/tordo-macos-arm64"
ls -la "$out"
