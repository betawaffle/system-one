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

## Grammar

The synthetic-data grammar lives in `<data>/grammar` (design/synthetic-data/07, 09): content blobs plus append-only `versions.jsonl` and `events.jsonl`. `ganglion lexicon import` builds the word list from the raw WordNet and CMUdict downloads. To retire a template or other object, keeping its history and logging why:

```
ganglion grammar retire template:give-ee2862db "why it's retired" [-by NAME]
systemctl --user restart ganglion.service   # the server reads the grammar at start
```

## Backups

A plain file copy of a live SQLite database (and its `-wal` file) can be inconsistent, so don't back up or restore `index.db` and `blobs.db` directly. `ganglion snapshot-db` writes consistent copies (`VACUUM INTO`, safe while the server runs) to `<data>/snapshot/`. Restore from those. `blobs.db` can't be rebuilt, since it holds the small blobs themselves.

The backup should run the snapshot as its own first step, so the copy always happens right before the backup, whenever that is. With a systemd backup service, a drop-in like this does it (adjust user and path):

```
# /etc/systemd/system/<backup>.service.d/ganglion-snapshot.conf
[Service]
ExecStartPre=-/usr/sbin/runuser -u USER -- /home/USER/.local/bin/ganglion snapshot-db
```

Then exclude the live `index.db*` and `blobs.db*` from the backup. If the snapshot ever fails, the previous one stays in place: older, but consistent.

Rule for new data (design/synthetic-data/07, B1d): the source of truth is append-only files (JSONL records, blob files). SQLite is only an index that can be rebuilt from them, so nothing new depends on the backup getting SQLite right.

## Licence

MIT, see `../LICENSE`. The bundled IBM Plex fonts are under the SIL Open Font License (`static/fonts/LICENSE-OFL.txt`).
