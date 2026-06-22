#!/usr/bin/env bash
# Build a minimal iOS xcframework from the gomobile wrapper package.
#
# Prereqs (one-time):
#   go install golang.org/x/mobile/cmd/gomobile@latest
#   go install golang.org/x/mobile/cmd/gobind@latest
#   gomobile init
#
# Usage: ./mobile/build-ios.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)" # whatsmeow repo root (has go.mod)
GOBIN="$(go env GOPATH)/bin"
OUT="$ROOT/mobile/build/Wa.xcframework"

# gomobile shells out to gobind; make sure both are reachable.
export PATH="$GOBIN:$PATH"

cd "$ROOT"
mkdir -p "$ROOT/mobile/build"

# Ensure bind support + the pure-Go sqlite driver are resolvable in this module.
go get golang.org/x/mobile/bind >/dev/null 2>&1 || true
go get modernc.org/sqlite >/dev/null 2>&1 || true

# -ldflags "-s -w" strips the symbol table + DWARF (much smaller binary).
"$GOBIN/gomobile" bind -target=ios -ldflags="-s -w" -o "$OUT" ./mobile/wa
echo "Built: $OUT"
