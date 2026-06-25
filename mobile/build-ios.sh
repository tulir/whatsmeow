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
# Device-only target (ios/arm64): we develop and ship on physical devices via
# `bun run ios:device`, so the arm64 simulator slice is pure dead weight that
# also bloats the committed xcframework (~25MB). If you ever need to run the app
# in the iOS Simulator, append `,iossimulator/arm64` to -target and rebuild.
"$GOBIN/gomobile" bind -target=ios/arm64 -ldflags="-s -w" -o "$OUT" ./mobile/wa
echo "Built: $OUT"

# Install into the app module. Replace the destination outright: `cp -R` onto an
# existing .xcframework directory nests the new framework inside it (a classic cp
# gotcha), which leaves the app linked against the stale binary. Guarded so the
# build still works if the app checkout isn't a sibling.
APP_FRAMEWORK="$ROOT/../auraRN/modules/whatsapp/ios/Wa.xcframework"
if [ -d "$(dirname "$APP_FRAMEWORK")" ]; then
	rm -rf "$APP_FRAMEWORK"
	cp -R "$OUT" "$APP_FRAMEWORK"
	echo "Installed: $APP_FRAMEWORK"
fi
