#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)
PLUGIN_DIR="$ROOT_DIR/examples/plugin-demo"
BUILD_DIR="$ROOT_DIR/.run/plugin-demo-build"
PACKAGE_DIR="$ROOT_DIR/plugin-market/packages"
PLUGIN_VERSION=$(sed -n 's/.*"version":[[:space:]]*"\([^"]*\)".*/\1/p' "$PLUGIN_DIR/plugin.json" | head -n 1)

if [[ -z "$PLUGIN_VERSION" ]]; then
  echo "未能从 plugin.json 解析插件版本" >&2
  exit 1
fi

PACKAGE_NAME="demo-feature-${PLUGIN_VERSION}.zip"

rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR" "$PACKAGE_DIR"

cp "$PLUGIN_DIR/plugin.json" "$BUILD_DIR/plugin.json"
cp "$PLUGIN_DIR/README.md" "$BUILD_DIR/README.md"
if [[ -d "$PLUGIN_DIR/assets" ]]; then
  mkdir -p "$BUILD_DIR/assets"
  cp -R "$PLUGIN_DIR/assets/." "$BUILD_DIR/assets/"
fi
if [[ -d "$PLUGIN_DIR/migrations" ]]; then
  mkdir -p "$BUILD_DIR/migrations"
  cp -R "$PLUGIN_DIR/migrations/." "$BUILD_DIR/migrations/"
fi

pushd "$ROOT_DIR" >/dev/null
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -tags pluginexample -buildmode=plugin -o "$BUILD_DIR/plugin.so" ./examples/plugin-demo
popd >/dev/null

pushd "$BUILD_DIR" >/dev/null
zip -rq "$PACKAGE_DIR/$PACKAGE_NAME" .
popd >/dev/null

echo "已生成插件包: $PACKAGE_DIR/$PACKAGE_NAME"
