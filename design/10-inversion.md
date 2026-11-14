# Embedding inversion (draft 1, 2026-11-14)

Question (user): can the Ettin decoder "reproduce" anything from the encoder's embeddings?

## Background

B1. Not out of the box: Ettin's encoder/decoder pairs share data, recipe and tokenizer (both 50,368 tokens, checked), but the decoder only reads tokens. No cross-attention, no shared latent.
B2. A bridge is well-trodden: a projector turns the embedding into K soft tokens that prefix the decoder (LLaVA projector, prefix tuning). vec2text (Morris et al., 2023) inverted GTR embeddings to ~92% exact match on 32-token texts with iterative correction: embeddings keep more than they seem, and aren't a privacy boundary.
B3. Uses for us: an inspection probe for slots (decode with k slots, then fewer, 08/A2); practice for the speech effector (03/F1). Keep reconstruction gradients out of the encoder (05/P1): they'd push slots to keep everything.

## Setup (built)

S1. Image localhost/ganglion-gpu (ganglion/train/Containerfile): the gpu-run image (pinned ROCm 10.1 PyTorch 2.14 + git) plus pinned transformers 5.23, peft 0.21.3, pyarrow, same constraints pattern as the marimo image. train/run.sh launches it like gpu-run (GPU devices, experiments.slice), mounting the repo at /work, the store at /store, /data (HF_HOME=/data/hf).
S2. train/invert.py:
   - target: the exact embedding the nearest index uses (Ettin encoder 17M, mean-pooled, normalised, centred on the train mean, renormalised)
   - projector: 256 → 2048 → 16 soft tokens × 768, LayerNorm, learned scale
   - decoder: Ettin decoder 150M (Q29), LoRA r=32 on all linear layers; ~33.6M trainable
   - data: distinct commands ≤ 300 chars; train 101,355 / test 11,314 / local 1,536 (held-out sets exclude anything seen in train)
   - 3 epochs, batch 128, bf16, cosine schedule; greedy decoding for eval
   - baseline: retrieve the nearest training command by embedding. Inversion has to beat that to show it generates rather than recalls.
   - each run writes config (data counts, model revisions and file hashes, git commit, image), log, metrics, eval examples, projector + merged decoder under ~/.local/share/ganglion/runs/
S3. Playground "Invert" tab: type a command, see the reconstruction with word diff, the re-embedded cosine, and the run's held-out metrics next to the retrieval baseline. Runs on CPU in the worker; picks up the newest finished run.

## Embedding geometry (measured 2026-11-14)

Question (user): why can't two sentences be made to look dramatically different? Playground Similarity now has a Raw / Centred / Whitened switch and a geometry panel, from 5,000 sampled corpus commands and 20k random pairs per model.

G1. Raw mean-pooled embeddings are a narrow cone. Random pairs: mean cosine 0.99 (17M), 0.96 (32M); middle 90% at 0.97–0.99 and 0.94–0.98. The mean vector has norm 0.99 / 0.98 on unit vectors, so every embedding is almost the mean.
G2. Centring (what Nearest does) spreads random pairs to −0.46…0.49 (17M) and −0.32…0.37 (32M), mean 0.
G3. Whitening (BERT-whitening) makes random pairs nearly orthogonal: middle 90% within ±0.10. Read whitened scores against that band: 0.3 is already well above chance.
G4. Few directions carry the variance: effective dimensions (participation ratio) 9.6 of 256 for 17M, 19.5 of 384 for 32M; the top direction holds 23.5% / 16.2%.
G5. Example (whitened, 17M / 32M): `ls -la` vs `ls -al` 0.80 / 0.85; vs `find . -maxdepth 1` 0.15 / 0.31; vs prose "The weather is lovely today." 0.29 / 0.28. Surface overlap still beats meaning (the find command means nearly the same as ls -la), as expected from an MLM.
G6. So "weak" is two things: geometry (fixable with statistics) and content (lexical, needs training). The playground showed raw cosine, which hides the first.

## Results

R1. Run invert-command-20261114-203642: 2,376 steps, 67 min on the 8060S (~1.7 s/step), final training loss 0.65 nats/token (perplexity ~1.9; it started at 4.7). Started from commit e3ec848 with uncommitted changes (recorded in its config).
R2. Held-out (1,000 each), inverter vs retrieving the nearest training command:
   - test: exact 6.6% vs 0%; token F1 0.734 vs 0.638; char match 0.711 vs 0.632; re-embedded cosine 0.856 (centred)
   - local (your Claude Code commands): exact 0% vs 0%; token F1 0.360 vs 0.301; char 0.355 vs 0.286; cosine 0.679
R3. Not memorising: only 3 of 1,000 test outputs are exactly a training command. It beats retrieval on 683 test commands and loses on 147 (local: 641 vs 270).
R4. Length decides it (test): under 40 chars 59% exact, F1 0.87; 40–100 chars 11% exact, F1 0.81; over 100 chars 0.2% exact, F1 0.66. One 256-number vector holds a short command almost fully; long ones keep their shape and lose specifics.
R5. What it keeps: the command's structure, flags, pipes, file types, directory prefixes, many identifiers (e.g. `def parse_timestamp`, `crates/ide/src`). What it loses: rare identifiers, line numbers, exact search terms; it substitutes plausible ones (hallucination with the right shape).
R6. Local commands show the training prior winning: `cd /home/andy/tack-flaky-headings && …` comes back as `cd /testbed && …`, but the topic survives (browser tests, check scripts, grep for idle/send). Embeddings leak topic and structure even out of distribution: not a privacy boundary.
R7. Answer to the question: yes, quite a lot. Despite the near-collinear geometry (G1), the 17M pooled embedding carries most of a command's content, and more than its nearest neighbour does. The weakness is in how the information is laid out, not in whether it's there.
R8. Invert tab: 0.3–1.5 s per command on CPU (greedy, no KV cache).
R9. Prose (user asked), with the command-trained inverter, 8 sentences: every output is command-shaped; only the gist sometimes survives ("The weather is lovely today." → `ls day-of-week.yml`; "I fixed the bug in the parser." → `sleep 15; git diff`; "…look at the test file to understand the failure." → `go test -race -run …`). The decoder's prior is commands, and the centring statistics are commands' too. Re-embedded cosine is high for some wrong outputs (0.81 for weather → yml), so that metric is unreliable out of distribution.

## Decided

D43. Build on the gpu-run image, not the worker's CPU environment (user: "build a new image based on gpu-run like we did for marimo"). Answers Q28.
D44. Ettin decoder 150M. Answers Q29. (user: "yes, 150M")

## Open questions

(none for this note yet)

## Future topics

T7. vec2text-style iterative correction, if one-shot inversion looks promising.
T8. Output chunks as a second field; our own encoder's slots once they exist (the slot probe).
