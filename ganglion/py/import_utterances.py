"""Turn transcripts and documents into ganglion utterance events (design/11).

Usage: python -I import_utterances.py SOURCE... | ganglion import
  claude-code          every project under ~/.claude/projects
  tack                 tack conversations, read through tack-show -full
  pi                   Pi sessions under ~/pi-sandbox/pi-home/sessions
  documents            our Markdown: design notes, READMEs, memory, tack docs
  commits              commit messages in ~/tack and ~/projects/system-one

Who said what comes from structure only, never from guessing (capture rule 3):
  - Claude Code marks typed messages with origin.kind == "human"; older
    transcripts have no origin, so plain user text there counts as typed.
  - Known harness tags (<system-reminder>, <task-notification>, <command-*>,
    <local-command-*>, …) are cut out as the harness's, each tagged.
  - <pasted_content> is the person's paste, kept apart from what they typed.
  - Relay wrappers (<wake>…<message from="human">) are unwrapped.
  - Messages the person sent mid-turn ("The user sent a new message while
    you were working: …") are pulled out of their wrapper.
  - Compaction summaries are model-written: speaker claude, kind summary.
  - Sidechains: the main session's prompt to a subagent is claude/prompt;
    the subagent's text is subagent/reply.
"""
import glob
import html
import json
import os
import re
import subprocess
import sys

HOME = os.path.expanduser("~")
OUT = sys.stdout
SEEN = set()

HARNESS_TAGS = ["system-reminder", "task-notification", "local-command-stdout", "local-command-stderr", "local-command-caveat",
                "command-name", "command-message", "command-args", "system-note", "tools-used-before-summary", "relay",
                "agent-message", "attachment", "mod", "details", "reactions"]
TAG_RE = re.compile(r"<(%s)\b[^>]*>.*?</\1>" % "|".join(map(re.escape, HARNESS_TAGS)), re.S)
PASTE_RE = re.compile(r"<pasted_content\b[^>]*>(.*?)</pasted_content>", re.S)
MIDTURN_RE = re.compile(r"The user sent a new message while you were working:\n(.*?)\n\nThis is how Claude Code surfaces", re.S)
WAKE_MSG_RE = re.compile(r"<message\b([^>]*)>(.*?)</message>", re.S)
FROM_RE = re.compile(r'\bfrom="([^"]+)"')


def emit(speaker, kind, text, source, unit, turn, call, at=None, model=None, tag=None, **meta):
    text = (text or "").strip()
    if not text:
        return
    key = (source, unit, turn, call)
    if key in SEEN:
        return
    SEEN.add(key)
    ev = {"type": "utterance", "speaker": speaker, "kind": kind, "model": model, "tag": tag, "text": text}
    m = {"at": at} if at else {}
    m.update({k: v for k, v in meta.items() if v is not None})
    OUT.write(json.dumps({"event": ev, "src": {"source": source, "unit": unit, "split_key": unit, "turn": turn, "call": call, "meta": m}},
                         ensure_ascii=False) + "\n")


def split_human(text):
    """Split a human-origin user text into [(speaker, kind, tag, text)]."""
    parts = []
    for m in MIDTURN_RE.finditer(text):
        parts.append(("human", "typed", "midturn", m.group(1)))
    text = MIDTURN_RE.sub("", text)
    for m in WAKE_MSG_RE.finditer(text):
        who = (FROM_RE.search(m.group(1)) or [None, "unknown"])[1]
        inner = html.unescape(m.group(2))
        for t in TAG_RE.finditer(inner):
            parts.append(("harness", "injected", t.group(1), t.group(0)))
        parts.append(("human" if who == "human" else who, "typed", "relay", TAG_RE.sub("", inner)))
    if "<wake" in text:
        text = re.sub(r"<wake\b.*?</wake>", "", text, flags=re.S)
    for m in PASTE_RE.finditer(text):
        parts.append(("human", "pasted", None, m.group(1)))
    text = PASTE_RE.sub("", text)
    for m in TAG_RE.finditer(text):
        parts.append(("harness", "injected", m.group(1), m.group(0)))
    text = TAG_RE.sub("", text)
    if text.strip():
        parts.append(("human", "typed", None, text))
    return parts


def harness_kind(text):
    t = text.lstrip()
    if t.startswith("Base directory for this skill:"):
        return "skill"
    if t.startswith("[Request interrupted"):
        return "interrupt"
    if t.startswith("[Image") or t.startswith("[SYSTEM NOTIFICATION"):
        return "notice"
    if t in ("Tool loaded.", "Tools loaded."):
        return "tool-load"
    return None


def claude_code():
    for proj in sorted(glob.glob(os.path.join(HOME, ".claude", "projects", "*"))):
        name = os.path.basename(proj)
        for path in sorted(glob.glob(os.path.join(proj, "**", "*.jsonl"), recursive=True)):
            source = "claude-code/" + name
            for line in open(path, errors="replace"):
                try:
                    d = json.loads(line)
                except json.JSONDecodeError:
                    continue
                msg = d.get("message") or {}
                role, content = msg.get("role"), msg.get("content")
                uuid, session, at = d.get("uuid"), d.get("sessionId") or path, d.get("timestamp")
                if not uuid or role not in ("user", "assistant"):
                    continue
                blocks = [{"type": "text", "text": content}] if isinstance(content, str) else content if isinstance(content, list) else []
                side = bool(d.get("isSidechain"))
                origin = (d.get("origin") or {}).get("kind")
                turn = int(uuid.replace("-", "")[:12], 16)  # stable per message across resumed files
                for i, b in enumerate(blocks):
                    if not isinstance(b, dict):
                        continue
                    t = b.get("type")
                    common = dict(source=source, unit=session, turn=turn, at=at, file=os.path.relpath(path, proj), uuid=uuid)
                    if role == "assistant":
                        model = msg.get("model")
                        if t == "text":
                            emit("subagent" if side else "claude", "reply", b.get("text"), call=i * 10, model=model, **common)
                        elif t == "thinking" and b.get("thinking"):
                            emit("subagent" if side else "claude", "thinking", b.get("thinking"), call=i * 10, model=model, **common)
                        continue
                    if t != "text":
                        continue
                    text = b.get("text") or ""
                    if side:
                        emit("claude", "prompt", text, call=i * 10, **common)
                    elif d.get("isCompactSummary"):
                        emit("claude", "summary", text, call=i * 10, **common)
                    elif origin == "human" or (origin is None and not d.get("isMeta") and not harness_kind(text)):
                        for j, (who, kind, tag, part) in enumerate(split_human(text)):
                            emit(who, kind, part, call=i * 10 + j, tag=tag, **common)
                    else:
                        tag = origin or harness_kind(text) or ("meta" if d.get("isMeta") else "other")
                        emit("harness", "injected", text, call=i * 10, tag=tag, **common)


def tack():
    listing = subprocess.run(["tack-show"], capture_output=True, text=True, check=True).stdout
    sect = re.compile(r"^--- (\d+)\. (\w+)\s*$", re.M)
    for conv in [l.split()[0] for l in listing.splitlines() if l.strip()]:
        out = subprocess.run(["tack-show", "-full", conv], capture_output=True, text=True).stdout
        head = re.search(r"^model (\S+)", out, re.M)
        model = head.group(1) if head else None
        marks = list(sect.finditer(out))
        for k, m in enumerate(marks):
            n, role = int(m.group(1)), m.group(2)
            body = out[m.end():marks[k + 1].start() if k + 1 < len(marks) else len(out)]
            common = dict(source="tack", unit=conv, turn=n)
            if role == "user":
                emit("human", "typed", body, call=0, **common)
            elif role == "system":
                emit("harness", "injected", body, call=0, tag="system-prompt", **common)
            elif role == "assistant":
                for j, r in enumerate(re.findall(r"\[reasoning\](.*?)\[/reasoning\]", body, re.S)):
                    emit("model", "thinking", r, call=1 + j, model=model, **common)
                rest = re.sub(r"\[reasoning\].*?\[/reasoning\]", "", body, flags=re.S)
                keep = []
                skip_indented = False
                for line in rest.splitlines():
                    if line.startswith("[tool ") or re.match(r"^\[[A-Za-z]+, \d+ in", line):
                        skip_indented = line.startswith("[tool ")
                        continue
                    if skip_indented and (line.startswith("  ") or not line.strip()):
                        continue
                    skip_indented = False
                    keep.append(line)
                emit("model", "reply", "\n".join(keep), call=0, model=model, **common)


def pi():
    for path in sorted(glob.glob(os.path.join(HOME, "pi-sandbox", "pi-home", "sessions", "*", "*.jsonl"))):
        unit = os.path.basename(path)
        model = None
        for n, line in enumerate(open(path, errors="replace")):
            d = json.loads(line)
            if d.get("type") == "model_change":
                model = d.get("modelId") or d.get("model") or model
            if d.get("type") != "message":
                continue
            msg = d.get("message") or {}
            role = msg.get("role")
            model = msg.get("model") or model
            for i, b in enumerate(msg.get("content") or []):
                if not isinstance(b, dict):
                    continue
                common = dict(source="pi", unit=unit, turn=n, call=i, at=d.get("timestamp"))
                if role == "user" and b.get("type") == "text":
                    emit("human", "typed", b.get("text"), **common)
                elif role == "assistant" and b.get("type") == "text":
                    emit("model", "reply", b.get("text"), model=model, **common)
                elif role == "assistant" and b.get("type") == "thinking":
                    emit("model", "thinking", b.get("thinking"), model=model, **common)


def git_authors(repo, path):
    r = subprocess.run(["git", "-C", repo, "log", "--format=%an", "--", path], capture_output=True, text=True)
    return {a for a in r.stdout.split("\n") if a}


def documents():
    """Markdown we wrote. Authorship by git history where there is one: all-Claude → claude,
    otherwise mixed. Notes and memory files outside git are Claude-written."""
    files = []
    for repo in (os.path.join(HOME, "tack"), os.path.join(HOME, "projects", "system-one")):
        r = subprocess.run(["git", "-C", repo, "ls-files", "*.md"], capture_output=True, text=True)
        files += [(repo, f) for f in r.stdout.split()]
    for path in glob.glob(os.path.join(HOME, "projects", "*", "**", "*.md"), recursive=True):
        if "/system-one/" not in path and "/node_modules/" not in path:
            files.append((None, path))
    for path in glob.glob(os.path.join(HOME, ".claude", "projects", "*", "memory", "*.md")):
        files.append((None, path))
    if os.path.exists(os.path.join(HOME, "pi-sandbox", "HANDOVER.md")):
        files.append((None, os.path.join(HOME, "pi-sandbox", "HANDOVER.md")))
    for repo, f in files:
        path = os.path.join(repo, f) if repo else f
        if "LICENSE" in path:
            continue
        if repo:
            authors = git_authors(repo, f)
            speaker = "claude" if authors == {"Claude"} else "mixed"
        else:
            speaker = "claude"
        rel = os.path.relpath(path, HOME)
        emit(speaker, "document", open(path, errors="replace").read(), source="documents", unit=rel, turn=0, call=0,
             path=rel)


def commits():
    for repo in (os.path.join(HOME, "tack"), os.path.join(HOME, "projects", "system-one")):
        r = subprocess.run(["git", "-C", repo, "log", "--format=%H%x1f%an%x1f%aI%x1f%B%x1e"], capture_output=True, text=True)
        for rec in r.stdout.split("\x1e"):
            parts = rec.strip("\n").split("\x1f")
            if len(parts) < 4:
                continue
            h, author, at, body = parts
            body = re.sub(r"^Co-Authored-By:.*$", "", body, flags=re.M)
            emit("claude" if author == "Claude" else "human", "commit", body, source="commits/" + os.path.basename(repo),
                 unit=h, turn=0, call=0, at=at)


PR_RE = re.compile(r"<pr_description>\s*Consider the following PR description:\s*(.*?)</pr_description>", re.S)


def openswe():
    """Open-SWE shards already on disk: GitHub PR descriptions (speaker author: third-party
    people, not the person in our sessions) and the agent model's replies and reasoning."""
    import pyarrow.parquet as pq
    for path in sorted(glob.glob(os.path.join(HOME, ".local", "share", "ganglion", "raw", "openswe-*.parquet"))):
        source = "openswe/" + os.path.basename(path).removesuffix(".parquet")
        pf = pq.ParquetFile(path)
        for batch in pf.iter_batches(batch_size=64, columns=["instance_id", "trajectory_id", "messages", "metadata"]):
            for row in batch.to_pylist():
                model = ((row.get("metadata") or {}).get("teacher_model") or {}).get("name")
                unit = row["trajectory_id"]
                for n, m in enumerate(row["messages"]):
                    common = dict(source=source, unit=unit, turn=n, instance=row["instance_id"])
                    if m["role"] == "user" and n <= 1:
                        pr = PR_RE.search(m["content"] or "")
                        if pr:
                            emit("author", "issue", pr.group(1), call=0, **common)
                    elif m["role"] == "assistant":
                        emit("model", "reply", m.get("content"), call=0, model=model, **common)
                        emit("model", "thinking", m.get("reasoning_content"), call=1, model=model, **common)


FINEWEB = "/data/corpora/fineweb-edu/000_00000.parquet"


def fineweb(every=15):
    """fineweb-edu sample/10BT shard 000_00000 (web pages kept by fineweb's educational
    quality classifier). Speaker web; a deterministic 1-in-`every` sample by document ID.
    Never print this text into a Claude session (the corpora README asks for summaries only)."""
    import hashlib
    import pyarrow.parquet as pq
    source = "fineweb-edu/sample-10BT/000_00000"
    pf = pq.ParquetFile(FINEWEB)
    for batch in pf.iter_batches(batch_size=2000, columns=["text", "id", "dump", "url", "language_score", "token_count", "score", "int_score"]):
        for row in batch.to_pylist():
            if int(hashlib.sha256(row["id"].encode()).hexdigest()[:8], 16) % every:
                continue
            emit("web", "document", row["text"], source=source, unit=row["id"], turn=0, call=0,
                 url=row["url"], dump=row["dump"], score=row["score"], int_score=row["int_score"],
                 language_score=row["language_score"], token_count=row["token_count"], selection=f"1 in {every} by sha256(id)")


if __name__ == "__main__":
    for s in sys.argv[1:]:
        {"claude-code": claude_code, "tack": tack, "pi": pi, "documents": documents, "commits": commits, "openswe": openswe,
         "fineweb": fineweb}[s]()
