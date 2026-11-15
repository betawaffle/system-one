# Using a local model (draft 1, 2026-11-15)

The model does a first pass; the user reviews it (user: "A first pair of eyes that I review (as the second)"). It never decides anything on its own: its output is a suggestion, labelled as the model's, and only the user's verdicts and choices count.

## Runtime

R1. The user's `llm-server` (llama.cpp, Vulkan, OpenAI-style API on port 8081, router mode) with `gpt-oss-120b` (MXFP4, Apache 2.0, ~5B active parameters). From its README: ~52 tok/s generation, ~960 tok/s prompt processing, ~10 s cold start, unloads after 15 idle minutes, ~63 GB of GPU memory while loaded, 16 parallel slots sharing a 128K cache.
R2. The unit is currently disabled. Starting it is the user's call (Q76). If it's down, every page still works; model parts say "no model" (sd/08/M1).
R3. `reasoning_effort` per call: low for the per-sample work, higher for the rare batch jobs. Output is constrained to a JSON schema (`response_format: json_schema`, which llama.cpp turns into a grammar), so it always parses.

## The call path (sd/07/D90)

C1. One Go package in ganglion makes every model call; nothing else talks to the server.
C2. Each call appends a record to `<data>/model-calls/YYYY-MM.jsonl` (per CAPTURE-REQUIREMENTS): the exact request body bytes, the exact response bytes, response headers, requested and returned model, timestamps (sent, first byte, done), the model file's sha256 (computed once), llama.cpp's build and chat template (from its `/props` endpoint), sampling settings and seed, errors and retries. Records get an id; events that use a result point at it.
C3. Calls are asynchronous: pages show samples and alignment at once, and model output arrives later. Nothing waits on the model.

## What it's used for

M1. **First pass over samples.** The main use.
  - When: every time samples appear (Write previews, Coverage's Show Me), for each sample not yet judged by the user.
  - Call: one request per sample, run in parallel on the server's slots, so each sample is judged on its own, not anchored on its neighbours. The shared start (instructions, meaning card, template) is the same across a batch, so the server reuses it.
  - Input: the meaning card (frame and its explainer, roles and fillers, features, extra meaning), the user's own sentence as the known-good reference, the sample and its tags, the verdict definitions (the same explainers the buttons show), the user's past ◐ notes so it reuses their wording, and a few of the user's own verdicts on similar samples as examples of their taste (Q74).
  - Output:
    ```json
    {"verdict": "ok | but | bad",
     "but_kind": "context | implies | null",
     "reason": "Word Form | Wrong Kind of Filler | Ungrammatical | Means Something Else | Sounds Odd | Other | null",
     "note": "short, in the user's wording where one fits",
     "confidence": "low | medium | high"}
    ```
  - Shown: on each sample, a line under the sentence: "Model: ◐ implies the apple changed (medium)". The user's buttons stay as they are; **Accept** (key A) copies the model's verdict to theirs. Samples the model flagged (◐ or ✕) sort first, and so do ones where its confidence is low.
  - Used: logged as a `model-verdict` event with the call id. When the user gives a verdict, the event records whether it matches the model's. Coverage counts only the user's verdicts.
M2. **Calibration against the user's verdicts.** Before relying on M1, and continuously after.
  - Run M1 over every sample the user has already judged, and report agreement: overall, by verdict, by reason, and where they disagree. After that, every new user verdict updates the numbers.
  - Shown: at the top of Coverage: "The model agrees with you on 41 of 48 (✓ 30/31, ◐ 6/10, ✕ 5/7)", with the disagreements listed for a look.
  - Used: to decide how much weight its flags get, and whether bulk acceptance is ever safe (Q75). Also to compare reasoning effort settings (Q77).
M3. **Span suggestions.** Replaces the keyword guesses (sd/08/W4).
  - When: an untraced span appears.
  - Input: meaning card, the sentence, the span, the choices with explainers, existing modifier kinds and frames.
  - Output: `{"choice": "...", "detail": "an existing kind, or a new name", "explainer": "one line, for a new one", "confidence": "..."}`
  - Shown: as now, preselected and tagged **Model** instead of Guess; the user still confirms.
M4. **Word helper** (sd/03/A8).
  - When: the user clicks a word.
  - Input: the sentence, the word and what it traces to, the meaning card.
  - Output: `{"role": "...", "same": ["..."], "different": [{"word": "...", "adds": "..."}], "breaks": [{"word": "...", "why": "..."}]}`. Words are checked against the lexicon; unknown ones are dropped.
  - Shown: the table, under the lexicon's forms; the user's edits are vetting data (sd/03/A8c).
M5. **Notes into rules.** On demand, not per sample.
  - Input: all ◐ notes and ✕ reasons with their samples and tags.
  - Output: candidate rules: `{"pattern": "...", "mechanism": "attribute can-change | ambiguous word | frame kind | other", "examples": [...], "confidence": "..."}`, using the mechanisms already in the grammar where they fit.
  - Shown: a Rule Ideas list on Coverage. The user picks which to make real; implementing one is a code change (with me) until rules can be edited in the UI (sd/06/T18).
M6. Later: the random audit before an export (sd/08/W9d), where the model judges the drawn rows and the user reviews disagreements; and proposing new realisations of a meaning for the user to vet (sd/03/A5).

## Guardrails

G1. Model output is never a verdict, a choice or a rule until the user accepts it; everywhere it appears it says it's the model's.
G2. Schema-constrained output, plus validation (reasons and choices from the fixed lists, notes length-limited, helper words in the lexicon). Anything that fails is shown as "the model's answer didn't fit" and logged.
G3. Timeouts and the cold start: the first call after idle takes ~10 s; pages say "model warming up" instead of spinning silently.

## Decided

D110. The model does the first pass over samples and the user reviews it; only the user's verdicts count. (user: "A first pair of eyes that I review (as the second).")
D111. Use the user's llm-server with gpt-oss-120b, configured from ~/work/llm. (user: "Yes.")

## Open questions

Q73. One call per sample, in parallel (M1), rather than one call per batch? I lean per sample: no anchoring, and the server's parallel slots make it about as fast.
Q74. Few-shot examples from the user's own verdicts (a handful of recent ✕ and ◐ with notes, and some ✓, from the same frame)? I lean yes: it's how the model learns this user's taste rather than generic taste.
Q75. Accept only one sample at a time (key A), with no "accept all", until calibration (M2) shows high agreement? I lean yes.
Q76. Enable llm-server so it starts at boot (it still sleeps after 15 idle minutes and frees its memory), or start it only while working on this? I lean enable.
Q77. Low reasoning effort for M1, checked against medium on the calibration set? I lean yes: start low, measure.
