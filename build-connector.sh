#!/bin/sh
# Cross-compile the connector for every supported OS into ./dist
set -e
cd "$(dirname "$0")/connector"
mkdir -p ../dist
for t in linux/amd64 linux/arm64 windows/amd64 darwin/arm64 darwin/amd64; do
  os=${t%/*}; arch=${t#*/}; ext=""; [ "$os" = windows ] && ext=".exe"
  GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -o "../dist/nodemon-connector-$os-$arch$ext" .
  echo "built dist/nodemon-connector-$os-$arch$ext"
done
