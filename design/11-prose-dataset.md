# Prose dataset: who said what (draft 1, 2026-11-14)

Ask (user): a dataset from everything either of us has said in transcripts, plus any other known prose; "high quality realistic prose". Follows 10/R9 (the command inverter can't read prose).

## Utterances (built)

U1. New event type `utterance` in ganglion's store: speaker, kind, model, tag, text; content-addressed like commands, every sighting kept (source, unit, turn, part, time). Browse at /utterances.
   - speaker: human (the person in our sessions), claude, subagent, model (other models), author (third-party people), mixed (documents with more than one author), harness
   - kind: typed, pasted, injected, summary, reply, thinking, prompt, document, commit, issue
U2. Who said what comes from structure, never from guessing (capture rule 3):
   - Claude Code marks typed messages with `origin.kind = "human"`; transcripts from before that field count plain user text as typed.
   - Known harness tags (`<system-reminder>`, `<task-notification>`, `<command-*>`, `<local-command-*>`, …) are cut out as the harness's, each tagged with its mechanism; so are skills, tool loads, interrupts and image notices.
   - `<pasted_content>` is the person's, kind pasted, apart from typed.
   - Relay wrappers (`<wake>…<message from="human">`) are unwrapped; mid-turn messages are pulled out of their wrapper.
   - Compaction summaries are model-written: claude/summary. Prompts to subagents are claude/prompt; subagent text is subagent.
   - tack is read through tack-show -full: user, system prompt (harness), model reasoning and replies; tool calls and output dropped.
   - Documents: Markdown in our repos, design notes, memory, HANDOVER; authorship from git (all-Claude → claude, else mixed); files outside git are Claude-written.
   - Commits: author from git.
U3. Splits: 1 in 10 units (session, conversation, document, trajectory) held out as test, for every source; our own text is training data here, unlike local commands (06/G7).
U4. Imported 2026-11-14 (distinct texts): you typed 757 (313 KB); my replies 1,456 (1.2 MB), thinking 1,218 (1.1 MB), documents 74 (311 KB), summaries 13, commits 144; harness 479 (402 KB, not anyone's words); other models (tack, Pi, Open-SWE) 26k replies and 126k reasoning blocks (226 MB); PR authors 2,499 issues (6.2 MB).

## Decided

D45. Include my thinking: "it's prose, grammatical, etc." Answers Q30. (user)
D46. Include tack and Pi: your messages, and other models' replies under their own speaker. Answers Q31. (user: "yes")
D47. Include any other known prose; aim for high quality, realistic prose. (user)

## Prose sentences (built, py/build_prose.py)

P1. Kinds that are someone's own prose (typed, pasted, reply, thinking, summary, document, commit, issue, prompt); never harness text.
P2. Markdown structure that isn't prose is dropped: fenced code, headings, tables, tags, indented blocks, shell lines. Quoted lines (`>`) are dropped: in your messages they're usually my words quoted back.
P3. Sentences by pysbd; then computable rules reject what clearly isn't a sentence (20–300 chars, 4–80 words, mostly letters, mostly plain words, little code punctuation). Judging quality beyond that is a model's job (00/R1); a classifier could come later.
P4. Ours kept in full; other speakers capped per speaker/kind (default 150k sentences) so they can't drown ours. Exact duplicates collapse; a sentence in train never appears in test.
P5. Output: ~/.local/share/ganglion/datasets/prose-v1/{train,test}.jsonl with provenance per sentence, and stats.json. Never in the git repo.
P6. First build showed fragments from hard-wrapped list items and choppy agent notes ending in ":". Fixed: wrapped list items rejoin, and a sentence must end like one (. ! ? with optional closing quote, bracket or backtick).
P7. prose-v1 (2026-11-14, 8 minutes, mostly segmenting Qwen's 226 MB of reasoning):
   - train 202,098 / test 26,576 sentences
   - you: 2,356 train / 425 test (from 757 messages)
   - me: 21,956 / 2,645 (replies 11.9k, thinking 8.3k, documents 2.6k, summaries 1.4k, commits 0.6k, prompts)
   - PR authors: 22,602 / 2,965; other models: 155,180 / 20,541 (Qwen reasoning capped at 150k of 1.9M)
   - 10k exact duplicates dropped
P8. Quality by eye: your sentences and mine read as clean prose; Qwen's reasoning is realistic but choppy (asides, half-thoughts, code-heavy); PR text mixes good prose with API-doc fragments. Ours is small: ~27k sentences, under 15% of the set.

## Public prose: fineweb-edu (2026-11-14)

F1. The reader project's "real text is test-only" rule was about quality: the user wants to validate quality themselves. So fineweb-edu may be training data here, marked as its source. (answers Q32)
F2. Imported a deterministic 1-in-15 sample by sha256(document ID) of the on-box shard (sample/10BT, 000_00000, 726k documents, ~3.5 GB): 48,191 documents, 231 MB, speaker `web`, kind `document`, source `fineweb-edu/sample-10BT/000_00000`, with URL, crawl dump, fineweb's educational score (2.5–4.6 in this shard), language score and token count. Held out 1 in 10 by document. The rest of the shard can follow if wanted.
F3. Per the corpora README, fineweb text is never printed into a Claude session: I look at counts and statistics only, and the user judges quality on the Samples page.
F4. prose-v2 = prose-v1's rules plus fineweb (capped at 150k sentences like other non-ours sources). prose-v1 stays as it was.

## Samples page (built)

S1. /samples: random draws from any dataset (utterance sources, command sources, derived dataset files), filterable by speaker and kind, 5–50 at a time. Each sample shows its source, speaker, kind, model, split, and for fineweb its score and URL; long texts are cut at 6,000 characters with a link to the full one.
S2. The seed is in the address, so a draw can be revisited or shared; "n" draws again. New datasets appear automatically (any datasets/<name>/<split>.jsonl, any new source).
S3. Possible next: rating samples as you read them (good / bad / not prose), stored as labels with the sample's provenance (07/A4). Not built; behaviour to agree first.

## Laundering: quotes and the blacklist (2026-11-14)

L1. Found by the user: I read a fineweb document aloud on request, so its text sat in a claude/reply and would have entered the dataset as my prose ("now it has been laundered into your output"). I'd also edited it lightly for speech ("Ms. Manners" → "Miss Manners", reflowed list), so exact matching alone misses parts.
L2. Blacklist: <data>/blacklist.jsonl, append-only, one entry per utterance (hash, reason, when, by); a later entry for the same hash replaces an earlier one. Only Claude's own output (claude, subagent) can be listed. `ganglion blacklist add HASH-PREFIX REASON [-by NAME]`, `ganglion blacklist list`. Blacklisted utterances contribute nothing to derived datasets, and their page says so.
L3. First entry: d711574bd3 (the read-aloud reply), at the user's request.
L4. Quote detection in build_prose.py (rules r2):
   - exact: a sentence of ours whose normalised text (lowercase, letters and digits) also appears from another source is a quote, dropped from ours; between the two of us, the earlier speaker keeps it.
   - paraphrase: a sentence of ours sharing ≥ 60% of its 4-word shingles with one sentence from outside (web, author, model) is dropped too. Not applied between you and me: I restate your words all the time when recording decisions, and those restatements are mine.
   - every drop goes to datasets/<name>/quotes.jsonl with what it matched, so it can be reviewed on the Samples page.
L4a. Tuned on prose-v2 (2026-11-14):
   - short common sentences matched by coincidence (your "Am I missing something?" also appears in a PR): quotes need ≥ 8 words; paraphrases ≥ 10 words and ≥ 75% shared shingles.
   - direction: in tack, DeepSeek quoted you, and your original was being credited to it. Within one conversation the earlier turn owns a sentence and the later one is the quote (dropped from the other speaker instead).
   - result: 41 drops, 40 of them me quoting you verbatim; the last one is Qwen repeating a tool description I wrote, in another conversation (left as is).
L4b. With the blacklist switched off (--ignore-blacklist), quote detection alone catches 14 of the read-aloud reply's 26 sentences (13 exact, 1 paraphrase). The other 12 are my framing, short sentences under the length limits, or lines I reworded for speech. So detection is a backstop, and the blacklist is what removes a whole reproduced text.
L5. Sentences are cached per utterance hash and rules version (datasets/.cache/sentences-r2.sqlite) and segmented in parallel, so rebuilds only segment new text.

## Ratings and document features (2026-11-15)

R1. Rating on the Samples page: Good / Bad / Not Prose per sample, by button or key (j/k move, 1/2/3 rate, 0 clears, pressing the current rating again clears it; rating moves to the next sample). Labels are append-only in <data>/labels.jsonl: target, rating, who (name claim from GANGLION_USER, set to andy in ~/.config/ganglion/env), when, the dataset and seed it was drawn in, what the page showed (source, speaker, kind, line, document features), and the client. Targets: u:<utterance hash>, c:<event hash>, s:<sha256 of the sentence> so sentence labels survive rebuilds. A later label replaces an earlier one for the same target and person. Labels are never in the rebuildable index.
R1a. The label API only takes same-origin JSON (a form post or a cross-site request gets 403). Tests run against a separate instance with -user claude-test, so no test label is ever recorded under the real labeller's name; four such labels exist.
R2. User's example of fineweb's quality (0ee47e132e): "missing spaces after periods, there's no structure to how it's formatted… it's awful". fineweb scored it 3.5 (int 4, above this shard's mean of 3.0). Measured without reading it: 31% of lines end like sentences, repeated lines; 4 of its sentences had entered prose-v2.
R3. Document features shown on each document sample, and saved with its label: missing-space joins ("word.Next"), share of lines ending like sentences, repeated lines, word variety. Software measures; the person, and later a model, judges.
R4. Sentence rule r3 rejects missing-space joins. Across the fineweb sample: 45,484 documents have none, 2,240 have 1–2, 363 have 3–9, 104 have 10 or more. The user's example has none of this form, so its missing spaces take another shape (not yet identified; text not read here, per F3).

## Decided

D51. Ratings on the Samples page, saved as labels. (user: "Yes, add ratings")

D50. Detect quotes and paraphrases of other sources in our text, and keep a blacklist for Claude's own output; the read-aloud reply is its first entry. (user: "Add it and also implement a blacklist for your own output, then add this to the blacklist.")

D48. fineweb-edu may be used for training, marked as its source; the user validates quality themselves. Answers Q32. (user: "The rule about testing is just for quality purposes. I want to validate quality myself.")
D49. A page to view random samples from fineweb and future datasets. (user)

## Open questions

(none for this note)

## Superseded

Q32 (answered by D48). Public prose for volume and quality. On the box: one fineweb-edu shard (2.15 GB, educational web text, quality-filtered) in /data/corpora, but its README records your rule from the reader project: "real, dirty text is for TESTING only". Does that rule apply here? Other options: Wikipedia (CC BY-SA), Project Gutenberg (public domain, older style), Ettin's own decay-phase data (textbooks, papers; Ettin has seen it). I lean fineweb-edu as the main public source if the rule doesn't apply, since it's realistic modern prose already filtered for quality, plus a Wikipedia slice.
