#!/bin/bash
cd $(dirname $0)
set -euo pipefail
if [[ ! -f "e2ee.js" ]]; then
	echo "Please download the encryption javascript file and save it to e2ee.js first"
	exit 1
fi
node parse-proto.js
protoc --go_out=. --go_opt=paths=source_relative --go_opt=embed_raw=true */*.proto
