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
		case "noun", "plural":
			pl := n.Form == "plural"
			word := e.Noun
			if pl {
				word, agr = plural(lex, e.Noun), realize.Agr{Person: 3, Plural: true}
			}
			det := m.Roles[byVar[n.Var]].Det
			if det == "" {
				det = n.Det
			}
			return detWord(det, pl) + word, agr, nil
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
			*out = append(*out, strings.Fields(s)...) // separate words, so a/an gets fixed
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

// detWord realises a reference setting before a noun; Sentence fixes a/an.
func detWord(det string, plural bool) string {
	switch {
	case det == "":
		return ""
	case plural && det == "a":
		return ""
	case plural && det == "this":
		return "these "
	case plural && det == "that":
		return "those "
	}
	return det + " "
}

// detSlots finds REF noun slots that carry a determiner, by role.
func detSlots(t Template) map[string]Node {
	byVar := map[string]string{}
	for role, v := range t.Means.Vars {
		byVar[v] = role
	}
	out := map[string]Node{}
	var walk func(n Node)
	walk = func(n Node) {
		if n.T == "slot" && n.Kind == "REF" && n.Det != "" {
			out[byVar[n.Var]] = n
		}
		for _, k := range n.Kids {
			walk(k)
		}
	}
	walk(t.Tree)
	return out
}

// detOptions are the reference settings a slot may vary over. "a" is left out
// for the subject of a "be" frame: "An apple is old" reads as a statement about
// apples in general (04/F4).
func detOptions(n Node, f Frame) []string {
	for _, v := range f.Verbs {
		if v == "be" && n.Label == "subj" {
			return []string{"the", "this", "that"}
		}
	}
	return []string{"the", "a", "this", "that"}
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
	Implies  []string          `json:"implies,omitempty"` // what a reader infers beyond the tags (02/L9)
}

// Samples draws n previews spread across fillers and supported features (03/A16a).
func Samples(t Template, m Meaning, w *World, lex *lexicon.Lexicon, seed uint64, n int) []Sample {
	r := rand.New(rand.NewPCG(seed, 0x9e3779b97f4a7c15))
	f := w.Frames[m.Frame]
	var out []Sample
	seen := map[string]bool{}
	dets := detSlots(t)
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
				if Bears(ro.Name) {
					pool = Fitting(pool, m, w) // the attribute must apply (06/P2)
				}
				var free []string
				for _, id := range pool {
					if !used[id] && w.KindOf(w.Entities[id].Type) == w.KindOf(w.Entities[fl.Entity].Type) {
						free = append(free, id)
					}
				}
				if len(free) > 0 {
					fl.Entity = free[r.IntN(len(free))]
				}
				used[fl.Entity] = true
				if n, ok := dets[ro.Name]; ok && i%3 != 0 {
					opts := detOptions(n, f)
					fl.Det = opts[r.IntN(len(opts))]
				}
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
		implies, skip := w.Implications(t, mm)
		if skip {
			continue
		}
		text, time, err := Generate(t, mm, w, lex, realize.Style{Contract: true}, r)
		if err != nil || seen[text] {
			continue
		}
		seen[text] = true
		s := Sample{Text: text, Features: mm.Features, Fillers: map[string]string{}, Changed: []string{}, Time: time, Implies: implies}
		for _, k := range []string{"tense", "polarity", "mood"} {
			if feat(mm.Features, k) != feat(m.Features, k) {
				s.Changed = append(s.Changed, k)
			}
		}
		for role, fl := range mm.Roles {
			if fl.Entity != "" {
				s.Fillers[role] = w.Entities[fl.Entity].Display()
				det := fl.Det
				if n, ok := dets[role]; ok {
					if det == "" {
						det = n.Det
					}
					s.Fillers[role] = det + " " + s.Fillers[role]
				}
				authored := m.Roles[role].Det
				if authored == "" {
					authored = dets[role].Det
				}
				if fl.Entity != m.Roles[role].Entity || det != authored {
					s.Changed = append(s.Changed, role)
				}
			}
		}
		out = append(out, s)
	}
	return out
}

// Fitting keeps the entities the meaning's attribute value applies to.
func Fitting(pool []string, m Meaning, w *World) []string {
	for _, fl := range m.Roles {
		if fl.Value == nil {
			continue
		}
		a := w.Attributes["attribute:"+fl.Value.Attribute]
		var out []string
		for _, id := range pool {
			t := w.Entities[id].Type
			if a.Applies(t, w) && (fl.Value.Value == "" || a.HasWord(fl.Value.Value, t, w)) {
				out = append(out, id)
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
