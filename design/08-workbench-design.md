# Workbench design: data browser and encoder playground (draft 2, 2026-11-14)

Builds on 07 (R1–R6, D25 Go server + Python workers, D26 A1 + A2 first).

## Processes

P1. Go server: one binary. Serves the UI, owns the event store, holds websockets, talks to workers.
P2. Python worker(s): load models (Ettin, later our encoders), encode, predict, search neighbours. Separate processes; restartable without the server (07/S1).
P3. Worker protocol (Q20): newline-delimited JSON requests/responses over a Unix socket; tensors as binary frames (raw float32 + shape). A worker announces what it can do on connect (models, snapshots).
P4. Reachable over Tailscale; links by hostname, not IP (Q22).

## Event store

E1. Events in the 06/W1 schema, stored as content-addressed blobs (hash = ID), with a rebuildable index for browsing and filters (Q21).
E2. Every event records its source (dataset + item, or sandbox run) and split membership by source (06/G7).
E1a. Small blobs (user: "consider something more compact for sufficiently small blobs? Maybe it's not a big deal yet"): blobs under a size threshold (say 4 KiB) go inline in a separate blobs.db (SQLite), larger ones as files. Keeps the index DB rebuildable and avoids thousands of tiny files. Not urgent.
E3. Encodings cached under (encoder snapshot, type, content hash) (02/S3a), full slot bank stored (02/S3b).

## Data browser (A1)

B1. Event list: filter by source, program family, exit code, duration, has-stderr, split; free-text search over command and output.
B2. Event card: command with argv items visible, cwd, exit code with its surprisal (05/B1), duration, stdout/stderr with long-output folding and line numbers. The same card component is reused everywhere an event appears.
B3. Field view: per field, base-rate distribution and how many events have it; click through to examples at either tail (usual and unusual).
B4. Corpus stats: counts by source and split, dedupe rate, output length distribution (informs T6).
B5. Schema view: the command-execution schema, field types, which are mechanical.

## Encoder playground (A2)

C1. Pick an event (from the browser) or paste one; choose an encoder snapshot.
C2. Slots: K × d heatmap, per-slot norms, similarity between slots (collapse at a glance, 06/V4).
C3. Masked prediction: choose fields to mask; the predictor's output is a representation, not text, so show the **nearest real values** for that field from the corpus, with distances. That's how "what did it predict?" becomes readable.
C4. Neighbours: nearest events by slots, with the k slider (06/V3) showing how neighbours change as slots are dropped.
C5. Compare: two snapshots side by side on the same event (e.g. 17M vs 32M, 06/D28).
C6. Before any of our encoders exist: a baseline snapshot of mean-pooled Ettin embeddings per field (06/V6), so C1, C4, C5 work from day one. C2 and C3 need a trained slot head and predictor.

## Ettin playground (A2b, user request)

K1. A view for playing with Ettin directly, apart from our encoders. Proposed parts:
   - K1a. Tokenizer: type or pick text (a command, some output), see how it splits into tokens. Shows how terminal output, paths and numbers tokenize (informs 02/N3, 06/E4, Q16).
   - K1b. Fill-mask: put a mask in text and see the top predictions with probabilities. Ettin is MLM-trained, so this is its native behaviour. (Uses [MASK] deliberately; 06/X1a applies only to our input pipeline.)
   - K1c. Surprise heatmap: mask each token in turn and shade the text by how surprised Ettin is. Shows what it finds predictable in terminal output vs prose.
   - K1d. Similarity: two or more texts, cosine similarity of their embeddings; and nearest field values from the corpus for a typed text.
   - K1e. Later: per-layer view (how a text's representation changes by layer, for 05/T2 reshaping); attention maps.
   - K1f. Size switch: 17M vs 32M side by side on any of the above (06/D28).

## Wire budget (user: "minimize data over the wire", Tailscale over two residential connections)

N1. Static assets: small, no frameworks, content-hashed filenames with long cache lifetimes; fonts subset and woff2. After the first visit, a page load should send almost nothing but data.
N2. Tensors for display are quantised on the server: uint8 is enough for a heatmap, float16 at most for anything numeric. Never ship float32 slot banks to the browser.
N3. Heatmaps and charts downsampled server-side to what the screen can show.
N4. Long outputs lazy: first lines plus a count; the rest on expand, by range.
N5. Lists are paginated; neighbour results are IDs + short previews; the shared event card fetches its full content once and caches it in the page.
N6. Websocket frames are binary where it matters, with permessage-deflate on; progress updates are throttled and send changes, not snapshots.
N7. Compression on all HTTP responses (zstd or gzip).

## Live behaviour

L1. Websockets for long-running work: corpus import, encoding the corpus, neighbour index builds (progress, then result). Plain requests for ordinary page loads.
L2. Playground interactions feel live: mask toggles and the k slider update without a page reload.

## Look

V1. A small design system first (07/S4): palette tokens, type scale, spacing, one card style, one table style, one chart style; light and dark (Q23).
V2. Fonts self-hosted; no external requests from the app.
V3. Minimalist: content-first, few colours with meaning (e.g. one accent, one for failure, one for surprise), monospace for command and output only.

## Data for the first examples

X1. Public agent trajectories with command/output pairs (SWE-style coding agents, web/tool-use agents) on Hugging Face; formats and licences unchecked (Q24).
X2. Our own: run commands in a sandbox and record them in the schema. Gives full fields (duration, exit code) that public traces may lack.

## Data found (checked 2026-11-14)

X3. Public, checked by sampling first rows:
   - X3a. nvidia/Open-SWE-Traces: 1.14M trajectories, 97.9 GB, CC-BY-4.0; five scaffolds (claudecode, opencode, openhands, sweagent, minisweagent); recent models (Qwen3.8, DeepSeek-V4-Flash, MiniMax, GLM). Its **minisweagent** split stores each observation as JSON `{"returncode": N, "output": "..."}`: per-command exit codes. The openhands split's observations are raw output, no exit code seen.
   - X3b. AlienKevin/SWE-ZERO-12M-trajectories: 12.3M rollouts, 36 GB, Apache-2.0, mini-swe-agent format, by a 1.7B model. Observations are plain "Observation: ..." text, no exit code. Weak agent, but outputs are real executions, which is what the encoder learns from.
   - X3c. Seen but not checked: nvidia/Nemotron-SWE-v1, LiteCoder-SFT-Terminal (11k terminal trajectories).
   - X3d. None seen record per-command duration.
X4. On the box:
   - X4a. Claude Code transcripts (~/.claude/projects): about 4,000 Bash calls with outputs (~8 MB of output). Needs dedupe (results outnumber calls in one folder, likely repeated lines across files). Errors flagged; exit code only in error text; no duration. Commands are often compound shell lines (cd … && …), so argv needs a shell parser. Includes sessions under ~/work.
   - X4b. Other local logs: about 150 tool executions in another harness's event log; a few Pi sessions; 20 lines of bash history. Small.
X5. Implications: duration is optional in the schema (mechanical with a "missing" flag); exit code too. Sandbox runs (X2) are the only source with every field.

## Built (2026-11-14)

U1. First build in ~/projects/system-one/ganglion: Overview, Events, Event, Fields, Ettin, Design pages; Go server + Python worker (CPU torch 2.15, transformers 5.23, Python 3.13); look in 09.
U2. Corpus: one Open-SWE shard (minisweagent/qwen38_flash_next/swe-rebench-v2, train-00000, 2,500 trajectories) → 156,104 sightings; plus 2,463 local Claude Code Bash calls (from ~/.claude/projects/-home-andy only; the ~/work sessions are not imported). 158,220 distinct events from 158,567 sightings (347 duplicates), 1.3 GB on disk with the index.
   - This split stores observations as `<returncode>N</returncode><output>…</output>`, with `<output_head>/<output_tail>` when truncated and `<exception>` for harness errors (timeouts). The JSON form seen in the viewer is a different split.
   - 32 identical events land in both train and test (trivial commands); shown on the Overview.
U3. Ettin ships `pytorch_model.bin`, not safetensors; loaded with transformers' weights-only loading. Worker identity reports revision SHA and file hashes; every request and reply is logged to worker-log/.
U4. Finding: raw mean-pooled Ettin embeddings are nearly collinear (cosine ~0.99 between unrelated commands). The nearest index centres on the corpus mean and renormalises; that helps (git queries find git commands) but pooled MLM embeddings stay a weak baseline. Building it takes ~90 s for 60k commands on CPU.
U5. Runs as systemd user services ganglion.service and ganglion-worker.service (units in ganglion/deploy; `deploy/install.sh` builds to ~/.local/bin/ganglion, installs, restarts). Lingering is on, so they survive logout and reboot.
U6. ~/work Claude Code transcripts imported too (user: "It's fine to import them"): 1,540 more events in the local split, as source local/claude-code/-home-andy-work.
U7. Browser checks: ganglion/test/browser/run.sh (Playwright screenshots, both themes, desktop and phone, console errors, overflow). Tooling kept in ~/.local/share/ganglion-tools (Playwright venv, Chromium libraries unpacked from RPMs, not installed system-wide).

## Name

M1. **ganglion**: a cluster of nerve cells outside the brain that handles signals locally, much as system one does. Port 7171 (free on the box as of 2026-11-14). Link as http://framework:7171.

## Decided

D32. Worker protocol: NDJSON over a Unix socket, binary tensor frames (P3). Answers Q20. (user: "Fine with me.")
D33. Store: blobs plus SQLite as the index (E1). Small blobs inline in a separate blobs.db, large ones as files (E1a). Answers Q21. (user; E1a: "Sounds good to me.")
D34. Access: tailnet trusted, no login for now. Answers Q22. (user: "Correct.")
D35. Look: support both light and dark. Gruvbox dark is an example of something the user likes, not a suggestion. Answers Q23. (user)
   - My reading for the palette: warm, low-glare, muted accents rather than neon; its own palette, not gruvbox.
D36. First data: public trajectory datasets plus anything suitable on the box (X3–X5). Answers Q24. (user: "Sounds good.")
D37. Name and port: my pick (M1). Answers Q25. (user: "Surprise me!")
D38. An Ettin playground view (K1). (user request)
D39. Minimise data over the wire (N1–N7). (user)
D40. Starting sources: a slice of the Open-SWE-Traces minisweagent split as the main corpus; local Claude Code transcripts as a separate source (held-out candidate); sandbox runs later; SWE-ZERO for volume if needed. Answers Q26. (user: "Sounds good to me.")
D41. Ettin playground: K1a–K1d and K1f first, K1e later. Answers Q27. (user: "Yes.")

## Open questions

(none for this note)
