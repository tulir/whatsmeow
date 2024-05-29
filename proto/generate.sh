#!/bin/bash
cd $(dirname $0)
set -euo pipefail
if [[ ! -d "js" ]]; then
	echo "Please download the WhatsApp JavaScript files into the js directory first"
	exit 1
fi
node parse-proto.js
protoc --go_out=. --go_opt=paths=source_relative --go_opt=embed_raw=true */*.proto
