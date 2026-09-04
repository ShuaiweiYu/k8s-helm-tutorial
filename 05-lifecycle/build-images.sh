#!/usr/bin/env bash
# Builds the three image tags used in this section's demo
# (TUTORIAL-OUTLINE.md · 47-57 min, 附录 B 第 7 条) from the one app source
# tree, and preloads them into the local `kind` cluster if one named
# "demo" is running — the live demo must never pull from the network.
set -euo pipefail
cd "$(dirname "$0")/../app"

docker build -t hello:v1 --build-arg VERSION=v1 .
docker build -t hello:v2 --build-arg VERSION=v2 .
docker build -t hello:v3-broken --build-arg VERSION=v3-broken --build-arg CRASH_ON_START=true .

if kind get clusters 2>/dev/null | grep -qx demo; then
  kind load docker-image hello:v1 hello:v2 hello:v3-broken --name demo
else
  echo "no 'demo' kind cluster running — built the images, skipped kind load" >&2
fi
