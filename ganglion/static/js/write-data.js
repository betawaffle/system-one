// Hard-coded data for the writing screen mock-up (design/synthetic-data/08,
// D97). Everything here stands in for things that will come from the store:
// entities, the lexicon, frames and meanings, and model output.

// Pronoun sets (sd/05/D76): subject, object, possessive determiner,
// possessive pronoun, reflexive. "plural" makes the verb plural (they are).
const she = { subj: "she", obj: "her", det: "her", pos: "hers", refl: "herself" };
const he = { subj: "he", obj: "him", det: "his", pos: "his", refl: "himself" };
const they = { subj: "they", obj: "them", det: "their", pos: "theirs", refl: "themselves", plural: true };
const it = { subj: "it", obj: "it", det: "its", pos: "its", refl: "itself" };

export const entities = {
  mara: { type: "person", name: "Mara", pron: she },
  tom: { type: "person", name: "Tom", pron: he },
  sam: { type: "person", name: "Sam", pron: they },
  ana: { type: "person", name: "Ana", pron: she },
  lee: { type: "person", name: "Lee", pron: he },
  book: { type: "artifact", noun: "book", plural: "books", pron: it },
  cup: { type: "artifact", noun: "cup", plural: "cups", pron: it },
  box: { type: "artifact", noun: "box", plural: "boxes", pron: it },
  lamp: { type: "artifact", noun: "lamp", plural: "lamps", pron: it },
  umbrella: { type: "artifact", noun: "umbrella", plural: "umbrellas", pron: it },
  doom: { type: "feeling", phrase: "a sense of impending doom", noun: "sense of impending doom", pron: it, coerced: "physical" },
};

// Verbs: base, third-person present, past, past participle, -ing.
export const verbs = {
  be: { forms: ["be", "is", "was", "been", "being"], copula: true },
  give: { forms: ["give", "gives", "gave", "given", "giving"] },
  get: { forms: ["get", "gets", "got", "gotten", "getting"] },
  receive: { forms: ["receive", "receives", "received", "received", "receiving"] },
  hand: { forms: ["hand", "hands", "handed", "handed", "handing"] },
};

// Adjectives: base, comparative, superlative.
export const adjectives = {
  old: ["old", "older", "oldest"],
  new: ["new", "newer", "newest"],
  tall: ["tall", "taller", "tallest"],
  short: ["short", "shorter", "shortest"],
  heavy: ["heavy", "heavier", "heaviest"],
};

// Grammar words: never untraced on their own (sd/03/A13b).
export const grammar = new Set(["a", "an", "the", "to", "than", "as", "of", "this", "that", "these", "those", "not", "n't", "did", "does", "do", "will", "has", "have", "had", "am", "are", "were", "very"]);

export const frames = {
  "has-property": {
    title: "Has Property",
    explainer: "Something has a quality, like being old or red.",
    roles: {
      holder: { explainer: "The thing or person that has the quality.", generic: "theme" },
      property: { explainer: "The quality it has: an attribute and a value, like age = old.", generic: "attribute" },
    },
    verbs: ["be"],
  },
  give: {
    title: "Give",
    explainer: "Someone passes something to someone else, who then has it.",
    roles: {
      giver: { explainer: "Who gives it. They had it before.", generic: "agent" },
      thing: { explainer: "What changes hands.", generic: "theme" },
      recipient: { explainer: "Who gets it. They have it after.", generic: "recipient" },
    },
    verbs: ["give", "hand"],
    converse: ["get", "receive"],
  },
  compare: {
    title: "Compare",
    explainer: "Two things are compared on one attribute: more, less, or the same.",
    roles: {
      a: { explainer: "The first thing compared.", generic: "theme" },
      b: { explainer: "What it's compared against.", generic: "standard" },
      attribute: { explainer: "What they're compared on, and which way: more, less, same.", generic: "attribute" },
    },
    verbs: ["be"],
  },
};

// Meanings to write for. "gap" is the mock coverage reason Next shows (sd/08/W8).
export const meanings = [
  {
    frame: "has-property",
    roles: { holder: "book", property: { attribute: "age", value: "old", word: "old" } },
    features: { tense: "present", polarity: "positive", mood: "statement" },
    gap: "No templates yet for Has Property.",
  },
  {
    frame: "give",
    roles: { giver: "mara", thing: "book", recipient: "tom" },
    features: { tense: "past", polarity: "positive", mood: "statement" },
    gap: "Give has 0 templates in the past tense.",
  },
  {
    frame: "compare",
    roles: { a: "mara", b: "tom", attribute: { attribute: "height", value: "more", word: "taller" } },
    features: { tense: "present", polarity: "positive", mood: "statement" },
    gap: "Compare has no templates yet.",
  },
  {
    frame: "give",
    roles: { giver: "mara", thing: "doom", recipient: "tom" },
    features: { tense: "past", polarity: "positive", mood: "statement" },
    modifiers: { purpose: "gift", occasion: "Tom's birthday" },
    joke: "A feeling treated as a physical object (sd/02/L6c).",
    gap: "Humour on purpose: a coerced type (sd/02/D61).",
  },
];

// Stand-in for the model's span suggestions (sd/08/W4): first keyword wins.
export const spanHints = [
  [/birthday|christmas|anniversary/, 0, "occasion"],
  [/gift|present\b/, 0, "purpose"],
  [/finally|at last|eventually/, 3, "expected and late"],
  [/mail|post|sent|send|ship/, 2, "send"],
  [/yesterday|today|monday|morning|night/, 0, "time"],
];
export const choices = ["Modifier", "New Modifier", "Event", "Attitude", "Drop"];

// Stand-in for word-helper tables (sd/03/A8), keyed by lowercase word.
export const helper = {
  gave: { role: "give, past", same: "handed (adds: physically)", diff: "lent, sold, showed, threw", breaks: "donated (no \"donated Tom a book\")" },
  book: { role: "thing (slot)", same: "any object: cup, lamp, box", diff: "", breaks: "an idea (another frame)" },
  a: { role: "the thing is indefinite (new to the reader)", same: "", diff: "the, his, her", breaks: "" },
  old: { role: "property: age = old", same: "aged (formal)", diff: "new, ancient, worn", breaks: "" },
  is: { role: "the verb be (grammar for Has Property)", same: "", diff: "seems, looks (add: appearance)", breaks: "" },
  taller: { role: "attribute: height, more", same: "", diff: "shorter, as tall as", breaks: "" },
  mara: { role: "a person (slot)", same: "any person, she, my aunt", diff: "", breaks: "the idea" },
  tom: { role: "a person (slot)", same: "any person, he, my uncle", diff: "", breaks: "the idea" },
};
