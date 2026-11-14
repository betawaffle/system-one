#!/bin/bash
# Build ganglion, install the binary and user units, and (re)start both services.
set -euo pipefail
here=$(cd "$(dirname "$0")/.." && pwd)
cd "$here"
go build -o "$HOME/.local/bin/ganglion.new" .
mv "$HOME/.local/bin/ganglion.new" "$HOME/.local/bin/ganglion"
mkdir -p "$HOME/.config/systemd/user"
cp deploy/ganglion.service deploy/ganglion-worker.service "$HOME/.config/systemd/user/"
systemctl --user daemon-reload
systemctl --user enable ganglion-worker.service ganglion.service
systemctl --user restart ganglion-worker.service ganglion.service
