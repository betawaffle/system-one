# Meaning language (draft 1, 2026-11-15)

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

## Role names

L7. The UI has to remind the user what roles are (user: "The UI is definitely going to need to remind me what these things (agent, theme) are, lol.").
L7a. Proposal: plain per-frame role names (giver, thing, recipient), so most need no reminder. A frame can record the generic name behind each (agent, theme, recipient as in VerbNet, 00/K4) for mining and cross-frame reuse, hidden by default. Each role also gets a one-line description the UI shows on hover.
  + readable at a glance; the frame explains itself
  + generic names kept for mining
  − two names per role to maintain
L7b. I lean L7a.

## Decided

D56. Our own small typed language (L1–L5). (user: "Fine with me, but you'll have to explain.")
D57. The UI explains what each role means (L7). (user)

## Open questions

Q33. Role names: plain per-frame names with generic names kept hidden (L7a)? I lean yes.
Q34. Borrow frames and role names from VerbNet/FrameNet to seed, or start empty and grow from what the user writes? I lean start empty, borrowing names when a frame is added.
Q35. Figurative and odd-but-typed combinations (L6a): generate them on purpose, allow them only when the user wrote one, or keep them out? Not urgent; decide when generation runs at volume.
