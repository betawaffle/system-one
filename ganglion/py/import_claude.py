"""Convert local Claude Code transcripts to ganglion NDJSON events.

Usage: python import_claude.py PROJECT_DIR [PROJECT_DIR ...] | ganglion import

Each Bash tool call with its result becomes a command_execution event. The
same tool call can appear in several transcript files (resumed sessions), so
calls are deduplicated by tool_use id here; identical events also collapse by
content hash in the store.
"""
import glob
import json
import os
import re
import sys

EXIT = re.compile(r"^Exit code (-?\d+)")


def text_of(content):
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        return "".join(b.get("text", "") for b in content if isinstance(b, dict))
    return ""


def main():
    seen = set()
    order = {}  # per-session call counter, so each call is its own sighting
    out = sys.stdout
    for proj in sys.argv[1:]:
        name = os.path.basename(proj.rstrip("/"))
        for path in sorted(glob.glob(os.path.join(proj, "**", "*.jsonl"), recursive=True)):
            calls = {}
            for line in open(path, errors="replace"):
                try:
                    d = json.loads(line)
                except json.JSONDecodeError:
                    continue
                msg = d.get("message") or {}
                content = msg.get("content")
                if not isinstance(content, list):
                    continue
                for b in content:
                    if not isinstance(b, dict):
                        continue
                    if b.get("type") == "tool_use" and b.get("name") == "Bash":
                        calls[b.get("id")] = (b.get("input") or {}, d.get("timestamp"), d.get("cwd"), d.get("sessionId"))
                    elif b.get("type") == "tool_result" and b.get("tool_use_id") in calls:
                        tid = b["tool_use_id"]
                        if tid in seen:
                            continue
                        seen.add(tid)
                        inp, started, cwd, session = calls[tid]
                        command = inp.get("command")
                        if not command:
                            continue
                        tur = d.get("toolUseResult") if isinstance(d.get("toolUseResult"), dict) else {}
                        text = text_of(b.get("content"))
                        interrupted = bool(tur.get("interrupted"))
                        if b.get("is_error"):
                            m = EXIT.match(text)
                            code = int(m.group(1)) if m else None
                        else:
                            code = None if interrupted else 0
                        has_split = "stdout" in tur
                        ev = {
                            "type": "command_execution",
                            "command": command,
                            "cwd": cwd,
                            "exit_code": code,
                            "output": None if has_split else text,
                            "stdout": tur.get("stdout") if has_split else None,
                            "stderr": tur.get("stderr") if has_split else None,
                            "truncated": False,
                            "error": "interrupted" if interrupted else None,
                            "duration_ms": None,
                        }
                        unit = session or path
                        order[unit] = order.get(unit, 0) + 1
                        src = {
                            "source": "local/claude-code/" + name,
                            "unit": unit,
                            "split_key": unit,
                            "turn": order[unit],
                            "call": 0,
                            "meta": {"tool_use_id": tid, "started_at": started, "finished_at": d.get("timestamp"),
                                     "file": os.path.relpath(path, proj)},
                        }
                        out.write(json.dumps({"event": ev, "src": src}, ensure_ascii=False) + "\n")


if __name__ == "__main__":
    main()
