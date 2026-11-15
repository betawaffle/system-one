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
	if got := classes(l); got != "Mara/role:giver gave/frame Tom/role:recipient a/grammar book/role:thing ./punct" {
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
	if !a.StableOrder || len(g.History("attribute:age")) != 2 {
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
