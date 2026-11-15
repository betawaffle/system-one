# Meaning language (draft 2, 2026-11-15)

A small typed language for meanings. "Typed": code checks every slot against a type, and that check is what keeps sentences meaningful rather than only grammatical. "Small": a type, frame or modifier is added only when a sentence the user wrote needs it.

## Parts

L1. Types: kinds of things, nested.
```
entity
  physical
    person
    artifact
      book
  abstract
    idea
    feeling
```
A slot that asks for `person` accepts Mara; one that asks for `physical` accepts Mara or a book.

L2. Entities: the particular things in one meaning.
```
mara: person, name "Mara", female
tom:  person, name "Tom", male
b1:   book, title "…"
```

L3. Frames: event types with named roles, each role with the type it needs.
```
give
  giver:     person
  thing:     physical
  recipient: person
```

L4. Features every event has, so one template can cover them all when its verb is a lemma slot (sd/03/A12):
```
tense:    past | present | future
polarity: positive | negative
mood:     statement | question | command
```
(user on mood: "Wooo, yessss, moooooood.")

L5. Facts: a meaning is a set of them. Modifiers hang off events.
```
e1: give(giver=mara, thing=b1, recipient=tom), tense=past
e1.occasion = birthday(of=tom)
e1.purpose  = gift
```

## Frames are senses, not verbs

L6. The user's example: "Mara gave Tom a sense of impending doom as a birthday gift." Here `give` isn't a transfer: Mara doesn't lose the doom, and doom isn't `physical`. It's a different frame that happens to use the verb "give":
```
cause-feel
  causer:      entity
  feeling:     feeling
  experiencer: person
```
Templates attach to frames, and a word can appear in templates of many frames ("gave Tom a headache / an idea / a scare"). The type check on `thing` is what tells the two senses apart.
L6a. "as a birthday gift" still works on cause-feel (purpose=gift, occasion=birthday). The joke comes from combining an abstract feeling with a gift purpose. Allowed as long as the types allow it; whether to generate such combinations on purpose is Q35.
L6b. The user's intended reading is different: the doom is a physical object. She hands it to him ("here you go"; "oh thanks, just what I wanted"). L6 is the normal reading, the one most people would take without that context. (user)
L6c. So the same text has two meanings, and meaning-first means code always knows which one it generated. The joke reading is plain `give` with a deliberate type coercion:
```
d1: feeling "a sense of impending doom", treated as physical
e1: give(giver=mara, thing=d1, recipient=tom), purpose=gift, occasion=birthday(of=tom)
```
The coercion is explicit in the meaning, so the joke is computable: code knows exactly which type rule it broke. A sentence whose text has more than one reading gets that recorded with its template, so a reader-side task can ask for the reading.

## Role names

L7. The UI has to remind the user what roles are (user: "The UI is definitely going to need to remind me what these things (agent, theme) are, lol.").
L7a. Proposal: plain per-frame role names (giver, thing, recipient), so most need no reminder. A frame can record the generic name behind each (agent, theme, recipient as in VerbNet, 00/K4) for mining and cross-frame reuse, hidden by default. Each role also gets a one-line description the UI shows on hover.
  + readable at a glance; the frame explains itself
  + generic names kept for mining
  − two names per role to maintain
L7b. I lean L7a.

## Humour and boundaries

L8. Humour is vital, and so is testing the boundaries (user, Q35). Both are generated on purpose:
L8a. Humour as deliberate coercion (L6c): code picks a type rule to break, marks it in the meaning, and realises it.
L8b. Boundary tests: code generates combinations just inside and just outside a slot's types for the user to judge. Each answer tightens or loosens a type, or becomes a "breaks it" entry (sd/03/A8b).
L8c. Coerced and boundary meanings stay marked as such, so they can be included or excluded by set.

## Decided

D56. Our own small typed language (L1–L5). (user: "Fine with me, but you'll have to explain.")
D57. The UI explains what each role means (L7). (user)
D59. Plain per-frame role names, with the generic names kept; generic names showing in the UI with small explainers is fine too. Answers Q33. (user: "Fine with this, but I'd also be fine with the generic names having little explainers in the UI.")
D60. Start with no frames and grow them from what the user writes, borrowing names when a frame is added. Answers Q34. (user: "Fine with me. Curious to see what that process will look like in the UI." Sketch in sd/03/A15.)
D61. Generate humour and boundary tests on purpose (L8). Answers Q35. (user: "Yes, humor is vital. Also, testing the boundaries is important.")

## Open questions

(none for this note)

## Superseded

Q33 (answered by D59). Role names: plain per-frame names with generic names kept hidden (L7a)?
Q34 (answered by D60). Borrow frames and role names from VerbNet/FrameNet to seed, or start empty and grow?
Q35 (answered by D61). Figurative and odd-but-typed combinations: generate on purpose, allow only when written, or keep out?
