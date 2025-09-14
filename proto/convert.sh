#!/usr/bin/env bash
set -e

echo ">> Mencari semua file .proto dan generate .pb.go"

find . -name "*.proto" -print0 | \
  xargs -0 protoc \
    --go_out=paths=source_relative:. \
    --go-grpc_out=paths=source_relative:.

echo "✅ Selesai generate semua .proto ke .pb.go"
