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

R1. (pending: first full run started 2026-11-14 20:36 UTC)

## Decided

D43. Build on the gpu-run image, not the worker's CPU environment (user: "build a new image based on gpu-run like we did for marimo"). Answers Q28.
D44. Ettin decoder 150M. Answers Q29. (user: "yes, 150M")

## Open questions

(none for this note yet)

## Future topics

T7. vec2text-style iterative correction, if one-shot inversion looks promising.
T8. Output chunks as a second field; our own encoder's slots once they exist (the slot probe).
