#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")/.."
npm ci
npm run check
npm run build
CGO_ENABLED=0 go test ./...
mkdir -p bin
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/wgsimple ./cmd/wgsimple
