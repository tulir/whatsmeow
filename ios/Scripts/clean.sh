#!/bin/bash

# Clean script for WhatsApp iOS project

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IOS_DIR="$(dirname "$SCRIPT_DIR")"
FRAMEWORKS_DIR="$IOS_DIR/Frameworks"

echo "Cleaning iOS project..."

# Remove frameworks
if [ -d "$FRAMEWORKS_DIR" ]; then
    echo "Removing frameworks..."
    rm -rf "$FRAMEWORKS_DIR"/*
fi

# Clean Xcode derived data for this project
PROJECT_NAME="WhatsApp"
DERIVED_DATA="$HOME/Library/Developer/Xcode/DerivedData"

if [ -d "$DERIVED_DATA" ]; then
    echo "Cleaning Xcode derived data..."
    find "$DERIVED_DATA" -name "${PROJECT_NAME}*" -type d -exec rm -rf {} + 2>/dev/null || true
fi

echo "Clean complete!"
