package realize

import (
	"strings"
	"testing"

	"ganglion/internal/lexicon"
)

func testLex() *lexicon.Lexicon {
	l := lexicon.New()
	l.Put(&lexicon.Entry{Lemma: "give", POS: "verb", Forms: map[string]string{"pres3": "gives", "past": "gave", "pastp": "given", "ing": "giving"}})
	l.Put(&lexicon.Entry{Lemma: "be", POS: "verb", Forms: map[string]string{"pres3": "is", "past": "was", "pastp": "been", "ing": "being"}})
	l.Put(&lexicon.Entry{Lemma: "have", POS: "verb", Forms: map[string]string{"pres3": "has", "past": "had", "pastp": "had", "ing": "having"}})
	l.Put(&lexicon.Entry{Lemma: "umbrella", POS: "noun", An: true})
	l.Put(&lexicon.Entry{Lemma: "hour", POS: "noun", An: true})
	l.Put(&lexicon.Entry{Lemma: "university", POS: "noun"})
	return l
}

// say builds "SUBJ VG REST" in statement or question order.
func say(t *testing.T, l *lexicon.Lexicon, verb, subj string, a Agr, f Features, st Style, rest string) string {
	vg, err := VerbGroup(l.Get("verb", verb), f, a, st)
	if err != nil {
		t.Fatalf("%s %+v: %v", verb, f, err)
	}
	var w []string
	end := "."
	switch f.Mood {
	case "question":
		before, after := vg.QuestionParts()
		w = append(append(append(w, before...), subj), after...)
		end = "?"
	case "command":
		w = vg.Statement()
	default:
		w = append([]string{subj}, vg.Statement()...)
	}
	return Sentence(l, append(w, strings.Fields(rest)...), end)
}

func TestVerbGroups(t *testing.T) {
	l := testLex()
	c := Style{Contract: true}
	mara := Agr{Person: 3}
	they := PronounAgr("they")
	cases := []struct {
		verb, subj string
		a          Agr
		f          Features
		st         Style
		rest, want string
	}{
		{"give", "Mara", mara, Features{"past", "", "positive", "statement"}, c, "Tom a book", "Mara gave Tom a book."},
		{"give", "Mara", mara, Features{"present", "", "positive", "statement"}, c, "Tom a book", "Mara gives Tom a book."},
		{"give", "they", they, Features{"present", "", "positive", "statement"}, c, "Tom a book", "They give Tom a book."},
		{"give", "Mara", mara, Features{"future", "", "positive", "statement"}, c, "Tom a book", "Mara will give Tom a book."},
		{"give", "Mara", mara, Features{"past", "", "negative", "statement"}, c, "Tom a book", "Mara didn't give Tom a book."},
		{"give", "Mara", mara, Features{"past", "", "negative", "statement"}, Style{}, "Tom a book", "Mara did not give Tom a book."},
		{"give", "Mara", mara, Features{"present", "", "negative", "statement"}, c, "Tom a book", "Mara doesn't give Tom a book."},
		{"give", "Mara", mara, Features{"future", "", "negative", "statement"}, c, "Tom a book", "Mara won't give Tom a book."},
		{"give", "Mara", mara, Features{"past", "", "positive", "question"}, c, "Tom a book", "Did Mara give Tom a book?"},
		{"give", "Mara", mara, Features{"past", "", "negative", "question"}, c, "Tom a book", "Didn't Mara give Tom a book?"},
		{"give", "Mara", mara, Features{"past", "", "negative", "question"}, Style{}, "Tom a book", "Did Mara not give Tom a book?"},
		{"give", "Mara", mara, Features{"present", "progressive", "positive", "statement"}, c, "Tom a book", "Mara is giving Tom a book."},
		{"give", "they", they, Features{"past", "progressive", "positive", "statement"}, c, "Tom a book", "They were giving Tom a book."},
		{"give", "Mara", mara, Features{"present", "perfect", "positive", "statement"}, c, "Tom a book", "Mara has given Tom a book."},
		{"give", "Mara", mara, Features{"present", "perfect", "negative", "question"}, c, "Tom a book", "Hasn't Mara given Tom a book?"},
		{"give", "Mara", mara, Features{"future", "perfect", "positive", "statement"}, c, "Tom a book", "Mara will have given Tom a book."},
		{"give", "", mara, Features{"present", "", "positive", "command"}, c, "Tom a book", "Give Tom a book."},
		{"give", "", mara, Features{"present", "", "negative", "command"}, c, "Tom a book", "Don't give Tom a book."},
		// the copula (04/C1b)
		{"be", "the book", mara, Features{"present", "", "positive", "statement"}, c, "old", "The book is old."},
		{"be", "the book", mara, Features{"present", "", "negative", "statement"}, c, "old", "The book isn't old."},
		{"be", "the book", mara, Features{"past", "", "positive", "question"}, c, "old", "Was the book old?"},
		{"be", "the book", mara, Features{"future", "", "positive", "statement"}, c, "old", "The book will be old."},
		{"be", "the books", Agr{Person: 3, Plural: true}, Features{"past", "", "negative", "statement"}, c, "old", "The books weren't old."},
		{"be", "I", Agr{Person: 1}, Features{"present", "", "negative", "statement"}, c, "old", "I am not old."},
		{"be", "they", they, Features{"present", "", "positive", "question"}, c, "tall", "Are they tall?"},
		// main-verb have takes do (US)
		{"have", "Tom", mara, Features{"present", "", "positive", "question"}, c, "a book", "Does Tom have a book?"},
		{"have", "Tom", mara, Features{"present", "", "positive", "statement"}, c, "a book", "Tom has a book."},
	}
	for _, tc := range cases {
		if got := say(t, l, tc.verb, tc.subj, tc.a, tc.f, tc.st, tc.rest); got != tc.want {
			t.Errorf("%s %+v: got %q, want %q", tc.verb, tc.f, got, tc.want)
		}
	}
}

func TestArticles(t *testing.T) {
	l := testLex()
	for _, c := range []struct{ in, want string }{
		{"Tom has a umbrella", "Tom has an umbrella."},
		{"it took an hour", "It took an hour."},
		{"she went to an university", "She went to a university."},
		{"a apple fell", "An apple fell."}, // unknown word: by letter
	} {
		if got := Sentence(l, strings.Fields(c.in), "."); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
}

func TestPronouns(t *testing.T) {
	if p, _ := Pronoun("they", "refl"); p != "themselves" {
		t.Error(p)
	}
	if s, c, ok := PronounCase("his"); !ok || s != "he" || c != "det" {
		t.Error(s, c)
	}
	if !PronounAgr("they").Plural {
		t.Error("singular they takes the plural verb")
	}
}
