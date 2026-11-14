#!/bin/bash
# Screenshot ganglion pages in light and dark, desktop and phone width, and report
# console errors and horizontal overflow. Needs the running service on :7171.
#
# Tooling lives outside the repo in ~/.local/share/ganglion-tools:
#   pw/    Python venv with playwright (Chromium itself is in ~/.cache/ms-playwright)
#   libs/  X11/ATK/GBM/ALSA libraries Chromium needs that the box doesn't have,
#          unpacked from Fedora RPMs (dnf download + rpm2cpio), not installed system-wide.
#
# Usage: test/browser/run.sh OUTDIR [/page,/page@waitms,...]
#        test/browser/run.sh OUTDIR ettin     # Ettin modes, side by side
set -euo pipefail
tools=${GANGLION_TOOLS:-$HOME/.local/share/ganglion-tools}
here=$(cd "$(dirname "$0")" && pwd)
out=${1:?output directory}
mkdir -p "$out"
export LD_LIBRARY_PATH="$tools/libs${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
if [ "${2:-}" = ettin ]; then
	exec "$tools/pw/bin/python" -I "$here/ettin_shots.py" "$out"
fi
exec "$tools/pw/bin/python" -I "$here/shots.py" "$out" "${2:-/,/events,/fields,/ettin@1500,/design}"
