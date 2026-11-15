# Authoring templates (draft 3, 2026-11-15)

## Loop

A1. Code picks a meaning to write for, e.g. `give(giver=Mara, thing=book, recipient=Tom, tense=past)`, choosing distinctive fillers (unique names, unconfusable objects).
A2. The user writes one or more sentences for it (00/P2).
A3. Code aligns each sentence with the meaning: it knows the values it put in, so it finds them in the text and cuts them out as slots. Inflected fillers ("Mara's", "gave" for give) match through known lemmas; the slot is marked to inflect.
A4. Refill check: code refills the template with other values and shows 2–3 results; the user accepts or fixes.
A5. Models can stand where the user stands in A2: given a meaning and the user's realisations, propose more. Same alignment and check; the user vets templates, not sentences.

## Extra meaning

A6. The user's first examples, for `give(giver=Mara, thing=book, recipient=Tom, tense=past)`:
  E1. "Mara gave Tom a book." Exact realisation; clean template.
  E2. "Tom got <book-title> as a gift from Mara on his birthday." Same transfer from Tom's side ("got … from" is the converse of "gave"; fine, one frame can have templates from either viewpoint). Adds: purpose=gift, occasion=birthday, the book's title.
  E3. "Tom finally received the book (<title>) Mara sent him in the mail." A different event or two: send, then arrive. "Gave" doesn't imply mail, "sent" doesn't imply hand-to-hand. "Finally" adds that it was expected and late.
A7. When words don't trace back to the meaning, the tool flags them and asks what they add. Answers become optional parts of the meaning (occasion, means, title). Templates that use them are picked only when the meaning includes them. This is how the meaning language grows: from what the user naturally writes.

## Word-by-word helper

A8. The user asked for help turning examples into good templates: what each word is doing, what could replace it keeping the same or similar meaning, and what would change the meaning while staying grammatical. (user)
A8a. Per word: its role, then three columns. Example for E1:

| Word | Role | Same meaning | Grammatical, different meaning | Breaks it |
|---|---|---|---|---|
| Mara | giver (slot) | any person: Ana, she, my aunt | | the idea |
| gave | give, past | handed (adds: physically) | lent, sold, showed, threw | donated (no "donated Tom a book") |
| Tom | recipient (slot) | any person | | |
| a | book is indefinite (new to the reader) | | the, his | |
| book | thing (slot) | any object | | an idea (another frame, sd/02/L6) |

A8b. "Same meaning" gives filler classes and alternate words. "Different meaning" gives neighbouring frames to add next (lent = give + expected back). "Breaks it" gives constraints code can't guess.
A8c. A model proposes the table; the user corrects it. Corrections are vetted data too.

## Templates filling templates

A9. A slot asks for a kind, not a word: `{recipient:PERSON-REF}` takes a name template ("Tom"), a pronoun ("him"), or a description ("the man {who-clause}"), where the who-clause is a sentence template with one role turned into a gap. (00/P4)
A9a. Meaning composes the same way: the slot holds a referent, a description template adds facts about it, and the sentence's meaning is the merge.
A9b. Code controls depth and the mix of kinds; that's where variety comes from without new rows.

## Code-side views

A10. Coverage map: frame × features (past, negated, question, …) → number of templates. Gaps say what to write next.
A11. Template metrics: distinct templates per frame and per kind (00/P5).
A12. Feature-generic templates: a verb marked as a lemma slot lets one template cover every tense and polarity; code inflects.

## Finding extra meaning (how, for Q37)

A13. Steps:
A13a. Trace: code knows every value in the meaning and the forms each can take (names, lemmas and inflections, pronouns, wordings from existing templates and the A8 "same meaning" lists). It marks every word that one of those explains.
A13b. Grammar words: determiners and prepositions that a traced part needs ("a" for an indefinite thing, "to" in "gave a book to Tom") count as traced too.
A13c. Leftovers: whatever is left is untraced. The UI highlights those spans. In E2: "as a gift", "on his birthday", the title. In E3: "finally", "sent … in the mail", "received".
A13d. For each span, a model proposes one of these and the user confirms or changes it:
  1. an existing modifier fits (occasion, purpose, means, time)
  2. a new modifier: the user names it, picks its type, writes a one-line explainer
  3. a separate event: "sent … in the mail" is send, so the template realises two events (send, then arrive)
  4. an attitude or expectation: "finally" (expected, late). Probably a modifier kind of its own.
  5. drop it: cut the span from the template
A13e. A different viewpoint ("Tom got … from Mara") isn't extra meaning. The first time, the helper proposes it as the converse of give; after the user confirms, it's a traced form.
A13f. When nothing is left untraced, the template's meaning is exactly the meaning plus the confirmed additions, and the refill check (A4) runs.

## First frames (options for Q38)

A14. Families to start from:
  a. Transfer: give, lend, send, receive, sell. Clear roles, word-order alternations ("gave Tom a book" / "gave a book to Tom"), converses (A13e).
  b. States and properties: X is Y, X has Y, comparisons ("taller than"). The simplest frames, and they make the descriptions other frames need ("the tall man", the PERSON-REF slots of A9).
  c. Motion and placement: go, come, put, move. Places and paths, lots of prepositions; leads toward a world model (sd/00/T10).
  d. Communication: say, tell, ask. Quotes, embedded questions, mood (sd/02/L4).
  e. Perception and belief: see, know, think, believe. Embedded clauses, so templates fill templates early (A9).
  f. Feelings: scare, please, cause-feel (sd/02/L6). Abstract types; good ground for coercion humour (sd/02/L8a).
  g. Everyday scripts: eat, buy, cook. Lots of world knowledge, so the strongest test of types (eat needs food).
A14a. I lean b then a: descriptions are needed inside every other frame, and transfer is the clearest frame family to grow from E1–E3.

## Bootstrapping in the UI (sketch for D60)

A15. With no frames yet, writing starts from a frame, not a meaning:
  1. New frame: the user types a word ("give"). The tool shows VerbNet/FrameNet candidates for that sense, with their roles and explainers, to borrow names from (sd/02/D59, D60), or the user starts blank.
  2. The user sets roles (name, type, one-line explainer). Types come from a picker over the type tree, or a new type gets added there.
  3. Code makes the first meaning with distinctive fillers (A1), and the loop runs from A2.
  4. Each untraced span resolved as option 3 in A13d offers to create that frame on the spot, the same way.

## Live samples while editing

A16. While the user works on a template, the UI generates samples to look at, to make sure they seem right, and generation is fast. (user: "it should be able to generate samples for me to look at to make sure they seem right. The generation should be fast.")
A16a. Samples regenerate on every edit and spread across what can vary, so one glance covers it:
  - fillers (different entities, plural vs singular, he/she/they)
  - feature values in `supports` (past, negated, question, …; sd/04/C1)
  - each lemma in a `lex` class (sd/04/D71)
  - optional parts present and absent (sd/04/N1c)
  - reference choices, and slots filled by nested templates (sd/04/F4, sd/03/A9)
A16b. Each sample shows its meaning beside it, so the user checks the text against what it should mean, not just whether it reads well.
A16c. A sample marked wrong is vetting data, like the refill check (A4): it points at the slot type, feature or lemma that caused it.
A16d. Fast means no model calls: generation is plain code over the tree, type tree and lexicon. A fixed seed per view keeps samples stable between edits so changes are easy to see, with a button for a fresh draw.
A16e. I'd build the generator in Go inside the ganglion server, so a preview is one request with no worker round trip; the same code does bulk generation later. Target: a batch of samples well under 100 ms. (Q46)

## Decided

D58. A word-by-word helper as in A8. (user)
D62. Templates are trees, all the way down. Answers Q36. (user: "Fine with me. It's trees all the way down, my friend.")
D63. Flag extra meaning and ask what was added. Answers Q37. (user: "Yes, but how?" Mechanism proposed in A13; Q39.)
D64. First frames: states and properties, then transfer (A14a). Answers Q38. (user: "Sounds good.")
D65. The A13 mechanism, including the A13d choices. Answers Q39. (user: "Yep, sounds good.")
D72. Live samples while editing a template, and fast generation (A16). (user)
D73. The generator is Go inside the ganglion server, one code path for previews and bulk generation; target a sample batch well under 100 ms (A16e). Answers Q46. (user: "Yes, please.")

## Open questions

(none for this note)

## Superseded

Q46 (answered by D73). Generator in Go inside the ganglion server?

Q38 (answered by D64). First frame family (A14)?
Q39 (answered by D65). Does the A13 mechanism look right?

Q36 (answered by D62). Templates as small trees or flat strings?
Q37 (answered by D63). Sentences with extra meaning: flag and ask what was added?
