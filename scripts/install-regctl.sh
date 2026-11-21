#!/usr/bin/env bash
# Installs a pinned, checksum-verified regctl into $RUNNER_TEMP/bin.
set -euo pipefail

version=v0.11.7
case "$(uname -m)" in
  x86_64) arch=amd64 sum=3737fbbdd791e04e7a831640b681f3397a376498e80e2ad9b1b2d27296f239bc ;;
  aarch64 | arm64) arch=arm64 sum=9f349bab5c407ac04f3dabf99c7c7a6b0f9e9723c23f2775380e804d02c6dbb9 ;;
  *) echo "unsupported arch $(uname -m)" >&2; exit 1 ;;
esac

bin="${RUNNER_TEMP:-/tmp}/bin"
mkdir -p "$bin"
curl -fsSL -o "$bin/regctl" "https://github.com/regclient/regclient/releases/download/${version}/regctl-linux-${arch}"
echo "${sum}  $bin/regctl" | sha256sum -c --quiet -
chmod +x "$bin/regctl"
echo "$bin" >> "${GITHUB_PATH:-/dev/null}"
