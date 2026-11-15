// Package realize turns lemmas and features into English words: verb groups
// with auxiliaries, negation and question order (design/synthetic-data/04/C1,
// 05/H6), pronouns by case (05/H4), and articles by sound (05/H5).
package realize

import (
	"fmt"
	"strings"

	"ganglion/internal/lexicon"
)

// Features of an event (02/L4, 05/V1).
type Features struct {
	Tense    string `json:"tense"`    // past, present, future
	Aspect   string `json:"aspect"`   // simple, progressive, perfect ("" is simple)
	Polarity string `json:"polarity"` // positive, negative
	Mood     string `json:"mood"`     // statement, question, command
}

// Agr is what the verb agrees with: the subject's person and number.
type Agr struct {
	Person int // 1, 2, 3 (0 means 3)
	Plural bool
}

func (a Agr) third() bool { return a.Person == 0 || a.Person == 3 }

// Style holds choices that aren't meaning (05/D77).
type Style struct {
	Contract bool // didn't vs did not
}

// VG is a realised verb group. In a statement the words are Aux, Neg, Rest; in
// a question Aux moves before the subject. Contracted negation is folded into Aux.
type VG struct {
	Aux  string   // the finite auxiliary, or "" when the main verb itself is finite
	Neg  string   // "not" when negative and not contracted
	Rest []string // the rest, ending with the main verb
}

// Statement returns the group's words in statement order.
func (v VG) Statement() []string {
	var out []string
	if v.Aux != "" {
		out = append(out, v.Aux)
	}
	if v.Neg != "" {
		out = append(out, v.Neg)
	}
	return append(out, v.Rest...)
}

// QuestionParts returns what goes before the subject and what goes after it.
func (v VG) QuestionParts() (before []string, after []string) {
	if v.Neg != "" {
		after = append(after, v.Neg)
	}
	return []string{v.Aux}, append(after, v.Rest...)
}

func beForm(tense string, a Agr) string {
	switch {
	case tense == "past" && (a.Plural || a.Person == 2):
		return "were"
	case tense == "past":
		return "was"
	case a.Person == 1 && !a.Plural:
		return "am"
	case a.Plural || a.Person == 2:
		return "are"
	}
	return "is"
}

func haveForm(tense string, a Agr) string {
	if tense == "past" {
		return "had"
	}
	if a.third() && !a.Plural {
		return "has"
	}
	return "have"
}

func doForm(tense string, a Agr) string {
	if tense == "past" {
		return "did"
	}
	if a.third() && !a.Plural {
		return "does"
	}
	return "do"
}

var contractions = map[string]string{
	"is": "isn't", "are": "aren't", "was": "wasn't", "were": "weren't", "do": "don't", "does": "doesn't",
	"did": "didn't", "has": "hasn't", "have": "haven't", "had": "hadn't", "will": "won't",
	// "am" has no contraction with not in statements; "am not" stays
}

// VerbGroup realises a verb (05/H6). The copula negates and inverts without
// "do" (04/C1b); main-verb "have" uses "do" (US, 05/D77).
func VerbGroup(e *lexicon.Entry, f Features, a Agr, st Style) (VG, error) {
	if e == nil || e.POS != "verb" {
		return VG{}, fmt.Errorf("not a verb entry")
	}
	base := e.Lemma
	copula := base == "be"
	neg := f.Polarity == "negative"
	question := f.Mood == "question"
	aspect := f.Aspect
	if aspect == "" {
		aspect = "simple"
	}

	if f.Mood == "command" {
		if !neg {
			return VG{Rest: []string{base}}, nil
		}
		if st.Contract {
			return VG{Aux: "don't", Rest: []string{base}}, nil
		}
		return VG{Aux: "do", Neg: "not", Rest: []string{base}}, nil
	}

	// the chain of verbs, first one finite: e.g. [will have given]
	var chain []string
	finite := func(lemma string) string { // a finite form of be/have/do or the main verb
		switch lemma {
		case "be":
			return beForm(f.Tense, a)
		case "have":
			return haveForm(f.Tense, a)
		case "do":
			return doForm(f.Tense, a)
		}
		if f.Tense == "past" {
			return e.Forms["past"]
		}
		if a.third() && !a.Plural {
			return e.Forms["pres3"]
		}
		return base
	}
	switch aspect {
	case "simple":
		switch {
		case f.Tense == "future":
			chain = []string{"will", base}
		case copula:
			chain = []string{finite("be")}
		case neg || question:
			chain = []string{finite("do"), base} // do-support
		default:
			chain = []string{finite(base)}
		}
	case "progressive":
		if f.Tense == "future" {
			chain = []string{"will", "be", e.Forms["ing"]}
		} else {
			chain = []string{finite("be"), e.Forms["ing"]}
		}
	case "perfect":
		if f.Tense == "future" {
			chain = []string{"will", "have", e.Forms["pastp"]}
		} else {
			chain = []string{finite("have"), e.Forms["pastp"]}
		}
	default:
		return VG{}, fmt.Errorf("aspect %q not supported yet (05/V2)", f.Aspect)
	}
	for _, w := range chain {
		if w == "" {
			return VG{}, fmt.Errorf("%s has no form for %v", base, f)
		}
	}

	// a lone finite main verb (gave, is) with nothing to move or negate
	if len(chain) == 1 && !neg && !question {
		return VG{Rest: chain}, nil
	}
	if len(chain) == 1 { // only the copula reaches here
		v := VG{Aux: chain[0]}
		if neg {
			if c, ok := contractions[v.Aux]; ok && st.Contract {
				v.Aux = c
			} else {
				v.Neg = "not"
			}
		}
		return v, nil
	}
	v := VG{Aux: chain[0], Rest: chain[1:]}
	if neg {
		if c, ok := contractions[v.Aux]; ok && st.Contract {
			v.Aux = c
		} else {
			v.Neg = "not"
		}
	}
	return v, nil
}

// Pronoun sets (05/H4); the set is named on the entity (09/Z4a).
var pronouns = map[string]map[string]string{
	"she":  {"subj": "she", "obj": "her", "det": "her", "pos": "hers", "refl": "herself"},
	"he":   {"subj": "he", "obj": "him", "det": "his", "pos": "his", "refl": "himself"},
	"they": {"subj": "they", "obj": "them", "det": "their", "pos": "theirs", "refl": "themselves"},
	"it":   {"subj": "it", "obj": "it", "det": "its", "pos": "its", "refl": "itself"},
}

// Pronoun returns a pronoun set's form for a case: subj, obj, det, pos, refl.
func Pronoun(set, c string) (string, bool) {
	p, ok := pronouns[set][c]
	return p, ok
}

// PronounAgr is how a pronoun set agrees with a verb: singular "they" takes the
// plural form ("they are").
func PronounAgr(set string) Agr { return Agr{Person: 3, Plural: set == "they"} }

// PronounCase finds which set and case a word is, for alignment.
func PronounCase(word string) (set, c string, ok bool) {
	w := strings.ToLower(word)
	for _, s := range []string{"she", "he", "they", "it"} { // a fixed order, so "her" is always she/obj first
		for _, cs := range []string{"subj", "obj", "det", "pos", "refl"} {
			if pronouns[s][cs] == w {
				return s, cs, true
			}
		}
	}
	return "", "", false
}

// Indefinite returns "a" or "an" for the word that follows it, by sound when
// the lexicon knows the word (05/H5).
func Indefinite(lex *lexicon.Lexicon, next string) string {
	w := strings.ToLower(strings.Fields(next + " x")[0])
	for _, pos := range []string{"noun", "adj"} {
		if e := lex.Get(pos, w); e != nil {
			if e.An {
				return "an"
			}
			return "a"
		}
	}
	if w != "" && strings.ContainsRune("aeiou", rune(w[0])) {
		return "an"
	}
	return "a"
}

// Sentence joins words, fixes a/an, capitalises and ends the sentence (04/N2).
func Sentence(lex *lexicon.Lexicon, words []string, end string) string {
	var ws []string
	for _, w := range words {
		if w != "" {
			ws = append(ws, w)
		}
	}
	for i := 0; i+1 < len(ws); i++ {
		if lw := strings.ToLower(ws[i]); lw == "a" || lw == "an" {
			ws[i] = Indefinite(lex, ws[i+1])
		}
	}
	s := strings.Join(ws, " ")
	for _, p := range []string{",", ".", "!", "?", ";", ":"} {
		s = strings.ReplaceAll(s, " "+p, p)
	}
	s = strings.TrimRight(s, ".!? ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:] + end
}
