// Package lexicon builds and reads the word list the generator inflects from
// (design/synthetic-data/05, shapes in 09/Z6). Rules propose forms, exception
// data overrides them, and generation only looks forms up (05/D74).
package lexicon

import "strings"

// Pron is a word's pronunciation from CMUdict: ARPAbet phones, vowels carry a
// stress digit (0 none, 1 primary, 2 secondary). Nil when unknown.
type Pron []string

func isVowelPhone(p string) bool { return len(p) > 0 && strings.ContainsAny(p[len(p)-1:], "012") }

// Syllables counts vowel phones; -1 when the pronunciation is unknown.
func (p Pron) Syllables() int {
	if p == nil {
		return -1
	}
	n := 0
	for _, ph := range p {
		if isVowelPhone(ph) {
			n++
		}
	}
	return n
}

// FinalStressed reports whether the last syllable carries primary stress.
func (p Pron) FinalStressed() bool {
	for i := len(p) - 1; i >= 0; i-- {
		if isVowelPhone(p[i]) {
			return strings.HasSuffix(p[i], "1")
		}
	}
	return false
}

// StartsWithVowelSound decides "an" (05/H5): by the first phone, not the letter.
func (p Pron) StartsWithVowelSound() bool { return len(p) > 0 && isVowelPhone(p[0]) }

func isVowel(c byte) bool { return strings.IndexByte("aeiou", c) >= 0 }

// lastWord and firstWord: multiword entries inflect one word (nouns the last,
// verbs the first: "ice creams", "gives up").
func lastWord(s string) (string, string) {
	if i := strings.LastIndexByte(s, ' '); i >= 0 {
		return s[:i+1], s[i+1:]
	}
	return "", s
}
func firstWord(s string) (string, string) {
	if i := strings.IndexByte(s, ' '); i >= 0 {
		return s[:i], s[i:]
	}
	return s, ""
}

// Plural proposes a noun's plural. doubt is true when the rule is guessing.
func Plural(noun string) (string, bool) {
	pre, w := lastWord(noun)
	n := len(w)
	switch {
	case n == 0:
		return noun, true
	case strings.HasSuffix(w, "s"), strings.HasSuffix(w, "x"), strings.HasSuffix(w, "z"), strings.HasSuffix(w, "ch"), strings.HasSuffix(w, "sh"):
		return pre + w + "es", strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss") // "bus" is fine, "lens"/"species" may not be
	case n > 1 && w[n-1] == 'y' && !isVowel(w[n-2]):
		return pre + w[:n-1] + "ies", false
	case n > 1 && w[n-1] == 'o' && !isVowel(w[n-2]):
		return pre + w + "s", true // potatoes vs pianos
	case strings.HasSuffix(w, "f"), strings.HasSuffix(w, "fe"):
		return pre + w + "s", true // roofs vs leaves; leaves are in the exception lists
	}
	return pre + w + "s", false
}

// doubles reports whether a verb or adjective doubles its last consonant
// before -ed/-ing/-er (stop/stopped, commit/committed, but visit/visited).
func doubles(w string, p Pron) (bool, bool) {
	n := len(w)
	if n < 3 {
		return false, false
	}
	c3, v, c := w[n-3], w[n-2], w[n-1]
	quC := c3 == 'u' && n >= 4 && w[n-4] == 'q' // the u in "qu" acts as a consonant: quit/quitting
	if isVowel(c) || strings.IndexByte("wxy", c) >= 0 || !isVowel(v) || (isVowel(c3) && !quC) {
		return false, false // not consonant-vowel-consonant
	}
	syl := p.Syllables()
	switch {
	case syl == 1:
		return true, false
	case syl > 1:
		return p.FinalStressed(), false
	}
	// unknown pronunciation: guess by spelling, one vowel group means one syllable
	groups := 0
	for i := 0; i < n; i++ {
		if isVowel(w[i]) && (i == 0 || !isVowel(w[i-1])) {
			groups++
		}
	}
	return groups == 1, groups != 1
}

// VerbForms proposes third-person present, past, past participle and -ing.
func VerbForms(verb string, p Pron) (pres3, past, ing string, doubt bool) {
	w, rest := firstWord(verb)
	n := len(w)
	switch {
	case strings.HasSuffix(w, "s"), strings.HasSuffix(w, "x"), strings.HasSuffix(w, "z"), strings.HasSuffix(w, "ch"), strings.HasSuffix(w, "sh"), strings.HasSuffix(w, "o"):
		pres3 = w + "es"
	case n > 1 && w[n-1] == 'y' && !isVowel(w[n-2]):
		pres3 = w[:n-1] + "ies"
	default:
		pres3 = w + "s"
	}
	dbl, d := doubles(w, p)
	doubt = d
	switch {
	case strings.HasSuffix(w, "ie"):
		past, ing = w+"d", w[:n-2]+"ying"
	case strings.HasSuffix(w, "ee"), strings.HasSuffix(w, "ye"), strings.HasSuffix(w, "oe"):
		past, ing = w+"d", w+"ing"
	case strings.HasSuffix(w, "e"):
		past, ing = w+"d", w[:n-1]+"ing"
	case n > 1 && w[n-1] == 'y' && !isVowel(w[n-2]):
		past, ing = w[:n-1]+"ied", w+"ing"
	case strings.HasSuffix(w, "ic"):
		past, ing = w+"ked", w+"king"
	case dbl:
		past, ing = w+w[n-1:]+"ed", w+w[n-1:]+"ing"
	default:
		past, ing = w+"ed", w+"ing"
	}
	return pres3 + rest, past + rest, ing + rest, doubt
}

// Comparative proposes comparative and superlative; more is true when the
// adjective takes "more"/"most" instead (beautiful).
func Comparative(adj string, p Pron) (er, est string, more, doubt bool) {
	if strings.Contains(adj, " ") || strings.Contains(adj, "-") {
		return "", "", true, false
	}
	w, n := adj, len(adj)
	syl := p.Syllables()
	if syl < 0 {
		doubt = true
		syl = 1
		for i := 1; i < n; i++ {
			if isVowel(w[i]) && !isVowel(w[i-1]) {
				syl++
			}
		}
		if strings.HasSuffix(w, "e") && syl > 1 {
			syl--
		}
	}
	switch {
	case syl == 1:
	case syl == 2 && strings.HasSuffix(w, "y"):
	case syl == 2 && (strings.HasSuffix(w, "le") || strings.HasSuffix(w, "ow") || strings.HasSuffix(w, "er")):
		doubt = true // simpler, narrower, cleverer: both styles occur
	default:
		return "", "", true, doubt
	}
	dbl, d := doubles(w, p)
	switch {
	case strings.HasSuffix(w, "e"):
		er, est = w+"r", w+"st"
	case n > 1 && w[n-1] == 'y' && !isVowel(w[n-2]):
		er, est = w[:n-1]+"ier", w[:n-1]+"iest"
	case dbl:
		er, est = w+w[n-1:]+"er", w+w[n-1:]+"est"
	default:
		er, est = w+"er", w+"est"
	}
	return er, est, false, doubt || d
}
