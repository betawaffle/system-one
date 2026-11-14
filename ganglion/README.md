# ganglion

The workbench for the system-one project: browse the event corpus, play with Ettin, and later watch and steer training. Design: `../design/07-workbench.md`, `08-workbench-design.md`, `09-look.md`.

- Go server (`main.go`, `internal/`): pages, static files, store, websocket. Port 7171.
- Python worker (`py/worker.py`): Ettin on CPU over a Unix socket. Logs every request and reply to `<data>/worker-log/`.
- Importers (`py/import_*.py`): turn sources into NDJSON events for `ganglion import`.

Data lives in `~/.local/share/ganglion` (index.db, blobs.db, blobs/, index/, worker-log/, raw/).

## Run

```
cd py && uv sync --python 3.13            # once: CPU torch, transformers, pyarrow
go build -o ganglion .
py/.venv/bin/python -I py/worker.py --socket ~/.local/share/ganglion/worker.sock --data ~/.local/share/ganglion
./ganglion                                # http://localhost:7171
```

The server and worker are independent: either can restart, and the server reconnects.

## Import

```
py/.venv/bin/python -I py/import_openswe.py SHARD.parquet openswe/minisweagent/qwen38-flash-next/rebench-v2 | ./ganglion import
py/.venv/bin/python -I py/import_claude.py ~/.claude/projects/<project-dir> | ./ganglion import
```

Events are content-addressed (SHA-256 of canonical JSON), so re-importing is harmless. The held-out split is by task (1 in 10 `split_key`s), and local transcripts form their own `local` split.

## Hostnames

Requests for unknown hostnames are refused (DNS rebinding). IP addresses and localhost always work; add other names with `-hosts` or `GANGLION_HOSTS` (comma-separated). The service reads `~/.config/ganglion/env`, e.g.:

```
GANGLION_HOSTS=myhost,myhost.example.ts.net
```

## Services

`deploy/install.sh` builds to `~/.local/bin/ganglion`, installs the two systemd user units from `deploy/`, and restarts them.

## Licence

MIT, see `../LICENSE`. The bundled IBM Plex fonts are under the SIL Open Font License (`static/fonts/LICENSE-OFL.txt`).
