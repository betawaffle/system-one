"""Convert an Open-SWE-Traces mini-swe-agent parquet shard to ganglion NDJSON events.

Usage: python import_openswe.py SHARD.parquet SOURCE_NAME | ganglion import

Each bash tool call plus the tool message answering it becomes one
command_execution event. Tool messages follow their assistant message in call
order (they carry no call id), so calls and results are matched by position.
"""
import json
import re
import sys

import pyarrow.parquet as pq

RC = re.compile(r"^<returncode>(-?\d+)</returncode>\s*", re.S)
TAG = lambda name: re.compile(rf"<{name}>\n?(.*?)\n?</{name}>", re.S)
OUTPUT, HEAD, TAIL, EXC, WARN = TAG("output"), TAG("output_head"), TAG("output_tail"), TAG("exception"), TAG("warning")
ELIDED = TAG("elided_chars")


def parse_observation(content):
    """Returns (exit_code, output, truncated, error) from a tool message."""
    content = content or ""
    m = RC.match(content)
    if not m:
        e = EXC.search(content)
        return None, None, False, (e.group(1) if e else content)
    code = int(m.group(1))
    rest = content[m.end():]
    o = OUTPUT.search(rest)
    if o:
        return code, o.group(1), False, None
    h, t = HEAD.search(rest), TAIL.search(rest)
    if h or t:
        el = ELIDED.search(rest)
        mid = f"\n[… {el.group(1).strip()} characters elided …]\n" if el else "\n[…]\n"
        return code, (h.group(1) if h else "") + mid + (t.group(1) if t else ""), True, None
    return code, rest, False, None


def main():
    path, source = sys.argv[1], sys.argv[2]
    pf = pq.ParquetFile(path)
    out = sys.stdout
    cols = ["instance_id", "repo", "language", "trajectory_id", "messages", "resolved", "metadata"]
    for batch in pf.iter_batches(batch_size=64, columns=cols):
        for row in batch.to_pylist():
            teacher = ((row.get("metadata") or {}).get("teacher_model") or {})
            pending = []
            turn = 0
            for m in row["messages"]:
                if m["role"] == "assistant":
                    turn += 1
                    pending = [(i, tc) for i, tc in enumerate(m.get("tool_calls") or [])]
                elif m["role"] == "tool" and pending:
                    i, tc = pending.pop(0)
                    fn = tc.get("function") or {}
                    if fn.get("name") != "bash":
                        continue
                    try:
                        command = json.loads(fn.get("arguments") or "{}").get("command")
                    except json.JSONDecodeError:
                        command = None
                    if not command:
                        continue
                    code, output, truncated, error = parse_observation(m.get("content"))
                    ev = {
                        "type": "command_execution",
                        "command": command,
                        "cwd": None,
                        "exit_code": code,
                        "output": output,
                        "stdout": None,
                        "stderr": None,
                        "truncated": truncated,
                        "error": error,
                        "duration_ms": None,
                    }
                    src = {
                        "source": source,
                        "unit": row["trajectory_id"],
                        "split_key": row["instance_id"],
                        "turn": turn,
                        "call": i,
                        "meta": {
                            "instance_id": row["instance_id"],
                            "repo": row["repo"],
                            "language": row["language"],
                            "resolved": row["resolved"],
                            "model": teacher.get("name"),
                            "reasoning_effort": teacher.get("reasoning_effort"),
                        },
                    }
                    out.write(json.dumps({"event": ev, "src": src}, ensure_ascii=False) + "\n")


if __name__ == "__main__":
    main()
