package author

import (
	"regexp"
	"sort"
	"strings"

	"ganglion/internal/lexicon"
	"ganglion/internal/realize"
)

// Token is one word of a written sentence and what it traces to (03/A13).
type Token struct {
	Text     string `json:"text"`
	At       int    `json:"at"`
	Cls      string `json:"cls"` // role, frame, grammar, untraced, punct
	Role     string `json:"role,omitempty"`
	Form     string `json:"form,omitempty"` // name, possname, noun, plural, pron; for properties "", er, est
	Case     string `json:"case,omitempty"` // pronoun case
	Part     int    `json:"part,omitempty"` // word index inside a multiword noun
	Prop     bool   `json:"prop,omitempty"`
	Lemma    string `json:"lemma,omitempty"`
	Slot     string `json:"slot,omitempty"`
	Neg      bool   `json:"neg,omitempty"`
	Converse bool   `json:"converse,omitempty"`
	Modal    bool   `json:"modal,omitempty"`
	Bang     bool   `json:"bang,omitempty"`
	Span     string `json:"span,omitempty"`
}

// Span is a run of untraced words to resolve (03/A13c).
type Span struct {
	Key   string `json:"key"`
	Text  string `json:"text"`
	First int    `json:"first"` // token indexes
	Last  int    `json:"last"`
	Modal bool   `json:"modal,omitempty"`
	Bang  bool   `json:"bang,omitempty"`
}

// Line is an aligned sentence.
type Line struct {
	Text     string   `json:"text"`
	Tokens   []Token  `json:"tokens"`
	Spans    []Span   `json:"spans"`
	Waits    []string `json:"waits"`    // unsupported verb shapes: the template saves frozen (05/D80)
	Warnings []string `json:"warnings"` // the sentence's features disagree with the meaning
	VG       int      `json:"vg"`       // index of the frame verb, -1 if none
	Said     realize.Features `json:"said"`
}

var tokenRE = regexp.MustCompile(`[A-Za-z]+(?:'[A-Za-z]+)?|[0-9]+|[^\sA-Za-z0-9]`)
var wordRE = regexp.MustCompile(`[a-z0-9]`)

// sortedRoles gives a meaning's roles in a fixed order, so matching is deterministic.
func sortedRoles(m Meaning) []string {
	var out []string
	for r := range m.Roles {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

var grammarWords = set("a", "an", "the", "to", "than", "as", "of", "this", "that", "these", "those", "not", "very", "from", "by",
	"did", "does", "do", "will", "has", "have", "had", "am", "are", "were", "is", "was", "be", "been", "being")
var modals = set("can", "could", "may", "might", "must", "shall", "should", "would")
var auxes = set("did", "does", "do", "will", "has", "have", "had", "not", "am", "is", "are", "was", "were", "be", "been")

func set(ws ...string) map[string]bool {
	m := map[string]bool{}
	for _, w := range ws {
		m[w] = true
	}
	return m
}

// negBase splits a contracted negative: "didn't" -> "did", "won't" -> "will".
func negBase(w string) (string, bool) {
	switch w {
	case "won't":
		return "will", true
	case "can't":
		return "can", true
	}
	if strings.HasSuffix(w, "n't") {
		return strings.TrimSuffix(w, "n't"), true
	}
	return w, false
}

// Align traces a sentence's words to a meaning (03/A13a-c). It needs no model.
func Align(text string, m Meaning, w *World, lex *lexicon.Lexicon) Line {
	f := w.Frames[m.Frame]
	var toks []Token
	for _, loc := range tokenRE.FindAllStringIndex(text, -1) {
		toks = append(toks, Token{Text: text[loc[0]:loc[1]], At: loc[0]})
	}
	lw := make([]string, len(toks))
	for i, t := range toks {
		lw[i] = strings.ToLower(t.Text)
	}
	roleType := map[string]string{}
	for _, r := range f.Roles {
		roleType[r.Name] = r.Type
	}
	holder := holderType(m, w)

	for i := range toks {
		t := &toks[i]
		if t.Cls != "" {
			continue
		}
		word := lw[i]
		base, neg := negBase(word)
		if word == "!" {
			t.Cls, t.Bang = "untraced", true // "!" is extra meaning (08/D99)
			continue
		}
		if !wordRE.MatchString(word) {
			t.Cls = "punct"
			continue
		}
		if matchEntity(toks, lw, i, m, w, lex) {
			continue
		}
		if matchProperty(t, word, m, w, lex, holder) {
			continue
		}
		if ref := frameVerb(base, f, lex); ref != nil {
			t.Cls, t.Lemma, t.Slot, t.Neg = "frame", ref.Lemma, ref.Slot, neg
			for _, c := range f.Converse {
				t.Converse = t.Converse || c == ref.Lemma
			}
			continue
		}
		if modals[base] {
			t.Cls, t.Modal, t.Neg = "untraced", true, neg
			continue
		}
		if grammarWords[word] || grammarWords[base] {
			t.Cls, t.Neg = "grammar", neg
			continue
		}
		t.Cls = "untraced"
	}
	line := Line{Text: text, Tokens: toks, VG: -1}
	for i, t := range toks {
		if t.Cls == "frame" {
			line.VG = i
			break
		}
	}
	line.Spans = findSpans(toks)
	for _, s := range line.Spans {
		for j := s.First; j <= s.Last; j++ {
			toks[j].Span = s.Key
		}
	}
	line.Waits = waitsOn(toks, lw)
	line.Said = said(toks, lw, line.VG, m.Features)
	line.Warnings = warnings(line.Said, m.Features, line.VG >= 0)
	if line.Spans == nil {
		line.Spans = []Span{}
	}
	if line.Waits == nil {
		line.Waits = []string{}
	}
	if line.Warnings == nil {
		line.Warnings = []string{}
	}
	return line
}

func holderType(m Meaning, w *World) string {
	for _, r := range []string{"holder", "a"} {
		if f, ok := m.Roles[r]; ok && f.Entity != "" {
			return w.Entities[f.Entity].Type
		}
	}
	return ""
}

func matchEntity(toks []Token, lw []string, i int, m Meaning, w *World, lex *lexicon.Lexicon) bool {
	word := lw[i]
	for _, role := range sortedRoles(m) {
		fl := m.Roles[role]
		if fl.Entity == "" {
			continue
		}
		e := w.Entities[fl.Entity]
		if e.Noun != "" && strings.Contains(e.Noun, " ") {
			parts := strings.Fields(e.Noun)
			ok := i+len(parts) <= len(lw)
			for j := 0; ok && j < len(parts); j++ {
				ok = lw[i+j] == parts[j]
			}
			if ok {
				for j := range parts {
					toks[i+j].Cls, toks[i+j].Role, toks[i+j].Form, toks[i+j].Part = "role", role, "noun", j
				}
				return true
			}
		}
		t := &toks[i]
		switch {
		case e.Name != "" && word == strings.ToLower(e.Name):
			t.Cls, t.Role, t.Form = "role", role, "name"
		case e.Name != "" && word == strings.ToLower(e.Name)+"'s":
			t.Cls, t.Role, t.Form = "role", role, "possname"
		case e.Noun != "" && word == e.Noun:
			t.Cls, t.Role, t.Form = "role", role, "noun"
		case e.Noun != "" && word == plural(lex, e.Noun):
			t.Cls, t.Role, t.Form = "role", role, "plural"
		default:
			if s, c, ok := realize.PronounCase(word); ok && s == e.Pronouns {
				t.Cls, t.Role, t.Form, t.Case = "role", role, "pron", c
			}
		}
		if t.Cls != "" {
			return true
		}
	}
	return false
}

func matchProperty(t *Token, word string, m Meaning, w *World, lex *lexicon.Lexicon, holder string) bool {
	for _, role := range sortedRoles(m) {
		v := m.Roles[role].Value
		if v == nil {
			continue
		}
		a := w.Attributes["attribute:"+v.Attribute]
		var cands []string
		if v.Value != "" {
			cands = []string{a.Word(v.Value, holder, w)}
		} else {
			lo, hi := a.Ends(holder, w)
			cands = []string{hi, lo}
		}
		for ci, adj := range cands {
			if adj == "" {
				continue
			}
			forms := map[string]string{adj: ""}
			if e := lex.Get("adj", adj); e != nil {
				for slot, f := range e.Forms {
					forms[f] = slot
				}
			}
			if slot, ok := forms[word]; ok {
				t.Cls, t.Role, t.Prop, t.Form = "role", role, true, slot
				// for a comparison, the low-end adjective says it from b's side ("shorter")
				t.Converse = v.Value == "" && ((v.Dir != "less") == (ci == 1))
				return true
			}
		}
	}
	return false
}

func frameVerb(word string, f Frame, lex *lexicon.Lexicon) *lexicon.FormRef {
	for _, ref := range lex.Lookup(word) {
		if ref.POS != "verb" {
			continue
		}
		for _, v := range append(append([]string{}, f.Verbs...), f.Converse...) {
			if ref.Lemma == v {
				r := ref
				return &r
			}
		}
	}
	return nil
}

func findSpans(toks []Token) []Span {
	var spans []Span
	var run []int
	flush := func() {
		for len(run) > 0 && toks[run[len(run)-1]].Cls != "untraced" {
			run = run[:len(run)-1]
		}
		for len(run) > 0 && (toks[run[0]].Text == "a" || toks[run[0]].Text == "an" || strings.EqualFold(toks[run[0]].Text, "the")) {
			run = run[1:]
		}
		has := false
		for _, i := range run {
			has = has || toks[i].Cls == "untraced"
		}
		if has {
			var words []string
			s := Span{First: run[0], Last: run[len(run)-1]}
			for _, i := range run {
				words = append(words, toks[i].Text)
				s.Modal = s.Modal || toks[i].Modal
				s.Bang = s.Bang || toks[i].Bang
			}
			s.Text = joinWords(words)
			s.Key = strings.ToLower(s.Text)
			spans = append(spans, s)
		}
		run = nil
	}
	leadsIn := func(i int) bool {
		j := i + 1
		for j < len(toks) && toks[j].Cls == "grammar" {
			j++
		}
		return j < len(toks) && toks[j].Cls == "untraced"
	}
	for i, t := range toks {
		next := i + 1 < len(toks)
		possInside := t.Cls == "role" && (t.Case == "det" || t.Form == "possname") && len(run) > 0 && next && toks[i+1].Cls == "untraced"
		if t.Cls == "untraced" || (t.Cls == "grammar" && (len(run) > 0 || leadsIn(i))) || possInside {
			run = append(run, i)
		} else {
			flush()
		}
	}
	flush()
	return spans
}

func joinWords(ws []string) string {
	s := strings.Join(ws, " ")
	for _, p := range []string{",", ".", "!", "?", ";", ":"} {
		s = strings.ReplaceAll(s, " "+p, p)
	}
	return s
}

var tagRE = regexp.MustCompile(`(?i),\s*(isn't|aren't|wasn't|didn't|doesn't|won't|is|was|did|does)\s+(she|he|they|it|i|you)\s*\?\s*$`)

func waitsOn(toks []Token, lw []string) []string {
	var out []string
	for _, t := range toks {
		if t.Modal {
			out = append(out, "modals")
			break
		}
	}
	for i, t := range toks {
		if t.Cls == "frame" && t.Lemma != "be" && t.Slot == "pastp" && i > 0 {
			if b, _ := negBase(lw[i-1]); b == "is" || b == "was" || b == "are" || b == "were" || b == "be" || b == "been" {
				out = append(out, "passive")
				break
			}
		}
	}
	var words []string
	for _, t := range toks {
		words = append(words, t.Text)
	}
	if tagRE.MatchString(joinWords(words)) {
		out = append(out, "tag questions")
	}
	return out
}

// said reads the features the sentence itself expresses, from its verb group.
func said(toks []Token, lw []string, vg int, def realize.Features) realize.Features {
	f := realize.Features{Tense: "present", Aspect: "simple", Polarity: "positive", Mood: "statement"}
	if vg < 0 {
		return def
	}
	start := vg
	for start > 0 && toks[start-1].Cls == "grammar" {
		b, _ := negBase(lw[start-1])
		if !auxes[b] {
			break
		}
		start--
	}
	for i := start; i <= vg+1 && i < len(toks); i++ {
		b, neg := negBase(lw[i])
		if neg || b == "not" {
			f.Polarity = "negative"
		}
		switch b {
		case "will":
			f.Tense = "future"
		case "did", "was", "were", "had":
			f.Tense = "past"
		}
	}
	switch toks[vg].Slot {
	case "past":
		f.Tense = "past"
	case "ing":
		f.Aspect = "progressive"
	case "pastp":
		f.Aspect = "perfect"
	}
	if toks[vg].Lemma == "be" && toks[vg].Slot == "" && f.Tense != "future" {
		if lw[vg] == "was" || lw[vg] == "were" {
			f.Tense = "past"
		}
	}
	if len(toks) > 0 && toks[len(toks)-1].Text == "?" {
		f.Mood = "question"
	}
	return f
}

func warnings(s, m realize.Features, hasVerb bool) []string {
	if !hasVerb {
		return []string{"No verb for this frame found yet."}
	}
	var out []string
	if s.Tense != m.Tense {
		out = append(out, "Says "+s.Tense+"; the meaning is "+m.Tense+".")
	}
	if s.Polarity != m.Polarity {
		out = append(out, "Says "+s.Polarity+"; the meaning is "+m.Polarity+".")
	}
	if s.Mood != m.Mood {
		out = append(out, "Is a "+s.Mood+"; the meaning is a "+m.Mood+".")
	}
	return out
}
