package lexicon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// a few pronunciations, as CMUdict has them
var cmu = map[string]Pron{
	"stop": strings.Fields("S T AA1 P"), "commit": strings.Fields("K AH0 M IH1 T"), "visit": strings.Fields("V IH1 Z IH0 T"),
	"big": strings.Fields("B IH1 G"), "happy": strings.Fields("HH AE1 P IY0"), "beautiful": strings.Fields("B Y UW1 T AH0 F AH0 L"),
	"hour": strings.Fields("AW1 ER0"), "university": strings.Fields("Y UW2 N AH0 V ER1 S AH0 T IY0"),
	"umbrella": strings.Fields("AH0 M B R EH1 L AH0"), "book": strings.Fields("B UH1 K"), "quit": strings.Fields("K W IH1 T"),
	"tall": strings.Fields("T AO1 L"), "old": strings.Fields("OW1 L D"), "open": strings.Fields("OW1 P AH0 N"),
}

func TestVerbForms(t *testing.T) {
	cases := []struct{ base, pres3, past, ing string }{
		{"stop", "stops", "stopped", "stopping"},
		{"commit", "commits", "committed", "committing"},
		{"visit", "visits", "visited", "visiting"},
		{"open", "opens", "opened", "opening"},
		{"try", "tries", "tried", "trying"},
		{"play", "plays", "played", "playing"},
		{"lie", "lies", "lied", "lying"},
		{"make", "makes", "maked", "making"}, // the rule; the irregular table supplies made
		{"panic", "panics", "panicked", "panicking"},
		{"fix", "fixes", "fixed", "fixing"},
		{"watch", "watches", "watched", "watching"},
		{"agree", "agrees", "agreed", "agreeing"},
		{"quit", "quits", "quitted", "quitting"}, // the rule; the table supplies quit
		{"give up", "gives up", "gived up", "giving up"},
	}
	for _, c := range cases {
		p3, past, ing, _ := VerbForms(c.base, cmu[strings.Fields(c.base)[0]])
		if p3 != c.pres3 || past != c.past || ing != c.ing {
			t.Errorf("%s: got %s/%s/%s, want %s/%s/%s", c.base, p3, past, ing, c.pres3, c.past, c.ing)
		}
	}
}

func TestPlural(t *testing.T) {
	for in, want := range map[string]string{"book": "books", "box": "boxes", "city": "cities", "day": "days", "church": "churches", "ice cream": "ice creams"} {
		if got, _ := Plural(in); got != want {
			t.Errorf("%s: got %s, want %s", in, got, want)
		}
	}
}

func TestComparative(t *testing.T) {
	cases := []struct {
		adj, er, est string
		more         bool
	}{
		{"big", "bigger", "biggest", false},
		{"tall", "taller", "tallest", false},
		{"happy", "happier", "happiest", false},
		{"old", "older", "oldest", false},
		{"beautiful", "", "", true},
	}
	for _, c := range cases {
		er, est, more, _ := Comparative(c.adj, cmu[c.adj])
		if er != c.er || est != c.est || more != c.more {
			t.Errorf("%s: got %s/%s/%v", c.adj, er, est, more)
		}
	}
}

func TestAn(t *testing.T) {
	for w, an := range map[string]bool{"hour": true, "university": false, "umbrella": true, "book": false} {
		if got := cmu[w].StartsWithVowelSound(); got != an {
			t.Errorf("%s: an=%v", w, got)
		}
	}
}

// TestBuild runs the real import when the raw downloads are present.
func TestBuild(t *testing.T) {
	home, _ := os.UserHomeDir()
	raw := filepath.Join(home, ".local/share/ganglion/raw")
	src := Sources{WordNetDict: filepath.Join(raw, "wordnet-3.1/dict"), CMUDict: filepath.Join(raw, "cmudict/cmudict.dict")}
	if _, err := os.Stat(src.CMUDict); err != nil {
		t.Skip("raw lexicon sources not downloaded")
	}
	entries, err := Build(src)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path, hash, err := Write(dir, entries)
	if err != nil {
		t.Fatal(err)
	}
	l, err := Load(path)
	if err != nil || l.Hash != hash {
		t.Fatalf("load: %v, hash %s vs %s", err, l.Hash, hash)
	}
	t.Logf("%d entries", l.Len())
	want := []struct{ pos, lemma, slot, form string }{
		{"verb", "give", "past", "gave"}, {"verb", "give", "pastp", "given"}, {"verb", "give", "pres3", "gives"},
		{"verb", "go", "past", "went"}, {"verb", "go", "pres3", "goes"}, {"verb", "be", "pres3", "is"},
		{"verb", "stop", "past", "stopped"}, {"verb", "visit", "past", "visited"}, {"verb", "commit", "ing", "committing"},
		{"verb", "swim", "pastp", "swum"}, {"verb", "make", "past", "made"},
		{"noun", "child", "plural", "children"}, {"noun", "person", "plural", "people"}, {"noun", "mouse", "plural", "mice"},
		{"noun", "book", "plural", "books"}, {"noun", "box", "plural", "boxes"},
		{"adj", "good", "er", "better"}, {"adj", "big", "est", "biggest"}, {"adj", "old", "er", "older"},
	}
	for _, w := range want {
		e := l.Get(w.pos, w.lemma)
		if e == nil {
			t.Errorf("missing %s %s", w.pos, w.lemma)
			continue
		}
		if e.Forms[w.slot] != w.form {
			t.Errorf("%s %s %s: got %q, want %q", w.pos, w.lemma, w.slot, e.Forms[w.slot], w.form)
		}
	}
	if e := l.Get("noun", "hour"); e == nil || !e.An {
		t.Error("an hour")
	}
	if e := l.Get("noun", "university"); e == nil || e.An {
		t.Error("a university")
	}
	if e := l.Get("adj", "beautiful"); e == nil || !e.More {
		t.Error("more beautiful")
	}
	refs := l.Lookup("gave")
	if len(refs) == 0 || refs[0].Lemma != "give" || refs[0].Slot != "past" {
		t.Errorf("reverse lookup of gave: %+v", refs)
	}
}
