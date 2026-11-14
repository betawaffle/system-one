#!/bin/bash
# run.sh [command...]  Like gpu-run, but with the ganglion-gpu image: rootless, as the
# calling user, inside experiments.slice, with the GPU. Mounts:
#   /data                        (HF_HOME=/data/hf, shared model cache)
#   ganglion repo   -> /work     (working directory)
#   ~/.local/share/ganglion -> /store   (event store, indexes, runs)
# IMAGE overrides the image (default: localhost/ganglion-gpu:latest).
set -euo pipefail
here=$(cd "$(dirname "$0")/.." && pwd)
image=${IMAGE:-localhost/ganglion-gpu:latest}
user=${SUDO_USER:-$(id -un)}
uid=$(id -u "$user")
home=$(getent passwd "$user" | cut -d: -f6)
tty=()
[ -t 0 ] && [ -t 1 ] && tty=(-it)
[ $# -gt 0 ] || set -- bash
exec sudo systemd-run --scope --quiet --slice=experiments.slice \
  --uid="$user" --gid="$(id -g "$user")" \
  --setenv=XDG_RUNTIME_DIR=/run/user/$uid --setenv=HOME="$home" -- \
  podman run --rm "${tty[@]}" --cgroups=disabled \
    --device /dev/kfd --device /dev/dri --group-add keep-groups \
    --security-opt label=disable --ipc=host \
    -v /data:/data -v "$here":/work -v "$home/.local/share/ganglion":/store -w /work \
    -e GANGLION_IMAGE="$image" \
    "$image" "$@"
