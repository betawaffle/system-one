package author

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"

	"ganglion/internal/lexicon"
	"ganglion/internal/realize"
)

func sortStrings(s []string) { sort.Strings(s) }

// time modifiers the generator adds when a combination needs context (08/D98)
var timeMods = map[string][]string{
	"future":  {"by next year,", "someday,", "in a few years,"},
	"past":    {"years ago,", "back then,"},
	"present": {"these days,", "by now,"},
}

// Generate realises a template for a meaning: plain code, no model (03/D73).
func Generate(t Template, m Meaning, w *World, lex *lexicon.Lexicon, st realize.Style, r *rand.Rand) (string, string, error) {
	f := m.Features
	byVar := map[string]string{}
	for role, v := range t.Means.Vars {
		byVar[v] = role
	}
	holder := holderType(m, w)
	ent := func(v string) (Entity, error) {
		fl, ok := m.Roles[byVar[v]]
		if !ok || fl.Entity == "" {
			return Entity{}, fmt.Errorf("no entity for %s", byVar[v])
		}
		return w.Entities[fl.Entity], nil
	}
	refWords := func(n Node) (string, realize.Agr, error) {
		e, err := ent(n.Var)
		if err != nil {
			return "", realize.Agr{}, err
		}
		agr := realize.Agr{Person: 3}
		switch n.Form {
		case "name":
			if e.Name == "" {
				return e.Noun, agr, nil
			}
			return e.Name, agr, nil
		case "possname":
			return e.Display() + "'s", agr, nil
		case "noun":
			return e.Noun, agr, nil
		case "plural":
			return plural(lex, e.Noun), realize.Agr{Person: 3, Plural: true}, nil
		case "pron":
			p, ok := realize.Pronoun(e.Pronouns, n.Case)
			if !ok {
				return "", agr, fmt.Errorf("no %s pronoun for %s", n.Case, e.Pronouns)
			}
			return p, realize.PronounAgr(e.Pronouns), nil
		}
		return "", agr, fmt.Errorf("unknown form %q", n.Form)
	}
	descWord := func(n Node) (string, error) {
		v := m.Roles[byVar[n.Var]].Value
		if v == nil {
			return "", fmt.Errorf("no value for %s", byVar[n.Var])
		}
		a := w.Attributes["attribute:"+v.Attribute]
		adj := ""
		if v.Value != "" {
			adj = a.Word(v.Value, holder, w)
		} else {
			lo, hi := a.Ends(holder, w)
			adj = hi
			if (v.Dir == "less") != n.Converse {
				adj = lo
			}
		}
		if n.Form == "" {
			return adj, nil
		}
		if e := lex.Get("adj", adj); e != nil {
			if e.More {
				return map[string]string{"er": "more ", "est": "most "}[n.Form] + adj, nil
			}
			if x := e.Forms[n.Form]; x != "" {
				return x, nil
			}
		}
		return "", fmt.Errorf("no %s form of %s", n.Form, adj)
	}

	// walk the tree: words before the verb group are the subject part
	var before, after []string
	var vg *Node
	subj := realize.Agr{Person: 3}
	var walk func(n Node) error
	walk = func(n Node) error {
		out := &before
		if vg != nil {
			out = &after
		}
		switch n.T {
		case "seq":
			for _, k := range n.Kids {
				if err := walk(k); err != nil {
					return err
				}
			}
		case "word":
			if n.Text != "!" {
				*out = append(*out, n.Text)
			}
		case "lex":
			nn := n
			vg = &nn
		case "slot":
			var s string
			var err error
			if n.Kind == "DESC" {
				s, err = descWord(n)
			} else {
				var a realize.Agr
				s, a, err = refWords(n)
				if n.Label == "subj" {
					subj = a
				}
			}
			if err != nil {
				return err
			}
			*out = append(*out, s)
		}
		return nil
	}
	if err := walk(t.Tree); err != nil {
		return "", "", err
	}

	var words []string
	end := "."
	if vg == nil { // frozen: the verb words are plain words already
		words = append(before, after...)
	} else {
		e := lex.Get("verb", vg.Lemma)
		g, err := realize.VerbGroup(e, f, subj, st)
		if err != nil {
			return "", "", err
		}
		if f.Mood == "question" {
			b, a := g.QuestionParts()
			words = append(append(append(append([]string{}, b...), lowerFirst(before, w)...), a...), after...)
			end = "?"
		} else {
			words = append(append(append([]string{}, before...), g.Statement()...), after...)
		}
	}
	for _, a := range t.Means.Adds {
		if a.Key == "emphatic" || a.Text == "!" {
			if end == "." {
				end = "!"
			}
		}
	}
	time := ""
	for _, nd := range t.Needs {
		if nd.Tense == f.Tense && nd.Modifier == "time" && !w.Stable(m) {
			opts := timeMods[f.Tense]
			time = opts[r.IntN(len(opts))]
			words = append(strings.Fields(time), lowerFirst(words, w)...)
		}
	}
	return realize.Sentence(lex, words, end), strings.TrimSuffix(time, ","), nil
}

func plural(lex *lexicon.Lexicon, noun string) string {
	if e := lex.Get("noun", noun); e != nil && e.Forms["plural"] != "" {
		return e.Forms["plural"]
	}
	p, _ := lexicon.Plural(noun)
	return p
}

// lowerFirst lowercases the first word unless it's a name (or "I").
func lowerFirst(ws []string, w *World) []string {
	if len(ws) == 0 || ws[0] == "" {
		return ws
	}
	out := append([]string{}, ws...)
	if !w.IsName(out[0]) {
		out[0] = strings.ToLower(out[0][:1]) + out[0][1:]
	}
	return out
}

// Sample is one generated preview (08/W6).
type Sample struct {
	Text     string            `json:"text"`
	Features realize.Features  `json:"features"`
	Fillers  map[string]string `json:"fillers"` // role -> display
	Changed  []string          `json:"changed"` // which tags differ from the authored meaning
	Time     string            `json:"time,omitempty"`
}

// Samples draws n previews spread across fillers and supported features (03/A16a).
func Samples(t Template, m Meaning, w *World, lex *lexicon.Lexicon, seed uint64, n int) []Sample {
	r := rand.New(rand.NewPCG(seed, 0x9e3779b97f4a7c15))
	f := w.Frames[m.Frame]
	var out []Sample
	seen := map[string]bool{}
	tenses := t.Supports.Tense
	if w.Stable(m) {
		tenses = []string{"present"} // 06/D100
	}
	for i := 0; len(out) < n && i < n*6; i++ {
		mm := m
		mm.Roles = map[string]Filler{}
		used := map[string]bool{}
		for role, fl := range m.Roles {
			mm.Roles[role] = fl
			if fl.Entity != "" {
				used[fl.Entity] = true
			}
		}
		if i > 0 {
			used = map[string]bool{}
			for _, ro := range f.Roles {
				fl := m.Roles[ro.Name]
				if fl.Entity == "" || w.Entities[fl.Entity].Coerced != "" {
					continue
				}
				pool := w.EntitiesOf(ro.Type)
				if ro.Name == "holder" || ro.Name == "a" || ro.Name == "b" {
					pool = fitting(pool, m, w) // the attribute must apply (06/P2)
				}
				var free []string
				for _, id := range pool {
					if !used[id] && w.Entities[id].Type == w.Entities[fl.Entity].Type {
						free = append(free, id)
					}
				}
				if len(free) > 0 {
					fl.Entity = free[r.IntN(len(free))]
				}
				used[fl.Entity] = true
				mm.Roles[ro.Name] = fl
			}
			mm.Features.Tense = tenses[i%len(tenses)]
			mm.Features.Polarity = t.Supports.Polarity[0]
			if i%4 == 3 && len(t.Supports.Polarity) > 1 {
				mm.Features.Polarity = t.Supports.Polarity[1]
			}
			mm.Features.Mood = t.Supports.Mood[0]
			if i%5 == 4 && len(t.Supports.Mood) > 1 {
				mm.Features.Mood = t.Supports.Mood[len(t.Supports.Mood)-1]
			}
		} else if len(t.Frozen) > 0 {
			mm.Features.Tense, mm.Features.Polarity, mm.Features.Mood = t.Supports.Tense[0], t.Supports.Polarity[0], t.Supports.Mood[0]
		}
		if !w.Allowed(mm) {
			continue
		}
		text, time, err := Generate(t, mm, w, lex, realize.Style{Contract: true}, r)
		if err != nil || seen[text] {
			continue
		}
		seen[text] = true
		s := Sample{Text: text, Features: mm.Features, Fillers: map[string]string{}, Changed: []string{}, Time: time}
		for _, k := range []string{"tense", "polarity", "mood"} {
			if feat(mm.Features, k) != feat(m.Features, k) {
				s.Changed = append(s.Changed, k)
			}
		}
		for role, fl := range mm.Roles {
			if fl.Entity != "" {
				s.Fillers[role] = w.Entities[fl.Entity].Display()
				if fl.Entity != m.Roles[role].Entity {
					s.Changed = append(s.Changed, role)
				}
			}
		}
		out = append(out, s)
	}
	return out
}

func fitting(pool []string, m Meaning, w *World) []string {
	for _, fl := range m.Roles {
		if fl.Value == nil {
			continue
		}
		a := w.Attributes["attribute:"+fl.Value.Attribute]
		var out []string
		for _, id := range pool {
			for _, t := range a.AppliesTo {
				if w.IsA(w.Entities[id].Type, t) {
					out = append(out, id)
					break
				}
			}
		}
		return out
	}
	return pool
}

func feat(f realize.Features, k string) string {
	switch k {
	case "tense":
		return f.Tense
	case "polarity":
		return f.Polarity
	}
	return f.Mood
}

func newRand(seed uint64) *rand.Rand { return rand.New(rand.NewPCG(seed, 1)) }
