# Authoring templates (draft 1, 2026-11-15)

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

## Decided

D58. A word-by-word helper as in A8. (user)

## Open questions

Q36. Templates as small trees with feature-carrying slots, or flat strings? I lean trees from the start (flat strings are a one-node tree), since flat madlibs break on agreement and can't nest (A9).
Q37. Sentences with extra meaning (E2, E3): flag and ask what was added (A7)? I lean yes.
Q38. First frames to build: give, plus its neighbours from A8 (lend, send, receive) and cause-feel (sd/02/L6)? I lean give family first.
