package author

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"

	"ganglion/internal/lexicon"
	"ganglion/internal/realize"
)

// Coverage (design/synthetic-data/08/W9): the kinds of sample a template can
// make, so the user can see which they've judged. One verdict covers every
// sentence of its kind, so the question is "did I check every kind", not
// "every sentence".

// Dim is one way samples of a template vary.
type Dim struct {
	Name   string   `json:"name"`   // verb, ref:<role>, who:<role>
	Title  string   `json:"title"`  // for the page
	Values []string `json:"values"` // e.g. "past positive statement", "this", "they"
}

// Dims lists a template's dimensions for a meaning like the one it was written for.
func Dims(t Template, m Meaning, w *World) []Dim {
	var out []Dim
	verb := Dim{Name: "verb", Title: "Verb form"}
	for _, te := range t.Supports.Tense {
		for _, po := range t.Supports.Polarity {
			for _, mo := range t.Supports.Mood {
				mm := m
				mm.Features = realize.Features{Tense: te, Aspect: "simple", Polarity: po, Mood: mo}
				if w.Stable(m) && te != "present" {
					continue
				}
				if !w.Allowed(mm) {
					continue
				}
				verb.Values = append(verb.Values, te+" "+po+" "+mo)
			}
		}
	}
	out = append(out, verb)
	f := w.Frames[m.Frame]
	dets := detSlots(t)
	for _, ro := range f.Roles {
		if n, ok := dets[ro.Name]; ok {
			out = append(out, Dim{Name: "ref:" + ro.Name, Title: "How " + ro.Name + " is referred to", Values: detOptions(n, f)})
		}
	}
	for _, ro := range f.Roles {
		fl := m.Roles[ro.Name]
		if fl.Entity == "" || w.Entities[fl.Entity].Name == "" {
			continue
		}
		sets := map[string]bool{}
		for _, id := range w.EntitiesOf(ro.Type) {
			if e := w.Entities[id]; e.Name != "" {
				sets[e.Pronouns] = true
			}
		}
		d := Dim{Name: "who:" + ro.Name, Title: cap1(ro.Name) + "'s pronouns"}
		for s := range sets {
			d.Values = append(d.Values, s)
		}
		sort.Strings(d.Values)
		if len(d.Values) > 1 {
			out = append(out, d)
		}
	}
	return out
}

func cap1(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// ComboOf says which value of each dimension a sample has.
func ComboOf(s Sample, w *World) map[string]string {
	c := map[string]string{"verb": s.Features.Tense + " " + s.Features.Polarity + " " + s.Features.Mood}
	for role, disp := range s.Fillers {
		parts := strings.Fields(disp)
		if len(parts) > 1 {
			if _, ok := determiners[strings.ToLower(parts[0])]; ok {
				c["ref:"+role] = strings.ToLower(parts[0])
				disp = strings.Join(parts[1:], " ")
			}
		}
		for _, e := range w.Entities {
			if e.Name != "" && e.Name == disp {
				c["who:"+role] = e.Pronouns
			}
		}
	}
	return c
}

// Draw makes one sample with some dimension values forced, the rest random.
func Draw(t Template, m Meaning, w *World, lex *lexicon.Lexicon, r *rand.Rand, want map[string]string) (Sample, error) {
	f := w.Frames[m.Frame]
	dets := detSlots(t)
	mm := m
	mm.Roles = map[string]Filler{}
	used := map[string]bool{}
	for _, ro := range f.Roles {
		fl := m.Roles[ro.Name]
		if fl.Entity != "" && w.Entities[fl.Entity].Coerced == "" {
			cur := w.Entities[fl.Entity]
			pool := w.EntitiesOf(ro.Type)
			if ro.Name == "holder" || ro.Name == "a" || ro.Name == "b" {
				pool = fitting(pool, m, w)
			}
			var free []string
			for _, id := range pool {
				e := w.Entities[id]
				if used[id] || e.Type != cur.Type {
					continue
				}
				if p, ok := want["who:"+ro.Name]; ok && e.Pronouns != p {
					continue
				}
				free = append(free, id)
			}
			if len(free) == 0 {
				return Sample{}, fmt.Errorf("no %s filler fits %v", ro.Name, want)
			}
			fl.Entity = free[r.IntN(len(free))]
		}
		if n, ok := dets[ro.Name]; ok {
			if d, ok := want["ref:"+ro.Name]; ok {
				fl.Det = d
			} else {
				opts := detOptions(n, f)
				fl.Det = opts[r.IntN(len(opts))]
			}
		}
		used[fl.Entity] = true
		mm.Roles[ro.Name] = fl
	}
	if v, ok := want["verb"]; ok {
		p := strings.Fields(v)
		mm.Features = realize.Features{Tense: p[0], Aspect: "simple", Polarity: p[1], Mood: p[2]}
	} else {
		var verbs []string
		for _, d := range Dims(t, m, w) {
			if d.Name == "verb" {
				verbs = d.Values
			}
		}
		if len(verbs) == 0 {
			return Sample{}, fmt.Errorf("no verb forms")
		}
		p := strings.Fields(verbs[r.IntN(len(verbs))])
		mm.Features = realize.Features{Tense: p[0], Aspect: "simple", Polarity: p[1], Mood: p[2]}
	}
	if !w.Allowed(mm) {
		return Sample{}, fmt.Errorf("not allowed: %v", mm.Features)
	}
	implies, skip := w.Implications(t, mm)
	if skip {
		return Sample{}, fmt.Errorf("too odd: %v", mm.Features)
	}
	text, time, err := Generate(t, mm, w, lex, realize.Style{Contract: true}, r)
	if err != nil {
		return Sample{}, err
	}
	s := Sample{Text: text, Features: mm.Features, Fillers: map[string]string{}, Changed: []string{}, Time: time, Implies: implies}
	for role, fl := range mm.Roles {
		if fl.Entity != "" {
			s.Fillers[role] = w.Entities[fl.Entity].Display()
			if _, ok := dets[role]; ok {
				s.Fillers[role] = fl.Det + " " + s.Fillers[role]
			}
		}
	}
	return s, nil
}

// Aimed draws up to n samples that cover unjudged values first: each sample
// forces one value nobody has judged yet (08/W9).
func Aimed(t Template, m Meaning, w *World, lex *lexicon.Lexicon, seed uint64, n int, judged map[string]map[string]bool) []Sample {
	r := rand.New(rand.NewPCG(seed, 0x5eed))
	var wants []map[string]string
	for _, d := range Dims(t, m, w) {
		for _, v := range d.Values {
			if !judged[d.Name][v] {
				wants = append(wants, map[string]string{d.Name: v})
			}
		}
	}
	var out []Sample
	seen := map[string]bool{}
	for i := 0; len(out) < n && i < n*8; i++ {
		var want map[string]string
		if i < len(wants) {
			want = wants[i]
		}
		s, err := Draw(t, m, w, lex, r, want)
		if err != nil || seen[s.Text] {
			continue
		}
		seen[s.Text] = true
		out = append(out, s)
	}
	return out
}

// AuthoredCombo is the kind of sample the user's own sentence is.
func AuthoredCombo(t Template, m Meaning, w *World) map[string]string {
	f := m.Features
	c := map[string]string{"verb": f.Tense + " " + f.Polarity + " " + f.Mood}
	for role, n := range detSlots(t) {
		c["ref:"+role] = n.Det
	}
	for role, fl := range m.Roles {
		if fl.Entity != "" && w.Entities[fl.Entity].Name != "" {
			c["who:"+role] = w.Entities[fl.Entity].Pronouns
		}
	}
	return c
}
