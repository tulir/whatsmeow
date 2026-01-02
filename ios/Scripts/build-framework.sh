#!/bin/bash

# Build script for WhatsApp iOS framework
# This script uses gomobile to build the Go mobile bindings for iOS

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== WhatsApp iOS Framework Build Script ===${NC}"

# Get the script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IOS_DIR="$(dirname "$SCRIPT_DIR")"
ROOT_DIR="$(dirname "$IOS_DIR")"
FRAMEWORKS_DIR="$IOS_DIR/Frameworks"

# Check for Go installation
if ! command -v go &> /dev/null; then
    echo -e "${RED}Error: Go is not installed${NC}"
    echo "Please install Go: https://golang.org/dl/"
    exit 1
fi

echo -e "${GREEN}Go version: $(go version)${NC}"

# Check for gomobile
if ! command -v gomobile &> /dev/null; then
    echo -e "${YELLOW}Installing gomobile...${NC}"
    go install golang.org/x/mobile/cmd/gomobile@latest
    gomobile init
fi

echo -e "${GREEN}gomobile installed${NC}"

# Create frameworks directory
mkdir -p "$FRAMEWORKS_DIR"

# Change to root directory
cd "$ROOT_DIR"

echo -e "${GREEN}Building iOS framework...${NC}"
echo "This may take a few minutes..."

# Build the framework
# -target ios builds for iOS (both device and simulator)
# -o specifies the output location
gomobile bind \
    -target ios \
    -o "$FRAMEWORKS_DIR/Mobile.xcframework" \
    -ldflags="-s -w" \
    go.mau.fi/whatsmeow/mobile

if [ $? -eq 0 ]; then
    echo -e "${GREEN}=== Build Successful ===${NC}"
    echo -e "Framework location: ${YELLOW}$FRAMEWORKS_DIR/Mobile.xcframework${NC}"
    echo ""
    echo "Next steps:"
    echo "1. Open the Xcode project: open ios/WhatsApp/WhatsApp.xcodeproj"
    echo "2. The framework should be automatically linked"
    echo "3. Build and run on your device or simulator"
else
    echo -e "${RED}Build failed${NC}"
    exit 1
fi
