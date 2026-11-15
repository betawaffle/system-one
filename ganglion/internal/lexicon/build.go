package lexicon

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Sources are the raw downloads an import reads (05/S1): WordNet's dict
// directory and CMUdict's dictionary file. Both are permissively licensed.
type Sources struct {
	WordNetDict string
	CMUDict     string
}

// ReadCMU loads first pronunciations, lowercased word -> phones.
func ReadCMU(path string) (map[string]Pron, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]Pron{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		fs := strings.Fields(line)
		if len(fs) < 2 || strings.Contains(fs[0], "(") { // "word(2)" is an alternate
			continue
		}
		w := strings.ToLower(fs[0])
		if _, ok := out[w]; !ok {
			out[w] = Pron(fs[1:])
		}
	}
	return out, sc.Err()
}

// readIndex lists WordNet lemmas for one part of speech ("noun", "verb", "adj").
func readIndex(dict, pos string) ([]string, error) {
	f, err := os.Open(filepath.Join(dict, "index."+pos))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, " ") || line == "" { // the licence header
			continue
		}
		lemma := strings.ReplaceAll(strings.Fields(line)[0], "_", " ")
		if strings.ContainsAny(lemma, "0123456789.") {
			continue
		}
		out = append(out, lemma)
	}
	return out, sc.Err()
}

// readExc maps base -> inflected forms from a WordNet exception list.
func readExc(dict, pos string) (map[string][]string, error) {
	f, err := os.Open(filepath.Join(dict, pos+".exc"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string][]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fs := strings.Fields(sc.Text())
		if len(fs) < 2 {
			continue
		}
		form := strings.ReplaceAll(fs[0], "_", " ")
		for _, base := range fs[1:] {
			base = strings.ReplaceAll(base, "_", " ")
			out[base] = append(out[base], form)
		}
	}
	return out, sc.Err()
}

func pronOf(cmu map[string]Pron, lemma string) Pron {
	w, _ := firstWord(lemma)
	return cmu[w]
}

func setAn(e *Entry, cmu map[string]Pron) {
	if p := pronOf(cmu, e.Lemma); p != nil {
		e.An = p.StartsWithVowelSound()
		e.Src = append(e.Src, "cmudict")
		return
	}
	e.An = isVowel(e.Lemma[0])
	e.Doubt = append(e.Doubt, "an")
}

// Build makes entries for nouns, verbs and adjectives.
func Build(src Sources) ([]*Entry, error) {
	cmu, err := ReadCMU(src.CMUDict)
	if err != nil {
		return nil, err
	}
	var out []*Entry

	// nouns
	nouns, err := readIndex(src.WordNetDict, "noun")
	if err != nil {
		return nil, err
	}
	nexc, err := readExc(src.WordNetDict, "noun")
	if err != nil {
		return nil, err
	}
	for _, n := range nouns {
		e := &Entry{Lemma: n, POS: "noun", Forms: map[string]string{}, Src: []string{"wordnet:index.noun"}}
		switch {
		case irregularNouns[n] != "":
			e.Forms["plural"] = irregularNouns[n]
			e.Src = append(e.Src, "ganglion:irregular")
		case len(nexc[n]) > 0:
			sort.Strings(nexc[n])
			e.Forms["plural"] = nexc[n][0]
			e.Src = append(e.Src, "wordnet:noun.exc")
			if len(nexc[n]) > 1 {
				e.Doubt = append(e.Doubt, "plural")
			}
		default:
			pl, doubt := Plural(n)
			e.Forms["plural"] = pl
			if doubt {
				e.Doubt = append(e.Doubt, "plural")
			}
		}
		e.Uncount = uncountable[n]
		setAn(e, cmu)
		out = append(out, e)
	}

	// verbs
	verbs, err := readIndex(src.WordNetDict, "verb")
	if err != nil {
		return nil, err
	}
	vexc, err := readExc(src.WordNetDict, "verb")
	if err != nil {
		return nil, err
	}
	for _, v := range verbs {
		e := &Entry{Lemma: v, POS: "verb", Forms: map[string]string{}, Src: []string{"wordnet:index.verb"}}
		pres3, past, ing, doubt := VerbForms(v, pronOf(cmu, v))
		pastp := past
		w, rest := firstWord(v)
		if irr, ok := irregularVerbs[w]; ok {
			past, pastp = irr[0]+rest, irr[1]+rest
			doubt = false
			e.Src = append(e.Src, "ganglion:irregular")
		} else if forms := vexc[v]; len(forms) > 0 {
			e.Src = append(e.Src, "wordnet:verb.exc")
			var others []string
			for _, f := range forms {
				switch {
				case strings.HasSuffix(f, "ing"):
					ing = f
				case f == pres3:
				default:
					others = append(others, f)
				}
			}
			sort.Strings(others)
			switch len(others) {
			case 0:
			case 1:
				past, pastp = others[0], others[0]
			default:
				// the participle usually ends in n (given, swollen); otherwise we're guessing
				var ns, rest []string
				for _, f := range others {
					if strings.HasSuffix(f, "n") {
						ns = append(ns, f)
					} else {
						rest = append(rest, f)
					}
				}
				if len(ns) == 1 && len(rest) >= 1 {
					past, pastp = rest[0], ns[0]
				} else {
					past, pastp = others[0], others[1]
				}
				doubt = true
			}
		}
		if p3, ok := irregularPres3[w]; ok {
			pres3 = p3 + rest
		}
		e.Forms["pres3"], e.Forms["past"], e.Forms["pastp"], e.Forms["ing"] = pres3, past, pastp, ing
		if doubt {
			e.Doubt = append(e.Doubt, "past", "pastp", "ing")
		}
		out = append(out, e)
	}

	// adjectives
	adjs, err := readIndex(src.WordNetDict, "adj")
	if err != nil {
		return nil, err
	}
	aexc, err := readExc(src.WordNetDict, "adj")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, a := range adjs {
		a = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(a, "(a)"), "(p)"), "(ip)")
		if seen[a] {
			continue
		}
		seen[a] = true
		e := &Entry{Lemma: a, POS: "adj", Forms: map[string]string{}, Src: []string{"wordnet:index.adj"}}
		if irr, ok := irregularAdj[a]; ok {
			e.Forms["er"], e.Forms["est"] = irr[0], irr[1]
			e.Src = append(e.Src, "ganglion:irregular")
		} else {
			er, est, more, doubt := Comparative(a, pronOf(cmu, a))
			for _, f := range aexc[a] {
				if strings.HasSuffix(f, "est") {
					est, more = f, false
				} else if strings.HasSuffix(f, "er") {
					er, more = f, false
				}
			}
			if len(aexc[a]) > 0 {
				e.Src = append(e.Src, "wordnet:adj.exc")
				doubt = false
			}
			e.More = more
			if !more {
				e.Forms["er"], e.Forms["est"] = er, est
			}
			if doubt {
				e.Doubt = append(e.Doubt, "er", "est")
			}
		}
		setAn(e, cmu)
		out = append(out, e)
	}

	// the copula and auxiliaries aren't in WordNet's verb index the way we need them
	for _, e := range out {
		if e.POS == "verb" && e.Lemma == "be" {
			e.Forms["pres3"], e.Forms["past"], e.Forms["pastp"], e.Forms["ing"] = "is", "was", "been", "being"
		}
	}
	return out, nil
}
