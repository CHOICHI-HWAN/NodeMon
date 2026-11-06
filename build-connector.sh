#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
# Cross-compile the connector for every supported OS into ./dist
# (scripts/build-connector.mjs does the same and also writes SHA256SUMS; keep the target lists in sync)
set -e
cd "$(dirname "$0")/connector"
mkdir -p ../dist
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo dev)
for t in linux/amd64 linux/arm64 linux/arm windows/amd64 darwin/arm64 darwin/amd64; do
  os=${t%/*}; arch=${t#*/}; ext=""; [ "$os" = windows ] && ext=".exe"
  GOOS=$os GOARCH=$arch GOARM=7 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "../dist/nodemon-connector-$os-$arch$ext" .
  echo "built dist/nodemon-connector-$os-$arch$ext"
done
