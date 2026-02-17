#!/bin/bash

TARGET=$1

if [ "$TARGET" == "wasm" ]; then
    echo "🚀 Starting optimized WASM build process..."
    
    OUTPUT_WASM="main.wasm"
    DEPLOY_DIR="deploy/wasm"
    
    # 1. Clean up old artifacts
    rm -f $OUTPUT_WASM main.wasm.gz
    
    # 2. Build with standard Go compiler
    echo "📦 Compiling WASM package..."
    GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o $OUTPUT_WASM ./cmd/wasm
    
    if [ $? -ne 0 ]; then
        echo "❌ Build failed!"
        exit 1
    fi
    
    # 3. Generate Gzip for compatibility
    echo "📦 Compression: Gzipping $OUTPUT_WASM..."
    gzip -c -9 $OUTPUT_WASM > main.wasm.gz
    
    # 4. Restore standard Go wasm_exec.js
    echo "📄 Updating wasm_exec.js..."
    cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" .
    
    # 5. Prepare deploy/wasm folder
    mkdir -p $DEPLOY_DIR
    cp index.html $DEPLOY_DIR/
    cp wasm_exec.js $DEPLOY_DIR/
    cp main.wasm.gz $DEPLOY_DIR/
    
    # 6. Cleanup
    rm -f $OUTPUT_WASM main.wasm.gz wasm_exec.js
    
    echo "✅ Deployment assets ready in: $DEPLOY_DIR/"

elif [ "$TARGET" == "pc" ]; then
    echo "🚀 Building PC (Terminal) binary..."
    DEPLOY_DIR="deploy/cli"
    mkdir -p $DEPLOY_DIR
    go build -o $DEPLOY_DIR/whatsapp_cli ./cmd/cli
    if [ $? -eq 0 ]; then
        echo "✅ Terminal binary ready: $DEPLOY_DIR/whatsapp_cli"
    else
        echo "❌ PC build failed!"
        exit 1
    fi

else
    echo "Usage: ./build.sh [wasm|pc]"
    exit 1
fi
