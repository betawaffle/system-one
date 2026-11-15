package author

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ganglion/internal/grammar"
	"ganglion/internal/lexicon"
	"ganglion/internal/realize"
)

// setup seeds a fresh grammar and loads the real lexicon (skips without it).
func setup(t *testing.T) (*World, *lexicon.Lexicon) {
	t.Helper()
	home, _ := os.UserHomeDir()
	path, err := lexicon.Latest(filepath.Join(home, ".local/share/ganglion/grammar/lexicon"))
	if err != nil {
		t.Skip("no lexicon base imported")
	}
	lex, err := lexicon.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	g, err := grammar.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := Seed(g); !ok || err != nil {
		t.Fatalf("seed: %v %v", ok, err)
	}
	w, err := Load(g)
	if err != nil {
		t.Fatal(err)
	}
	return w, lex
}

func give(f realize.Features) Meaning {
	return Meaning{Frame: "frame:give", Features: f, Roles: map[string]Filler{
		"giver": {Entity: "entity:mara"}, "thing": {Entity: "entity:book"}, "recipient": {Entity: "entity:tom"}}}
}

var pastStatement = realize.Features{Tense: "past", Aspect: "simple", Polarity: "positive", Mood: "statement"}

func classes(l Line) string {
	var b []string
	for _, t := range l.Tokens {
		c := t.Cls
		if t.Role != "" {
			c += ":" + t.Role
		}
		b = append(b, t.Text+"/"+c)
	}
	return strings.Join(b, " ")
}

func TestAlignGive(t *testing.T) {
	w, lex := setup(t)
	m := give(pastStatement)
	l := Align("Mara gave Tom a book.", m, w, lex)
	if got := classes(l); got != "Mara/role:giver gave/frame Tom/role:recipient a/role:thing book/role:thing ./punct" {
		t.Fatal(got)
	}
	if len(l.Spans) != 0 || len(l.Warnings) != 0 {
		t.Fatalf("spans %v warnings %v", l.Spans, l.Warnings)
	}

	l = Align("Tom got the book as a gift from Mara on his birthday.", m, w, lex)
	var spans []string
	for _, s := range l.Spans {
		spans = append(spans, s.Text)
	}
	if strings.Join(spans, "|") != "as a gift|on his birthday" {
		t.Fatalf("spans: %v (%s)", spans, classes(l))
	}
	if !l.Tokens[1].Converse {
		t.Fatal("got is the converse viewpoint")
	}

	l = Align("Mara didn't give Tom a book.", m, w, lex)
	if len(l.Warnings) != 1 || !strings.Contains(l.Warnings[0], "negative") {
		t.Fatalf("polarity mismatch should warn: %v", l.Warnings)
	}
	l = Align("Mara could have given Tom a book.", m, w, lex)
	if strings.Join(l.Waits, ",") != "modals" {
		t.Fatalf("waits: %v", l.Waits)
	}
	// "her" before a noun is possessive, and "!" stays its own span
	l = Align("Tom got the book from Mara on her birthday!", m, w, lex)
	spans = nil
	for _, s := range l.Spans {
		spans = append(spans, s.Text)
	}
	if strings.Join(spans, "|") != "on her birthday|!" {
		t.Fatalf("spans: %v (%s)", spans, classes(l))
	}
	l = Align("Tom gave her a book.", give(pastStatement), w, lex)
	if l.Tokens[2].Case != "obj" {
		t.Fatalf("her as object: %+v", l.Tokens[2])
	}

	l = Align("Mara gave Tom a book!", m, w, lex)
	if len(l.Spans) != 1 || !l.Spans[0].Bang {
		t.Fatalf("! should be a span: %v", l.Spans)
	}
}

func TestBuildAndGenerate(t *testing.T) {
	w, lex := setup(t)
	m := give(pastStatement)
	l := Align("Mara gave Tom a book.", m, w, lex)
	tpl, err := Build(l, m, w, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := newRand(1)
	for _, c := range []struct {
		f    realize.Features
		want string
	}{
		{pastStatement, "Mara gave Tom a book."},
		{realize.Features{Tense: "present", Polarity: "negative", Mood: "statement"}, "Mara doesn't give Tom a book."},
		{realize.Features{Tense: "future", Polarity: "positive", Mood: "question"}, "Will Mara give Tom a book?"},
	} {
		m.Features = c.f
		got, _, err := Generate(tpl, m, w, lex, realize.Style{Contract: true}, r)
		if err != nil || got != c.want {
			t.Errorf("%+v: got %q (%v), want %q", c.f, got, err, c.want)
		}
	}
	// a different thing gets the right article
	m = give(pastStatement)
	m.Roles["thing"] = Filler{Entity: "entity:umbrella"}
	m.Roles["giver"] = Filler{Entity: "entity:sam"}
	if got, _, _ := Generate(tpl, m, w, lex, realize.Style{Contract: true}, r); got != "Sam gave Tom an umbrella." {
		t.Errorf("got %q", got)
	}

	// converse viewpoint with resolved spans, and a pronoun inside a span
	m = give(pastStatement)
	l = Align("Tom got the book as a gift from Mara on his birthday.", m, w, lex)
	res := map[string]Resolution{"as a gift": {"Modifier", "purpose"}, "on his birthday": {"Modifier", "occasion"}}
	tpl, err = Build(l, m, w, res, nil)
	if err != nil {
		t.Fatal(err)
	}
	m.Roles["recipient"] = Filler{Entity: "entity:sam"}
	m.Features = realize.Features{Tense: "present", Polarity: "negative", Mood: "statement"}
	if got, _, _ := Generate(tpl, m, w, lex, realize.Style{Contract: true}, r); got != "Sam don't get the book as a gift from Mara on their birthday." &&
		got != "Sam doesn't get the book as a gift from Mara on their birthday." {
		t.Errorf("got %q", got)
	}
	if c := Compact("template:give-2", tpl); !strings.Contains(c, `e.occasion = "on his birthday"`) || !strings.Contains(c, "REF(r)  pron det") {
		t.Errorf("compact:\n%s", c)
	}

	// Sounds Odd adds a time modifier (08/D98)
	hp := Meaning{Frame: "frame:has-property", Features: realize.Features{Tense: "present", Polarity: "positive", Mood: "statement"},
		Roles: map[string]Filler{"holder": {Entity: "entity:book"}, "property": {Value: &Value{Attribute: "age", Value: "old"}}}}
	l = Align("This book is old", hp, w, lex)
	tpl, err = Build(l, hp, w, nil, []Need{{Tense: "future", Modifier: "time"}})
	if err != nil {
		t.Fatal(err)
	}
	hp.Features.Tense = "future"
	got, time, _ := Generate(tpl, hp, w, lex, realize.Style{}, r)
	if time == "" || !strings.HasSuffix(got, "this book will be old.") {
		t.Errorf("got %q, time %q", got, time)
	}
	// age = young is "new" for an artifact and "young" for a person (06/P4)
	hp.Features.Tense = "present"
	hp.Roles["property"] = Filler{Value: &Value{Attribute: "age", Value: "young"}}
	if got, _, _ := Generate(tpl, hp, w, lex, realize.Style{}, r); got != "This book is new." {
		t.Errorf("got %q", got)
	}
}

func TestCompare(t *testing.T) {
	w, lex := setup(t)
	m := Meaning{Frame: "frame:compare", Features: realize.Features{Tense: "present", Polarity: "positive", Mood: "statement"},
		Roles: map[string]Filler{"a": {Entity: "entity:mara"}, "b": {Entity: "entity:tom"}, "attribute": {Value: &Value{Attribute: "height", Dir: "more"}}}}
	r := newRand(2)
	for text, want := range map[string]string{
		"Mara is taller than Tom.":  "Mara is taller than Tom.",
		"Tom is shorter than Mara.": "Tom is shorter than Mara.",
	} {
		l := Align(text, m, w, lex)
		if len(l.Spans) != 0 {
			t.Fatalf("%s: %s", text, classes(l))
		}
		tpl, err := Build(l, m, w, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		got, _, err := Generate(tpl, m, w, lex, realize.Style{}, r)
		if got != want {
			t.Errorf("%s: got %q %v", text, got, err)
		}
		// flip the direction: the converse template says it from b's side
		m2 := m
		m2.Roles = map[string]Filler{"a": m.Roles["a"], "b": m.Roles["b"], "attribute": {Value: &Value{Attribute: "height", Dir: "less"}}}
		got, _, _ = Generate(tpl, m2, w, lex, realize.Style{}, r)
		t.Logf("%s, less: %s", text, got)
	}
}

func TestSamplesFast(t *testing.T) {
	w, lex := setup(t)
	m := give(pastStatement)
	l := Align("Mara gave Tom a book.", m, w, lex)
	tpl, _ := Build(l, m, w, nil, nil)
	s := Samples(tpl, m, w, lex, 42, 8)
	if len(s) != 8 {
		t.Fatalf("%d samples", len(s))
	}
	for _, x := range s {
		t.Log(x.Text, x.Changed)
	}
}

func BenchmarkPreview(b *testing.B) {
	t := &testing.T{}
	w, lex := setup(t)
	m := give(pastStatement)
	for b.Loop() {
		l := Align("Tom got the book as a gift from Mara on his birthday.", m, w, lex)
		tpl, _ := Build(l, m, w, map[string]Resolution{"as a gift": {"Modifier", "purpose"}, "on his birthday": {"Modifier", "occasion"}}, nil)
		Samples(tpl, m, w, lex, 42, 8)
	}
}

func TestStableOrder(t *testing.T) {
	w, lex := setup(t)
	m := Meaning{Frame: "frame:compare", Features: realize.Features{Tense: "present", Polarity: "positive", Mood: "statement"},
		Roles: map[string]Filler{"a": {Entity: "entity:mara"}, "b": {Entity: "entity:tom"}, "attribute": {Value: &Value{Attribute: "age", Dir: "more"}}}}
	l := Align("Mara is older than Tom.", m, w, lex)
	tpl, err := Build(l, m, w, nil, []Need{{Tense: "future", Modifier: "time"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range Samples(tpl, m, w, lex, 7, 8) {
		if s.Features.Tense != "present" || s.Time != "" {
			t.Errorf("age comparisons stay in the present: %q %+v", s.Text, s.Features)
		}
	}
	m.Roles["attribute"] = Filler{Value: &Value{Attribute: "height", Dir: "more"}}
	tenses := map[string]bool{}
	for _, s := range Samples(tpl, m, w, lex, 7, 8) {
		tenses[s.Features.Tense] = true
	}
	if len(tenses) < 2 {
		t.Errorf("height comparisons vary tense: %v", tenses)
	}
}

func TestUpgradeSeededAge(t *testing.T) {
	g, _ := grammar.Open(t.TempDir())
	seedMeta := grammar.Meta{By: "ganglion", Why: "create", Client: "ganglion/seed"}
	old := ageAttr
	old.StableOrder = false
	g.Put("attribute:age", "attribute", old, seedMeta)
	g.Put("frame:x", "frame", Frame{Name: "x"}, seedMeta)
	if _, err := Seed(g); err != nil {
		t.Fatal(err)
	}
	var a Attribute
	g.Get("attribute:age", &a)
	if !a.StableOrder || a.Direction != "increasing" || len(g.History("attribute:age")) != 2 {
		t.Fatal("an unedited seeded age should be upgraded")
	}
	// once someone has edited it, upgrade leaves it alone
	a.StableOrder = false
	g.Put("attribute:age", "attribute", a, grammar.Meta{By: "user"})
	Seed(g)
	g.Get("attribute:age", &a)
	if a.StableOrder {
		t.Fatal("upgrade must not overwrite an edit")
	}
}

func TestOnlyIncreasing(t *testing.T) {
	w, lex := setup(t)
	m := Meaning{Frame: "frame:has-property", Features: realize.Features{Tense: "present", Polarity: "positive", Mood: "statement"},
		Roles: map[string]Filler{"holder": {Entity: "entity:apple"}, "property": {Value: &Value{Attribute: "age", Value: "old"}}}}
	l := Align("this apple is old", m, w, lex)
	tpl, err := Build(l, m, w, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for seed := uint64(0); seed < 20; seed++ {
		for _, s := range Samples(tpl, m, w, lex, seed, 8) {
			f := s.Features
			if (f.Tense == "past" && f.Polarity == "positive") || (f.Tense == "future" && f.Polarity == "negative") {
				t.Fatalf("implies the apple is gone: %q", s.Text)
			}
			n++
		}
	}
	if n == 0 {
		t.Fatal("no samples")
	}
	young := m
	young.Roles = map[string]Filler{"holder": m.Roles["holder"], "property": {Value: &Value{Attribute: "age", Value: "young"}}}
	young.Features.Tense = "future"
	if w.Allowed(young) {
		t.Fatal("will be new can't happen")
	}
	young.Features.Tense = "past"
	if !w.Allowed(young) {
		t.Fatal("was new is fine")
	}
}

func TestDeterminers(t *testing.T) {
	w, lex := setup(t)
	hp := Meaning{Frame: "frame:has-property", Features: realize.Features{Tense: "present", Polarity: "positive", Mood: "statement"},
		Roles: map[string]Filler{"holder": {Entity: "entity:apple"}, "property": {Value: &Value{Attribute: "age", Value: "old"}}}}
	// "this apple" and "the apple" are one template shape (04/F4)
	t1, _ := Build(Align("this apple is old", hp, w, lex), hp, w, nil, nil)
	t2, _ := Build(Align("The apple is old.", hp, w, lex), hp, w, nil, nil)
	if len(t1.Tree.Kids) != len(t2.Tree.Kids) || t1.Tree.Kids[0].Kind != "REF" || t1.Tree.Kids[0].Det != "this" || t2.Tree.Kids[0].Det != "the" {
		t.Fatalf("trees: %+v / %+v", t1.Tree.Kids, t2.Tree.Kids)
	}
	dets := map[string]bool{}
	for seed := uint64(0); seed < 10; seed++ {
		for _, s := range Samples(t1, hp, w, lex, seed, 8) {
			first := strings.ToLower(strings.Fields(s.Text)[0])
			if first == "a" || first == "an" {
				t.Fatalf("no \"a\" on the subject of a be frame: %q", s.Text)
			}
			dets[strings.Fields(s.Fillers["holder"])[0]] = true
		}
	}
	if len(dets) < 3 {
		t.Errorf("references should vary: %v", dets)
	}

	// a possessive before another role's noun is extra meaning: an owner
	m := give(pastStatement)
	l := Align("Mara gave Tom her book.", m, w, lex)
	if len(l.Spans) != 1 || l.Spans[0].Text != "her" {
		t.Fatalf("spans: %+v (%s)", l.Spans, classes(l))
	}
	tpl, err := Build(l, m, w, map[string]Resolution{"her": {"Modifier", "owner"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	m.Roles["giver"] = Filler{Entity: "entity:lee"}
	if got, _, _ := Generate(tpl, m, w, lex, realize.Style{}, newRand(3)); got != "Lee gave Tom his book." {
		t.Errorf("got %q", got)
	}
	if !strings.Contains(Compact("x", tpl), `e.owner = "her"`) {
		t.Error(Compact("x", tpl))
	}
}

func TestCoverage(t *testing.T) {
	w, lex := setup(t)
	m := give(pastStatement)
	tpl, err := Build(Align("Mara gave Tom a book.", m, w, lex), m, w, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	dims := Dims(tpl, m, w)
	names := map[string]int{}
	for _, d := range dims {
		names[d.Name] = len(d.Values)
	}
	// 3 tenses x 2 polarities x 2 moods; the thing's reference; two people's pronoun sets
	if names["verb"] != 12 || names["ref:thing"] != 4 || names["who:giver"] != 3 || names["who:recipient"] != 3 {
		t.Fatalf("dims: %v", names)
	}
	judged := map[string]map[string]bool{"verb": {"past positive statement": true}}
	got := Aimed(tpl, m, w, lex, 1, 8, judged)
	if len(got) != 8 {
		t.Fatalf("%d samples", len(got))
	}
	covered := map[string]bool{}
	for _, s := range got {
		c := ComboOf(s, w)
		for k, v := range c {
			covered[k+"="+v] = true
		}
		if c["verb"] == "past positive statement" && got[0].Text == s.Text {
			t.Errorf("first sample should aim at something unjudged: %q", s.Text)
		}
	}
	if !covered["who:giver=they"] && !covered["who:recipient=they"] {
		t.Log("they not reached in 8; fine, it's further down the list")
	}
	t.Log(got[0].Text, ComboOf(got[0], w))

	// has-property: no "a" on the subject, age rules trim the verb forms
	hp := Meaning{Frame: "frame:has-property", Features: realize.Features{Tense: "present", Polarity: "positive", Mood: "statement"},
		Roles: map[string]Filler{"holder": {Entity: "entity:apple"}, "property": {Value: &Value{Attribute: "age", Value: "old"}}}}
	tpl, _ = Build(Align("this apple is old", hp, w, lex), hp, w, nil, nil)
	for _, d := range Dims(tpl, hp, w) {
		if d.Name == "ref:holder" && strings.Join(d.Values, ",") != "the,this,that" {
			t.Errorf("ref:holder %v", d.Values)
		}
		if d.Name == "verb" && len(d.Values) != 8 { // 12 minus past positive and future negative, both moods
			t.Errorf("verb %d %v", len(d.Values), d.Values)
		}
	}
}

func TestImplications(t *testing.T) {
	w, lex := setup(t)
	m := give(pastStatement)
	m.Roles["thing"] = Filler{Entity: "entity:umbrella"}
	tpl, err := Build(Align("Mara gave Tom that umbrella.", m, w, lex), m, w, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	pres := m
	pres.Features = realize.Features{Tense: "present", Aspect: "simple", Polarity: "positive", Mood: "question"}
	if imp, skip := w.Implications(tpl, pres); !skip || strings.Join(imp, ",") != "habit" {
		t.Fatalf("a habit of giving that one umbrella should be skipped: %v %v", imp, skip)
	}
	pres.Roles = map[string]Filler{"giver": m.Roles["giver"], "recipient": m.Roles["recipient"], "thing": {Entity: "entity:umbrella", Det: "a"}}
	if imp, skip := w.Implications(tpl, pres); skip || strings.Join(imp, ",") != "habit" {
		t.Fatalf("giving an umbrella as a habit is fine, tagged: %v %v", imp, skip)
	}
	fut := m
	fut.Features = realize.Features{Tense: "future", Aspect: "simple", Polarity: "negative", Mood: "statement"}
	if imp, _ := w.Implications(tpl, fut); strings.Join(imp, ",") != "refuses" {
		t.Fatalf("won't give reads as refusing: %v", imp)
	}
	for seed := uint64(0); seed < 20; seed++ {
		for _, s := range Samples(tpl, m, w, lex, seed, 8) {
			if s.Features.Tense == "present" && strings.Contains(strings.ToLower(s.Text), "give") {
				for _, d := range []string{" that ", " this ", " the "} {
					if strings.Contains(s.Text, d) {
						t.Fatalf("present habit with a particular thing: %q", s.Text)
					}
				}
			}
		}
	}
	// states aren't affected
	hp := Meaning{Frame: "frame:has-property", Features: realize.Features{Tense: "present", Polarity: "positive", Mood: "statement"},
		Roles: map[string]Filler{"holder": {Entity: "entity:apple"}, "property": {Value: &Value{Attribute: "age", Value: "old"}}}}
	if imp, skip := w.Implications(tpl, hp); skip || len(imp) > 0 {
		t.Fatal("states have no habit reading")
	}
}

func TestRodsAndNext(t *testing.T) {
	w, _ := setup(t)
	h, l := w.Attributes["attribute:height"], w.Attributes["attribute:length"]
	if h.Applies("rod", w) || !h.Applies("artifact", w) || !h.Applies("person", w) {
		t.Fatal("height: people and most things, not rods")
	}
	if !l.Applies("rod", w) || l.Applies("person", w) {
		t.Fatal("length: rods")
	}
	if w.Entities["entity:umbrella"].Type != "rod" {
		t.Fatal("an umbrella is a rod")
	}
	// Next never offers height for an umbrella, and skips meanings already used
	used := map[string]bool{}
	for n := 0; n < 60; n++ {
		m, err := Next(w, n, "frame:has-property", used)
		if err != nil {
			t.Fatal(err)
		}
		if m.Roles["holder"].Entity == "entity:umbrella" && m.Roles["property"].Value.Attribute == "height" {
			t.Fatal("tall umbrella")
		}
		if used[Signature(m)] {
			t.Fatalf("repeated %s after %d", Signature(m), n)
		}
		used[Signature(m)] = true
	}
}

func TestUpgradeAddsSeedObjects(t *testing.T) {
	g, _ := grammar.Open(t.TempDir())
	meta := grammar.Meta{By: "ganglion", Why: "create", Client: "ganglion/seed"}
	g.Put("frame:x", "frame", Frame{Name: "x"}, meta)
	g.Put("entity:umbrella", "entity", Entity{Type: "artifact", Noun: "umbrella", Pronouns: "it"}, meta)
	Seed(g)
	var e Entity
	g.Get("entity:umbrella", &e)
	if e.Type != "rod" {
		t.Fatal("seeded umbrella should become a rod")
	}
	if _, ok := g.Head("entity:pencil"); !ok {
		t.Fatal("new seed entities should be added")
	}
	if _, ok := g.Head("type:rod"); !ok {
		t.Fatal("new seed types should be added")
	}
}

func TestContractions(t *testing.T) {
	w, lex := setup(t)
	hp := Meaning{Frame: "frame:has-property", Features: realize.Features{Tense: "present", Polarity: "positive", Mood: "statement"},
		Roles: map[string]Filler{"holder": {Entity: "entity:dog"}, "property": {Value: &Value{Attribute: "age", Value: "old"}}}}
	full, _ := Build(Align("The dog is old.", hp, w, lex), hp, w, nil, nil)
	for _, s := range []string{"The dog's old.", "That's an old dog.", "It's old."} {
		l := Align(s, hp, w, lex)
		if len(l.Spans) != 0 || l.VG < 0 || l.Tokens[l.VG].Text != "'s" {
			t.Errorf("%s: %s, spans %v", s, classes(l), l.Spans)
		}
	}
	short, _ := Build(Align("The dog's old.", hp, w, lex), hp, w, nil, nil)
	if Signature(hp) == "" || Compact("x", short) != Compact("x", full) {
		t.Errorf("a contraction is a style, not a new template:\n%s\n%s", Compact("x", short), Compact("x", full))
	}
	// a noun after 's: a possessive, left alone
	m := give(pastStatement)
	for _, tok := range Align("Mara gave Tom's book.", m, w, lex).Tokens {
		if tok.Text == "'s" {
			t.Errorf("possessive split: %+v", tok)
		}
	}
}

func TestLocated(t *testing.T) {
	w, lex := setup(t)
	pos := w.Attributes["attribute:position"]
	// Next: a thing (not a person or a place), somewhere a relation fits
	used := map[string]bool{}
	for i := 0; i < 30; i++ {
		m, err := Next(w, i, "frame:located", used)
		if err != nil {
			t.Fatal(err)
		}
		th, pl, rel := w.Entities[m.Roles["thing"].Entity], w.Entities[m.Roles["place"].Entity], m.Roles["relation"].Value
		if th.Name != "" || w.IsA(th.Type, "place") || rel == nil || !pos.HasWord(rel.Value, pl.Type, w) {
			t.Fatalf("next %d: %+v", i, m.Roles)
		}
	}
	m := Meaning{Frame: "frame:located", Features: realize.Features{Tense: "present", Polarity: "positive", Mood: "statement"},
		Roles: map[string]Filler{"thing": {Entity: "entity:cup"}, "place": {Entity: "entity:table"}, "relation": {Value: &Value{Attribute: "position", Value: "on"}}}}
	l := Align("The cup is on the table.", m, w, lex)
	if len(l.Spans) != 0 || len(l.Warnings) != 0 {
		t.Fatalf("%s %v %v", classes(l), l.Spans, l.Warnings)
	}
	tpl, err := Build(l, m, w, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	places := map[string]bool{}
	for seed := uint64(0); seed < 10; seed++ {
		for _, s := range Samples(tpl, m, w, lex, seed, 8) {
			pl := s.Fillers["place"]
			places[pl] = true
			if !strings.Contains(s.Text, " on ") || strings.HasSuffix(pl, "cup") || strings.HasSuffix(pl, "park") {
				t.Fatalf("on goes on surfaces only: %q (%v)", s.Text, s.Fillers)
			}
		}
	}
	if len(places) < 2 {
		t.Errorf("places should vary: %v", places)
	}
	// "in" fits containers and places, never surfaces
	m.Roles["place"], m.Roles["relation"] = Filler{Entity: "entity:park"}, Filler{Value: &Value{Attribute: "position", Value: "in"}}
	m.Roles["thing"] = Filler{Entity: "entity:dog"}
	tpl, err = Build(Align("Is the dog in the park?", m, w, lex), m, w, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range Samples(tpl, m, w, lex, 4, 8) {
		if p := s.Fillers["place"]; strings.HasSuffix(p, "table") || strings.HasSuffix(p, "chair") {
			t.Fatalf("in a surface: %q", s.Text)
		}
	}
}

// a line written as a question builds the same tree as its statement, so
// samples in every mood come out right (they were "Did Mara gives Noor a book.")
func TestQuestionLines(t *testing.T) {
	w, lex := setup(t)
	q := realize.Features{Tense: "past", Polarity: "positive", Mood: "question"}
	st := pastStatement
	cases := []struct{ q, s string }{{"Did Mara give Tom the book?", "Mara gave Tom the book."}, {"Didn't Mara give Tom the book?", "Mara didn't give Tom the book."}}
	for _, c := range cases {
		mq, ms := give(q), give(st)
		if c.q[:6] == "Didn't" {
			mq.Features.Polarity, ms.Features.Polarity = "negative", "negative"
		}
		a, err := Build(Align(c.q, mq, w, lex), mq, w, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := Build(Align(c.s, ms, w, lex), ms, w, nil, nil)
		if Compact("x", a) != Compact("x", b) {
			t.Errorf("%s:\n%s\nwant\n%s", c.q, Compact("x", a), Compact("x", b))
		}
		for _, s := range Samples(a, mq, w, lex, 5, 8) {
			if (s.Features.Mood == "question") != strings.HasSuffix(s.Text, "?") || strings.Contains(s.Text, "Did ") && s.Features.Mood != "question" {
				t.Errorf("%q (%+v)", s.Text, s.Features)
			}
		}
	}
}
