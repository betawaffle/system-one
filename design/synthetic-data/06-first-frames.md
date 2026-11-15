# First frames: states and properties (draft 2, 2026-11-15)

The first frame family (sd/03/D64). These frames give the descriptions every other frame needs inside its REF slots (sd/04/N3), so they come before transfer.

## Frames

F1. `has-property(holder: physical, property: P)`: "The book is old." "Mara is tall."
F2. `is-a(member: entity, category: type)`: "Mara is a doctor." "Rex is a dog." The category is a type from the type tree (sd/02/L1) or a role noun (doctor, teacher).
F3. `own(owner: person, thing: physical)`: "Tom has a book." "The book is Tom's."
F4. `part-of(whole: physical, part: physical)`: "The chair has four legs." "The cup has a handle."
F5. `compare(a: entity, b: entity, attribute: A, direction: more | less | same)`: "Mara is taller than Tom." "The book is as old as the chair."
F6. `located(thing: physical, place: physical, relation: on | in | under | next-to | …)`: "The book is on the table." A static state, so it belongs here rather than with motion.
F7. "have" is several frames, like "give" (sd/02/L6): own (F3), part-of (F4), kinship ("Mara has a sister") and attributes ("Mara has red hair"). Kinship and has-attribute come later.
F8. These frames are also really useful for a model to understand: they are facts about things. (user) So the data serves both directions: generating text from facts, and training a model to read the facts back out of text, with the meaning as an exact label.

## Properties

P1. A property is an attribute plus a value: `age=old`, `size=big`, `colour=red`, `material=wood`.
P2. Each attribute says which types it applies to. That's the type check for adjectives: "the idea is tall" breaks it (sd/02/L8b), and a coerced version is a joke (sd/02/L8a).
P3. Gradable attributes have values on an ordered scale (young < old; small < big), which is what F5 compares and what degree words modify ("very old", "a bit small"). Non-gradable ones (material, alive/dead) don't take degrees; "very wooden" is a boundary test (sd/02/D61).
P4. Words for a value can depend on the holder's type: age of a person is young/old, age of an artifact is new/old. The value is the same meaning; the lexicon picks the word by type.
P5. "Not old" and "new" are different meanings: negating a property (polarity on F1) vs stating the opposite value. Language keeps them apart ("not old, but not new either"), so the meaning language does too.
P6. Seed: WordNet links adjectives to attributes (old → age, big → size) and records antonyms. That gives attributes, values and scales from a permissive source (sd/05/D75).
P7. Starting attributes: age, size, height (for people and buildings), colour, material, condition (broken, clean), temperature (food and drink).

## Descriptions

R1. Every F1–F6 fact can also be said inside a REF, not just as a sentence. That's how one entity's facts ride along in other frames:
  - property: "the old book"
  - is-a: "Mara, a doctor,"
  - own: "Tom's book", "the book of Tom's"
  - part-of: "the cup's handle", "the cup with a handle"
  - located: "the book on the table"
  - any of them as a REL: "the book that is old" (derived, sd/04/D68)
R2. A REF's meaning is the facts it realises, merged with the sentence's (sd/04/M2). "Tom's old book is on the table." = own(tom, b1) + has-property(b1, age=old) + located(b1, on, t1).
R3. Adjective order is a rule code applies: opinion, size, age, shape, colour, origin, material, purpose ("a lovely little old red wooden box"). Getting it wrong sounds off at once, so it's a good early check of the generator.
R4. Degree: a property value can carry a degree (`age=old, degree=very`). DESC templates realise it ("very old", "really old", "a bit old").

## Found while writing

P8. The user's note on "In a few years, Ana will be older than Lee." (an Other verdict, 2026-11-15): "Age differences between people never change." So for age, which of two people is older is fixed for life: if Ana is older than Lee now, she always was and always will be. Changing the tense of that comparison doesn't make a new meaning, it makes an odd one, and the Sounds Odd fix (a time modifier) made it worse, not better.
P8a. Proposal: an attribute can say its order between two holders is stable (`stable_order: true` for age). Comparisons on a stable attribute are generated in the present only (or the tense of the authored sentence), and Sounds Odd doesn't offer a time modifier for them. Height, size and weight can change, so they keep all tenses. (Q72)
P8b. The user's nuance: "X will be younger than Y" is fine if X isn't born yet, and the past tense is generally fine too, but it implies the person has died. So other tenses aren't wrong, they carry extra meaning about existence: future implies X doesn't exist yet, past implies X (or Y) no longer does. Those need entities with an existence state, which is the world model (sd/00/T10). Until then, stable-order comparisons are generated in the present only.

P9. The user asked whether the age rule should apply to "this apple is old". Not the stable-order one (P8): one holder's age does change, so "wasn't old" and "will be old" are real meanings. But age only goes up, which gives a neighbouring rule. For the high value, "was old" implies the holder is gone (it's older now), and "won't be old" implies it won't last. For the low value, "will be new" can't happen. Other combinations are fine.
P9a. An attribute can say `direction: increasing` (age). Those combinations are skipped by the generator until the world model can state existence, as in P8b. The rule sits on the attribute and applies at generation time, so templates saved before or after it are the same.

P10. "Umbrellas aren't tall, they are long." (user, from a meaning card offering "umbrella, height = tall"). Which dimension a thing has depends on its shape, finer than "artifact". Fix: a new type `rod` (long, thin made things: umbrella, pencil) under artifact; attributes can list types they don't apply to (`except`), so height skips rods; a new attribute `length` (short/long) covers them. Also: age = young is "fresh" for food, now that Next offers both ends of a scale.
F6a. Built (2026-11-15; user: "Let's do F9", the chat's name for this frame): `located(thing, place, relation)`, title Located, a state with the verb be. The relation is an attribute of a new kind, `relation` (`position`: in, on, under, near), held by the place: its word depends on the place's type the way property words depend on the holder's (P4), and a value fits a place only where it has a word for it. In: containers and places. On: surfaces. Under: surfaces and containers. Near: anything placeable.
F6b. New types under artifact: `container` (box, cup, bottle) and `surface` (table, chair); and `place` under physical (park, kitchen, garden, house). Samples and New Fillers used to swap a filler only for one of exactly the same type, which after this retyping would have stopped a box swapping with a chair. They now swap within the same kind (person, animal, artifact, food, place…), still only where the attribute or relation fits.
F6c. Next picks a thing that isn't a person or a place, a place some relation fits, and a relation with a word for that place.
F6d. Found at once: sizes. Next offers "the chair is in the bottle". Containing and supporting depend on relative size, which nothing records yet. (Q81)
P10a. This is the type system growing from what the user notices (sd/02/D60). Doing it by hand for now; editing types and attributes in the UI is T18.

## Starting types

Y1. A small tree, grown as the user writes (sd/02/D60):
```
entity
  physical
    person
    animal (dog, cat, bird)
    artifact (book, chair, table, box, cup, house)
    food (apple, bread, soup)
  abstract
    idea
    feeling
```
Y2. Enough for samples to vary in meaningful ways: people for F3 owners, artifacts with parts for F4, places to put things for F6, food for temperature.

## Examples

E1. has-property(b1, age=old): "The book is old." / "The book isn't old." / "Is the book old?" (sd/05/V1)
E2. compare(mara, tom, height, more): "Mara is taller than Tom." / "Tom is shorter than Mara." (a converse viewpoint, sd/03/A13e) / "Mara isn't as tall as Tom." (direction less, said with "not as")
E3. located(b1, on, t1) + own(tom, b1) + has-property(b1, age=old): "Tom's old book is on the table." / "The old book on the table is Tom's." / "On the table is Tom's old book."

## Future topics

T18. Editing types, attributes and entities in the UI, so findings like P10 don't need code.

T13. Superlatives ("the tallest"): compare against a group, so it needs groups of entities first.
T14. Generics ("Dogs are animals", "Books have pages"): statements about kinds, not individuals.
T15. Existentials ("There is a book on the table", "There are two"): a viewpoint on located, with its own agreement quirks.
T16. Numbers and quantities ("four legs", "two books"), and numeric values for attributes once a world model exists (sd/00/T10).

## Decided

D109. Rods (umbrella, pencil) have length, not height; attributes can have exceptions (P10). (user: "Umbrellas aren't tall, they are long.")
D102. Age only increases; the generator skips "was old", "won't be old" and "will be new" (and their questions) until the world model (P9, P9a). (user: "Yes")
D100. Age has a stable order; its comparisons are generated in the present only for now, and Sounds Odd adds no time modifier to them (P8a). Past and future come back with the world model, carrying their implied meaning: not born yet, or no longer alive (P8b). Answers Q72. (user: "Proposal sounds fine. Slight nuance: "X will be younger than Y" would be _ok_ if X is a baby that isn't born yet. And past tense is generally fine too, but kinda implies the person died.")
D81. First frames F1–F6; kinship and has-attribute later (F7). Answers Q54. (user: "Yes.")
D82. Properties as attribute + value, with per-type domains and ordered scales seeded from WordNet (P1–P6). Answers Q55. (user: "Yes.")
D83. "Not old" and "new" are different meanings (P5). Answers Q56. (user: "Yes.")
D84. Adjective order applied by code (R3). Answers Q57. (user: "I guess so, not sure I understand what you're asking. What is the alternative? Oh, yeah, ok, this is fine.") The alternative would have been each template spelling out its own adjective order.
D85. Starting types and attributes as in Y1 and P7, grown as the user writes. Answers Q58. (user: "Yup, sounds good.")

## Open questions

Q81. Sizes for Located (F6d). I lean a rough size class per entity (small: pencil, cup, apple, book; medium: chair, box, dog, lamp, umbrella; large: table, ladder, person; places are larger than all), with in and under needing the thing smaller than the place, and on needing it no bigger. Alternatives: let verdicts mark them Sounds Odd until the world model, or drop "in" for cups and bottles.

(none for this note)

## Superseded

Q54 (answered by D81). First frames F1–F6?
Q55 (answered by D82). Properties as attribute + value?
Q56 (answered by D83). "Not old" and "new" as different meanings?
Q57 (answered by D84). Adjective order by code?
Q58 (answered by D85). Starting types and attributes?
