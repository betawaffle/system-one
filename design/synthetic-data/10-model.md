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

## Narrow checks (the Jev route)

J1. Why: one open question ("is this okay?", five verdicts, six reasons) is hard for a local model; narrow yes/no questions suit it far better (sd/00 jev). The task is whether a sentence makes sense, which mostly has a right answer, so each check is a question with a fact behind it.
J2. Checks, one call each, run in parallel; each answers yes or no, quotes the words involved, and says why in under 12 words:
  - **form**: is any word spelled or inflected wrong? → ✕ Word Form
  - **grammar**: is it broken English? → ✕ Ungrammatical
  - **filler**: can each thing really fill its role, or have that property? → ✕ Wrong Kind of Filler
  - **meaning**: does it say something its tags don't, or leave something out? → ✕ Means Something Else
  - **change**: does the tense suggest the fact is different at another time, and if so, could it really be different? Only an impossible change counts → ✕ Other ("can't change"). The gap the single prompt never caught (P3a).
  - **implies**: would a reader infer more than the tags say, beyond what's already recorded? → ◐ implies
  - **context**: would it be natural as the first sentence of a conversation? No → ◐ context
J3. Code makes the verdict, never the model: the first ✕ check in that order wins, then implies, then context, else ✓. Its note is the deciding check's reason. Confidence is the deciding check's, or for ✓ the lowest across checks.
J4. The person's earlier notes go only to implies and context, as wording to borrow when it describes exactly the same thing (P3a).
J5. Version: the hash of the checks prompt file plus the combining rules' version, so calibration compares it against v2 like any prompt.
J6. Shown: the model line says which check decided; Why? opens that call, with every check's answer listed to switch between them. Calibration adds, per check, how often it fired on your ✓ (false alarms) and on your ◐ and ✕.
J2a. Change is asked only for state frames; for actions ("Did Lee give…") code answers no without a call.
J7. Cost: seven calls per sample instead of one, and with 16 running at once each takes ~75 s, so 44 samples take ~25 min (v2: a few minutes) and a Write preview of 8 samples waits ~1–2 min.

P4. `20fb3318f9a9` (checks-1): **9 of 44** (✓ 8/39, ◐ 1/1, ✕ 0/4). False alarms on your ✓: implies 27/39, context 21/39, change 4/39, meaning 3/39, form 1/39. Implies counted the literal meaning (a negative implying the opposite, "his lamp" implying ownership, a question implying the speaker doesn't know); context said any named thing "needs prior mention"; meaning read "positive" on a question as a mismatch. Change saw the tense right on all four age ✕ ("future tense implies not older now") and then said the age order *can* change, despite that example in the prompt.
P5. checks-3: implies and context tightened to exclude the literal meaning and to accept names, "this", "that" and possessives as openers (your ✓ include "That apple is old."); meaning told that a positive question isn't a mismatch; change skipped for actions, and made to write out the opposite fact first ("then") before judging whether it can change, with the reason ages keep their order. Caveat: that reason is close to the very ✕ cases being measured, so a catch here proves less than it looks; fresh ✕ samples are needed. Its run was stopped part way (user: "Just kill this run, and don't start the next one until we have a better plan in place.").

P6. Jev `jev-6b03606e` (jev-1.14.0), minimal state (the sentence and its meaning as flat fields), four noul checks flagged below 0.5, on the 40 samples marked ✓ or ◐ plus 9 broken in code (J11): **no false alarms on the 40, and 7 of 9 broken caught by the right check**, ~80 ms per request. Lowest value on a good sample, against the broken ones:
  - grammar: good ≥ 0.90; broken 0.16 (gived), 0.04 (more older), 0.50 (verb dropped). A threshold of ~0.75 would catch all three.
  - meaning: good ≥ 0.63 (negative questions lowest); broken 0.02 (swapped roles), 0.09 (recipient dropped), 0.38 (present for past), 0.11 (negated). Clear, though the gap is narrower.
  - filler: good ≥ 0.57 ("That chair is tall."); broken 0.15 (an idea giving), 0.59 (an idea older than Noor, a weak case since ideas can be old). Overlaps; the least reliable.
  - context: good ≥ 0.87; not tested, since nothing in J11 needs context.
  - Not tested: the age-change ✕ (left to rules, J14 discussion), Word Form apart from "gived".
P6a. The user found that Jev knows "the relative age of two things doesn't change" only weakly (72%), and the answer flipped with rewording, so facts about how attributes change go in the meaning language, written by the user, not asked of a model.

## Plan before the next run

J8. Measure first: the same check prompt at 1, 2, 4, 8 and 16 at once, about 50 short calls (~2 min). With 16 at once the server managed ~20 tok/s in total, below one call alone (~52 tok/s), so the fastest setting is probably far lower. Cap ganglion's parallel calls there (it affects v2 too).
J9. One call per sample, with every question as its own field in a fixed order, each with its words and why; code still makes the verdict (J3). Seven times fewer prompts, at the cost of the checks seeing each other's answers. Drop Word Form: our inflection code makes those forms, and it flagged 1 of 39 and caught nothing.
J10. A small set to iterate on: the 4 ✕, the ◐, and ~10 of your ✓ chosen to cover the traps seen so far (negative, question, "this"/"that", possessive, past action). A run takes a minute or two; the full 44 only when the small set looks right.
J11. ✕ cases made by code, labelled by construction, so the test isn't only the four age samples the prompt was tuned on: a wrong inflection ("gived"), a wrong kind of filler (an idea as holder of height), a swapped role (giver and receiver), a dropped word, a tense that implies an impossible change on a different attribute. Kept apart from the prompt: never used as its examples.
J12. Until checks beat v2 on both sets, Write and Coverage go back to v2, so previews stay fast.
J14. Jev as the baseline (user: "Maybe we try using Jev as a baseline?"). Jev (typesafe.ai) takes a *state* plus typed questions in one request, answers them in parallel, with probabilities and a confidence, no reasoning. That is J9's shape exactly. Its pieces:
  - State: the sample card the local model sees now (meaning, roles, the authored sentence, the sample, its tags, what's already recorded).
  - Questions, all in one request: each check as a yes/no Choice with a description per option, plus one direct verdict Choice (✓, ◐ context, ◐ implies, and the six ✕ reasons) for comparison.
  - Code makes the verdict from the checks (J3); the direct verdict is shown beside it. Calibration gets a third column, and the probabilities give a threshold to tune instead of low/medium/high.
  - Logged like every model call (CAPTURE-REQUIREMENTS): what was sent, what came back, the model version from the response (`jev-1.x`), and tokens.
  - Needs: an API key (`TYPESAFE_API_KEY`, from console.typesafe.ai), and the user's OK to send samples out. They're generated sentences with seed names, nothing personal. Price per the site: $42 per billion input tokens; 44 samples is ~25k tokens, about a tenth of a cent.
  - −: an outside service, in early access; we can't see why it decided (no reasoning to read under Why?).
J13. Recommendation: J8, then J12 at once, then J9 + J10 + J11 together, with one calibration on the small set, and the user looks at the result before any full run.

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
D117. Try narrow checks (J1–J6) in place of the single prompt, and keep them only if calibration beats v2. (user: "Basically going the Jev route, but seeing if we can use a local model for it to save cost and get more data and control." … "let's try the jev thing." Also: "in this case I'm just trying to figure out if sentences make sense", not taste.)

## Open questions

Q78. Plan J8–J13 as recommended in J13? In particular, is J11 (✕ samples made by breaking good ones in code) a fair test, or do you want ✕ cases you write yourself?
Q79. Back to v2 on Write and Coverage now (J12), before anything else?
Q80. Jev baseline (J14): where's the API key kept (I lean an env file read by the ganglion service, outside the repo), and OK to send the 44 judged samples plus the J11 broken ones? I lean running it on the small set (J10) first, the same as the local model.

## Superseded

Q73 (answered by D112). One call per sample?
Q74 (answered by D113). Few-shot from the user's verdicts?
Q75 (answered by D114). Accept one at a time?
Q76 (answered by D115). Enable llm-server at boot?
Q77 (answered by D116). Low reasoning effort first?
