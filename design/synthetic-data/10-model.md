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

## Build status (2026-11-15)

B1. Built: llm-server enabled at boot (D115); the logged call path (`internal/llm`, records in `<data>/model-calls/`, chat template stored by hash, model file sha256 cached in `model-files.json`); the first pass on Write and Coverage (M1) with Accept (key A), accepted verdicts recorded with `via: model` and the call id; calibration on Coverage (M2) per prompt version and effort, with the disagreements listed. Few-shot examples are built and off (`-few-shot` / `GANGLION_FEW_SHOT=1`).
B1a. Seeing and fixing (user: couldn't undo a wrong verdict, and couldn't see the prompt): Coverage's model panel shows the prompt template (folded), and every suggestion and disagreement has **Why?**, which opens the exact messages sent, the model's reasoning and its answer, from the call log. Each disagreement has Use Model's, ✓ Fine and Clear Mine, so a wrong verdict of yours can be fixed where it shows up; the Judged list per template (sd/08/W9e) still works for any verdict.
B1b. Editing the prompt: it's `ganglion/internal/judge/prompts/judge.md`; an edit plus a deploy makes a new version, and calibration then shows old and new side by side.
B2. Not built yet: span suggestions (M3), the word helper's table (M4), rule ideas (M5), flagged-first sorting (M1 shows a highlighted border on flagged and low-confidence suggestions instead, so the list doesn't jump while you review).
B3. Measured: 8 samples judged in 11–16 s in parallel at low effort; a cold start adds ~20 s.

## Prompt versions

P1. `453f8cdca93e`: first version. Marked correct negative samples ("This ladder wasn't old.") as Means Something Else: it read the role line "age = old" as asserted and didn't understand that tags can deny it.
P2. `1b5ecc4381a3`: adds how to read a sample (the meaning says what it's about; tags set tense, affirm or deny, statement or question), with that example. Still flags some negatives with low confidence; needs real calibration data before more changes.

P3. `26eece049f8f`: adds a list of what tense and wording imply (facts that don't change, habit, refusal, this/that, other meanings). Much worse: 26 of 44 against v2's 40 of 44 at low effort. It said "implies refusal" for nearly everything ("That apple isn't old."), probably the list combined with "use the person's earlier notes word for word" and "implies refusal" being one of those notes; and it still passed the age ✕. Reverted to v2 the same day; v3's results stay in the log.
P3a. Lessons for the next try: change one thing at a time; don't push it to reuse notes word for word (offer them as wording to borrow only when they describe exactly the same thing); and add guidance aimed at the one gap (facts that can't change across tenses) rather than a list of everything.

## Guardrails

G1. Model output is never a verdict, a choice or a rule until the user accepts it; everywhere it appears it says it's the model's.
G2. Schema-constrained output, plus validation (reasons and choices from the fixed lists, notes length-limited, helper words in the lexicon). Anything that fails is shown as "the model's answer didn't fit" and logged.
G3. Timeouts and the cold start: the first call after idle takes ~10 s; pages say "model warming up" instead of spinning silently.

## Decided

D110. The model does the first pass over samples and the user reviews it; only the user's verdicts count. (user: "A first pair of eyes that I review (as the second).")
D111. Use the user's llm-server with gpt-oss-120b, configured from ~/work/llm. (user: "Yes.")

D112. One call per sample, in parallel (M1). Answers Q73. (user: "Yes, with caveats.")
D113. Few-shot examples from the user's verdicts: built, but **off to start**, behind a switch, because the prompt will need several rounds of iteration first. Answers Q74. (user: "Be prepared to turn this off, or maybe start with it off? I think we'll need to iterate a few times on the prompt.")
D113a. So prompts are versioned: each prompt lives in a file in the repo, its hash goes into every call record and model verdict, and calibration (M2) reports agreement per prompt version, so iterations can be compared.
D114. Accept one sample at a time; no accept-all until calibration shows high agreement. Answers Q75. (user: yes)
D115. Enable llm-server at boot. Answers Q76. (user: yes)
D116. Low reasoning effort for the first pass, compared against medium during calibration. Answers Q77. (user: yes)

## Open questions

(none for this note)

## Superseded

Q73 (answered by D112). One call per sample?
Q74 (answered by D113). Few-shot from the user's verdicts?
Q75 (answered by D114). Accept one at a time?
Q76 (answered by D115). Enable llm-server at boot?
Q77 (answered by D116). Low reasoning effort first?
