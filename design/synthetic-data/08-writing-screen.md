# The writing screen (draft 1, 2026-11-15)

The ganglion page where the user writes sentences for meanings and turns them into templates (sd/03). Uses the design system as built (09/D42): cards, tags, Title Case labels, status never by colour alone.

## Layout

W1. One page, top to bottom (stacked the same on a phone):
  1. Meaning card (W2)
  2. Writing box (W3)
  3. Per-sentence review: spans and word helper (W4, W5)
  4. Live samples (W6)
  5. Save bar (W7)
W2–W7 below; each part works without the ones after it, so the user can stop at any point.

## Meaning card

W2. What to write for:
  - The frame name with its one-line explainer, e.g. **Has Property**: "something has a quality, like being old or red".
  - One row per role: plain role name, its filler as a chip ("Mara · person · she/her"), and an ⓘ that shows the role's explainer and its generic name (sd/02/D59).
  - Feature tags: Past, Statement, Positive. Modifier tags if present: Occasion: Tom's birthday.
  - Each role gets a series colour (09/C4), used again in W3 to underline its words.
W2a. Controls: **Next** (a new meaning, W8), **New Fillers** (same meaning, different people and things), **Change** (pick a feature or frame by hand).
W2b. Fillers are chosen to be distinctive so alignment is exact (sd/03/A1): unique names, unconfusable objects.

## Writing box

W3. The user types one sentence per line. Several lines are several realisations of the same meaning (sd/03/A2).
W3a. Alignment runs as the user types (on a short pause), in Go, with no model: it's fast enough (sd/03/D73). Each traced word gets an underline in its role's colour. Grammar words stay plain. Untraced spans get a dashed warn underline plus a ? mark, so it never depends on colour (09/C3).
W3b. Pasting is allowed and recorded as pasted (sd/07/E1).
W3c. Punctuation (user asked whether to type it): capitals and full stops are optional, since code adds them (sd/04/N2). "!" and "?" carry meaning. "?" is the question mood. "!" (emphasis, surprise) is extra meaning, so it's treated like an untraced word: highlighted, suggested as an attitude ("emphatic"), confirmed or dropped. A confirmed "!" stays in the template and its samples; a dropped one gives ".". Mid-sentence commas stay code's job, since they follow from structure. (The first mock-up silently turned "!" into "."; fixed.)

## Per-sentence review

W4. Spans: one row per untraced span (sd/03/A13c), with the A13d choices as a segmented control: Modifier, New Modifier, Event, Attitude, Drop. A model suggestion arrives a moment later and preselects one, marked **Suggested** so it's never mistaken for the user's choice. The user confirms or changes it. New Modifier opens a small form: name, type, explainer.
W4a. A sentence with no untraced spans shows ✓ Fully Traced.
W4b. Clicking the confirmed choice again clears it, back to unconfirmed (user: "I can't uncheck things from 'What Do These Add?'").
W4b1. Span choices that never reach a template (user: "spans with a selection but no template"): none in the saved data, since the one saved choice ("her" as owner) has its template. On the page they could pile up, because choices were kept by span text and never cleared, so a choice made for "today" under one meaning came back already confirmed under the next. Now a new meaning starts clean, and choices, context marks and paste flags for text that's gone are dropped.
W4c. Contractions: "that's", "it's", "the dog's old", "'re", "'m", "'ll", "'ve" and "'d" are split into the word and its verb, so they trace like the full form; a contraction is a style, so "The dog's old" builds the same template as "The dog is old". "'s" before a noun stays a possessive ("the dog's bone"). (user: "It doesn't understand contractions yet, so it gets confused by 'that's'.")
W4d. A line written as a question is built in statement order ("Is the apple old?" as "the apple is old"), and the generator does the inversion for every mood. Before, its samples came out as "Did Mara gives Noor a book." and "Will be the apple old." None of the user's saved templates came from a question.
W4e. "The dog is an old one": "one" stands for the holder's noun, said again. None of the five choices fits it, since it adds nothing; it needs a sixth, Refers Back (a pro-form for a role already there). Until then the shape can't be traced. (user: "What am I supposed to say about 'one' in your second shape?")
W4g. "and" joining two of the meaning's people or things is grammar, like "than": "Of Ana and Noor, Ana is older." (user: "What do I say for the 'and'?") Between clauses it would add meaning, so there it stays a span. Commas inside a sentence are now kept in the template; before, Build dropped all punctuation, giving "Of Ana and Noor Ana is taller."
W4h. Compare checks its form against the direction: "as old as" says the same, "older" says more or less, and either against the other kind of meaning warns, which holds the line back from Save. Before, "Ana is as old as Noor" was accepted for a "more" meaning.
W4f. "That is an old dog" (saved by the user) traces, but its tree keeps "That" and "an" as fixed words: "an" should fold into the dog's reference, and "That" is a pointing subject the generator can't invert for questions. A shape to fix, separately.
W5. Word helper (sd/03/A8): clicking any word opens its row: role, Same Meaning, Different Meaning, Breaks It. Generated by a model on request, not automatically, since it's slower and not always wanted. The user edits cells in place; the edits are vetting data (sd/03/A8c).
W5a. Clicking a word also gives "hey wait a minute" for its forms (sd/05/D74): it opens the lexicon entry.

## Live samples

W6. As soon as a sentence is traced, 8 samples appear below it, refreshed on every edit (sd/03/A16):
  - each sample is one line, with its meaning as small tags beside it (what differs from the authored meaning is highlighted)
  - samples spread across fillers, features, lemma classes, optional parts and nested templates (sd/03/A16a)
  - the seed stays fixed between edits so changes are easy to spot; **Fresh Draw** gets new ones
W6a. Each sample has ✓ and ✕. ✕ asks what's wrong, one tap: Word Form, Wrong Kind of Filler, Ungrammatical, Means Something Else, Other. Each answer goes where it belongs (lexicon entry, slot type, template, the A13 resolution) and is logged (sd/07/E1).
W6a1. Each reason has a one-line explainer (shown on hover, and under the chips once picked, so it works on a phone) and says where the verdict goes. (user: "The 'What's wrong?' buttons need explainers.")
W6a2. New reason **Sounds Odd**: correct and means what its tags say, but nobody would say it like that without more context. Found by the user in the mock-up: "This book will be old." It's grammatical and the meaning is right (has-property, future), but a future statement about a state reads oddly with no time anchor ("by next year this book will be old"), and it hints the book isn't old now. That's the pragmatic oddness risk (sd/00/X2), not a mock-up bug: the real generator would make it too. None of the first five reasons fit it.
W6a3. Where Sounds Odd goes: the template's supported features (sd/04/N1a). That combination is marked "needs a modifier" (here a time), so the generator either adds one ("by next year", "someday") or skips the combination. (Q70)
W6e. A third verdict, ◐ Fine, But… (user: some samples are hard to judge without context): right, but it needs context (a scene or an earlier sentence: "what could come before it?") or implies more than its tags ("what does it imply?"). The user can pick a note written before, most used first, or write a new one (user: "Maybe I'd want to see things I've already said and select one or write a new one?"). It counts as judged and doesn't hold a line back. Rows of those kinds will be tagged in datasets: a standalone-sentence set can leave them out, and a later set of sentence pairs can use the "what could come before" notes. Key: B.
W6d. Verdicts carry over (user asked whether a ✓ would still show next time): the server reads every verdict from the event log at start and keeps the latest per sentence text; a sample whose sentence was judged before shows that verdict, dimmed and tagged Judged Before. Keyed on the text alone, since a sentence is right or wrong whichever template made it, except Means Something Else, which only carries over when the tags match. Clearing a verdict clears it for good. Prior Ungrammatical / Means Something Else verdicts also hold a line back (W7d).
W6b. A sample can also be saved as a ★ favourite: a good example to show later, or a boundary case worth keeping (sd/02/D61).

## Coverage

W9. The Coverage page (user worried about missing things to mark as odd): one verdict covers every sentence of its kind, so what matters is whether every kind has been checked. For each saved template it lists the kinds of sample it can make, as dimensions: verb form (tense × polarity × mood, minus combinations the age rules skip), how each noun is referred to (the, a, this, that, minus "a" on a "be" subject), and each named role's pronoun set (she, he, they). Judged values show ✓, the rest ○, with a count.
W9a. A value counts as judged once any sample showing it has a verdict (✓ or ✕). The user's own sentence counts for its own kind.
W9b. Show Me draws 8 samples, each aimed at one unjudged value (tagged as the reason it's shown), the rest random. Verdicts are the same as on Write. Sounds Odd on a saved template makes a new template version that adds a time modifier for that tense.
W9c. Values are counted one at a time, not every combination: 22 kinds for "Ana gave Lee a chair" rather than hundreds. Pairs (e.g. negative × question × "they") would come later if one-at-a-time misses things.
W9e. Judged list (user: to make sure they didn't judge one too quickly): a Judged button on each template lists the samples judged so far, newest first, with verdict, reason, note and date, and each verdict can be changed there. Clicking a ✓ value filters the list to samples of that kind.
W9f. A later sentence with the same shape becomes a new version of the same template (sd/04/I1): "Lee is shorter than Noor" after "Ana is older than Noor". Coverage first matched verdicts by the template's current sentence only, so the user's earlier verdicts stopped counting (user: "I think 'Lee is shorter than Noor' replaced an older template I had already judged everything on?"). Now verdicts on samples of any sentence in the template's history count, and the card shows "also written as …".
W9d. Later, before any bulk export: a random audit (50 rows judged with none wrong means fewer than about 6% are bad, at 95%), and a model flagging likely-odd samples once one is connected.

## Save

W7. **Save Template** saves each fully traced sentence as a template version (sd/07/D87). The compact text form (sd/04/S1) is shown folded under each one for when the user wants to see the tree.
W7a. A sentence can't be saved with unresolved spans, but it can be **Saved as Draft**: kept with its meaning, to finish later. Nothing the user writes is lost.
W7c. When to save (user asked): fully traced, no ⚠ (the sentence says what the meaning says), and the samples read right at a glance; ✓ on every sample isn't needed. A ✕ for Ungrammatical or Means Something Else means the pattern itself is wrong, so reword instead. Word Form and Wrong Kind of Filler are fixed elsewhere (lexicon, slot type), so the template can still be saved. Same-shape sentences update one template, so saving twice doesn't inflate the count.
W7d. Save holds back lines with a ⚠ or a sample marked Ungrammatical or Means Something Else, and says why under Save; Save as Draft still keeps them. The server holds back ⚠ lines too.
W7b. Sentences using unsupported verb groups save frozen (sd/05/D80) with a tag saying what they wait on.

## What to write next

W8. **Next** picks a meaning from the coverage map's gaps (sd/03/A10): frame × feature combinations with the fewest templates, favouring the current frame family (sd/06/D81). The user can override with **Change**.
W8b. Next skips meanings already written for (any saved or drafted sentence with the same frame and fillers), walks every combination of fillers, attributes and both ends of each scale in a shuffled order, and starts somewhere random on each page load. Before this it offered "Ana, Noor, age more" on every visit (user: "it gave me Ana Noor age=more again"). The seed gained more people (Kai, Omar, Lin) and things (pencil, ladder, bottle, table); grammars seeded earlier get them, and the umbrella becomes a rod, at start-up as long as nobody has edited those objects.
W8c. **Enough for Now** on the meaning card: Next stops offering that frame, recorded as a `frame-skip` event so it lasts. The card lists skipped frames, each with a × to bring it back, and a frame picked by hand with Change shows Bring Back. If every frame is skipped, Next ignores the skips. (user: "I can't seem to think of more than one template for Compare, and Next keeps taunting me about it.") Next still goes to the frame with the fewest templates, so after skipping Compare it offers Located until that catches up.
W8a. Occasionally **Next** offers a deliberate joke or boundary meaning (sd/02/L8), marked as such, since humour and boundary tests are generated on purpose (sd/02/D61).

## Keyboard

K1. Keyboard-first, like the review queues planned in 07/A4:
  - Enter: new line (next realisation); Ctrl+Enter: save
  - Alt+N: next meaning; Alt+F: new fillers
  - Tab / Shift+Tab: move between untraced spans; 1–5: pick that span's choice
  - J / K: move between samples; Y / X: ✓ / ✕; R: fresh draw
K2. A ? key shows them all.

## Model calls

M1. Two model features, both through the one logged call path (sd/07/D90): span suggestions (automatic, W4) and word-helper tables (on request, W5). Neither blocks typing or alignment. If no model is running, the page still works: the user just picks everything themselves.

## Other screens (later)

O1. Frames and types: browse and create (sd/03/A15). Lexicon entry. Coverage map (sd/03/A10). Template list and history (sd/07/V4). Each gets its own note when it's built.

## Decided

D92. One page laid out as in W1. Answers Q64. (user: "Yes.")
D93. **Next** picks from coverage gaps by default, with manual override (W8). Answers Q65. (user: "Yes.")
D94. Alignment while typing (W3a). Answers Q66. (user: "Yes.")
D95. Span suggestions automatic, word helper on request (W4, W5). Answers Q67. (user: "Yes.")
D96. Keyboard keys as in K1. Answers Q68. (user: "Yes.")
D97. Next step: a working mock-up of this page in ganglion with hard-coded data, before any generator code. Answers Q69. (user: "Yes.")

D107. ◐ Fine, But… with Needs Context / Implies Something and pickable past notes (W6e). (user)
D106. A Judged list per template, filterable by kind, with verdicts changeable in place (W9e). (user)
D105. A Coverage page with Show Me, as in W9. (user: "Yes")
D104. Verdicts carry over to later sessions by sentence text (W6d). (user: "Yes")
D101. Save holds back lines with a ⚠ or a blocking verdict (W7d). (user: "Yes.")
D99. "!" is extra meaning, resolved like an untraced span (W3c). (user: "Yes.")
D98. "Sounds Odd" marks the template's feature combination as needing a modifier, and the generator adds one ("By next year, this book will be old."); it skips the combination only when no suitable modifier template exists (W6a3). Answers Q70. (user: "Yes, add one")

## Open questions

(none for this note)

## Superseded

Q70 (answered by D98). Sounds Odd: add a modifier or skip?
Q64 (answered by D92). One-page layout?
Q65 (answered by D93). Next picks from coverage gaps?
Q66 (answered by D94). Align while typing?
Q67 (answered by D95). Span suggestions automatic, word helper on request?
Q68 (answered by D96). Keyboard keys?
Q69 (answered by D97). Mock-up first?
