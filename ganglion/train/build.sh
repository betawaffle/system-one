#!/bin/bash
# Build localhost/ganglion-gpu, tagged with today's date and "latest".
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
tag=$(date -u +%Y%m%d)
podman build -t "localhost/ganglion-gpu:$tag" -t localhost/ganglion-gpu:latest -f "$here/Containerfile" "$here"
podman image inspect "localhost/ganglion-gpu:$tag" --format '{{.Id}}'
