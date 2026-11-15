package author

import (
	"strings"

	"ganglion/internal/grammar"
)

// Seed writes the starting grammar when the store has no frames yet: the type
// tree (06/Y1), a few attributes (06/P7), the first frames, and entities to
// fill them. Everything after this grows from what the user writes (02/D60).
var ageAttr = Attribute{Name: "age", AppliesTo: []string{"physical"}, Scale: []string{"young", "old"},
	Words: map[string]map[string]string{"young": {"*": "young", "artifact": "new"}, "old": {"*": "old"}}, StableOrder: true, Direction: "increasing"}

func Seed(g *grammar.Store) (bool, error) {
	if len(g.Heads("frame")) > 0 {
		return false, upgrade(g)
	}
	m := grammar.Meta{By: "ganglion", Why: "create", Client: "ganglion/seed"}
	put := func(id, kind string, v any) error { _, err := g.Put(id, kind, v, m); return err }
	types := []Type{
		{"entity", "", "Anything at all."},
		{"physical", "entity", "Something you could touch."},
		{"abstract", "entity", "Something you can't touch: an idea, a feeling."},
		{"person", "physical", "A human being."},
		{"animal", "physical", "A living creature that isn't a person."},
		{"artifact", "physical", "A made thing."},
		{"food", "physical", "Something to eat or drink."},
		{"idea", "abstract", "A thought or plan."},
		{"feeling", "abstract", "Something felt."},
	}
	for _, t := range types {
		if err := put("type:"+t.Name, "type", t); err != nil {
			return false, err
		}
	}
	attrs := []Attribute{
		ageAttr,
		{Name: "height", AppliesTo: []string{"person", "animal", "artifact"}, Scale: []string{"short", "tall"},
			Words: map[string]map[string]string{"short": {"*": "short"}, "tall": {"*": "tall"}}},
		{Name: "size", AppliesTo: []string{"physical"}, Scale: []string{"small", "big"},
			Words: map[string]map[string]string{"small": {"*": "small"}, "big": {"*": "big"}}},
		{Name: "weight", AppliesTo: []string{"physical"}, Scale: []string{"light", "heavy"},
			Words: map[string]map[string]string{"light": {"*": "light"}, "heavy": {"*": "heavy"}}},
	}
	for _, a := range attrs {
		if err := put("attribute:"+a.Name, "attribute", a); err != nil {
			return false, err
		}
	}
	frames := []Frame{
		{Name: "has-property", Title: "Has Property", Explainer: "Something has a quality, like being old or big.",
			Roles: []Role{
				{Name: "holder", Type: "physical", Generic: "theme", Explainer: "The thing or person that has the quality."},
				{Name: "property", Type: "property", Generic: "attribute", Explainer: "The quality: an attribute and a value, like age = old."},
			}, Verbs: []string{"be"}},
		{Name: "give", Title: "Give", Explainer: "Someone passes something to someone else, who then has it.",
			Roles: []Role{
				{Name: "giver", Type: "person", Generic: "agent", Explainer: "Who gives it. They had it before."},
				{Name: "thing", Type: "physical", Generic: "theme", Explainer: "What changes hands."},
				{Name: "recipient", Type: "person", Generic: "recipient", Explainer: "Who gets it. They have it after."},
			}, Verbs: []string{"give", "hand"}, Converse: []string{"get", "receive"}},
		{Name: "compare", Title: "Compare", Explainer: "Two things are compared on one attribute: more, less, or the same.",
			Roles: []Role{
				{Name: "a", Type: "physical", Generic: "theme", Explainer: "The first thing compared."},
				{Name: "b", Type: "physical", Generic: "standard", Explainer: "What it's compared against."},
				{Name: "attribute", Type: "property", Generic: "attribute", Explainer: "What they're compared on, and which way."},
			}, Verbs: []string{"be"}},
	}
	for _, f := range frames {
		if err := put("frame:"+f.Name, "frame", f); err != nil {
			return false, err
		}
	}
	ents := map[string]Entity{
		"mara": {Type: "person", Name: "Mara", Pronouns: "she"}, "tom": {Type: "person", Name: "Tom", Pronouns: "he"},
		"sam": {Type: "person", Name: "Sam", Pronouns: "they"}, "ana": {Type: "person", Name: "Ana", Pronouns: "she"},
		"lee": {Type: "person", Name: "Lee", Pronouns: "he"}, "noor": {Type: "person", Name: "Noor", Pronouns: "she"},
		"book": {Type: "artifact", Noun: "book", Pronouns: "it"}, "cup": {Type: "artifact", Noun: "cup", Pronouns: "it"},
		"box": {Type: "artifact", Noun: "box", Pronouns: "it"}, "lamp": {Type: "artifact", Noun: "lamp", Pronouns: "it"},
		"umbrella": {Type: "artifact", Noun: "umbrella", Pronouns: "it"}, "chair": {Type: "artifact", Noun: "chair", Pronouns: "it"},
		"dog": {Type: "animal", Noun: "dog", Pronouns: "it"}, "apple": {Type: "food", Noun: "apple", Pronouns: "it"},
		"doom": {Type: "feeling", Noun: "sense of impending doom", Pronouns: "it", Coerced: "physical"},
	}
	for id, e := range ents {
		if err := put("entity:"+id, "entity", e); err != nil {
			return false, err
		}
	}
	return true, nil
}

// upgrade brings seeded objects nobody has edited up to the current seed, so a
// grammar seeded before a change gets it too. Objects with any later version
// are left alone.
func upgrade(g *grammar.Store) error {
	for id, want := range map[string]any{"attribute:age": ageAttr} {
		h := g.History(id)
		untouched := len(h) > 0
		for _, v := range h {
			untouched = untouched && v.By == "ganglion" && v.Client == "ganglion/seed"
		}
		if !untouched {
			continue
		}
		kind := id[:strings.IndexByte(id, ':')]
		if _, err := g.Put(id, kind, want, grammar.Meta{By: "ganglion", Why: "edit", Client: "ganglion/seed"}); err != nil {
			return err
		}
	}
	return nil
}
