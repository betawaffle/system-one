package author

import (
	"fmt"
	"strings"
)

// Template content (09/Z5).
type Template struct {
	Kind     string   `json:"kind"`
	Means    Means    `json:"means"`
	Supports Supports `json:"supports"`
	Needs    []Need   `json:"needs"`
	Frozen   []string `json:"frozen"`
	Tree     Node     `json:"tree"`
	Source   Source   `json:"source"`
}

type Means struct {
	Frame string            `json:"frame"`
	Vars  map[string]string `json:"vars"` // role -> variable
	Adds  []Add             `json:"adds"`
}

// Add is meaning the template carries beyond the frame (03/A13d).
type Add struct {
	Kind string `json:"kind"` // modifier, event, attitude
	Key  string `json:"key"`  // occasion, purpose, send, emphatic…
	Text string `json:"text"` // the words that say it
}

type Supports struct {
	Tense    []string `json:"tense"`
	Polarity []string `json:"polarity"`
	Mood     []string `json:"mood"`
}

// Need is a feature combination that needs a modifier (08/D98).
type Need struct {
	Tense    string `json:"tense"`
	Modifier string `json:"modifier"`
}

type Source struct {
	Text  string `json:"text"`
	Event string `json:"event,omitempty"`
}

// Node is a tree node (04/N2, 09/Z5a).
type Node struct {
	T        string `json:"t"` // seq, word, lex, slot
	Label    string `json:"label,omitempty"`
	Text     string `json:"text,omitempty"`
	Lemma    string `json:"lemma,omitempty"`
	Kind     string `json:"kind,omitempty"` // slot kind: REF, DESC
	Var      string `json:"var,omitempty"`
	Form     string `json:"form,omitempty"`
	Case     string `json:"case,omitempty"`
	Converse bool   `json:"converse,omitempty"`
	Det      string `json:"det,omitempty"` // a REF noun's determiner as written
	Realises string `json:"realises,omitempty"`
	Kids     []Node `json:"kids,omitempty"`
}

// Resolution is the user's answer for one span (03/A13d).
type Resolution struct {
	Choice string `json:"choice"` // Modifier, New Modifier, Event, Attitude, Drop
	Detail string `json:"detail"`
}

// Traced reports whether every span has a resolution.
func (l Line) Traced(res map[string]Resolution) bool {
	for _, s := range l.Spans {
		if _, ok := res[s.Key]; !ok {
			return false
		}
	}
	return l.VG >= 0
}

// Build makes a template from an aligned line and its span resolutions (03/A13f).
func Build(l Line, m Meaning, w *World, res map[string]Resolution, needs []Need) (Template, error) {
	if !l.Traced(res) {
		return Template{}, fmt.Errorf("not fully traced")
	}
	f := w.Frames[m.Frame]
	t := Template{Kind: "S", Means: Means{Frame: m.Frame, Vars: map[string]string{}, Adds: []Add{}}, Needs: needs, Frozen: l.Waits, Source: Source{Text: l.Text}}
	if t.Needs == nil {
		t.Needs = []Need{}
	}
	used := map[string]bool{}
	for _, r := range f.Roles { // a short variable per role: giver -> g
		v := r.Name[:1]
		if used[v] {
			v = r.Name
		}
		used[v] = true
		t.Means.Vars[r.Name] = v
	}
	frozen := len(l.Waits) > 0

	// the verb group: auxiliaries right before the frame verb, and a "not" right after it
	vgStart, vgEnd := l.VG, l.VG
	for vgStart > 0 && l.Tokens[vgStart-1].Cls == "grammar" && isAux(l.Tokens[vgStart-1].Text) {
		vgStart--
	}
	if vgEnd+1 < len(l.Tokens) && strings.EqualFold(l.Tokens[vgEnd+1].Text, "not") {
		vgEnd++
	}
	simpleSubject := true
	for i := 0; i < vgStart; i++ {
		tk := l.Tokens[i]
		if !(tk.Cls == "role" && !tk.Prop) {
			simpleSubject = false
		}
	}

	var kids []Node
	slotOf := func(tk Token) Node {
		if tk.Prop {
			return Node{T: "slot", Kind: "DESC", Var: t.Means.Vars[tk.Role], Form: tk.Form, Converse: tk.Converse, Realises: tk.Role}
		}
		return Node{T: "slot", Kind: "REF", Var: t.Means.Vars[tk.Role], Form: tk.Form, Case: tk.Case, Det: tk.Det}
	}
	bang := false
	for i := 0; i < len(l.Tokens); i++ {
		tk := l.Tokens[i]
		switch {
		case tk.Span != "":
			sp := spanByKey(l, tk.Span)
			r := res[sp.Key]
			if r.Choice == "Drop" {
				i = sp.Last
				continue
			}
			add := Add{Kind: kindOf(r.Choice), Key: r.Detail, Text: sp.Text}
			if add.Key == "" {
				add.Key = "?"
			}
			t.Means.Adds = append(t.Means.Adds, add)
			if sp.Bang && sp.First == sp.Last {
				bang = true
				i = sp.Last
				continue
			}
			n := Node{T: "seq", Realises: "e." + add.Key}
			for j := sp.First; j <= sp.Last; j++ {
				if l.Tokens[j].Cls == "role" || l.Tokens[j].Poss {
					n.Kids = append(n.Kids, slotOf(l.Tokens[j]))
				} else if !l.Tokens[j].Bang {
					n.Kids = append(n.Kids, Node{T: "word", Text: l.Tokens[j].Text})
				}
			}
			kids = append(kids, n)
			i = sp.Last
		case i == vgStart && !frozen:
			v := l.Tokens[l.VG]
			n := Node{T: "lex", Label: "vg", Lemma: v.Lemma, Converse: v.Converse, Realises: "e"}
			if v.Lemma == "be" {
				n.Realises = ""
			}
			kids = append(kids, n)
			i = vgEnd
		case tk.Cls == "punct":
		case tk.Cls == "role" && tk.Part > 0:
		case tk.Cls == "role" && tk.Form == "det": // folded into the noun's slot
		case tk.Cls == "role":
			n := slotOf(tk)
			if i < vgStart && !tk.Prop {
				n.Label = "subj"
			}
			kids = append(kids, n)
		case tk.Cls == "frame": // frozen: the verb words stay as written
			kids = append(kids, Node{T: "word", Text: tk.Text, Label: "vg", Realises: "e"})
		default:
			kids = append(kids, Node{T: "word", Text: tk.Text})
		}
	}
	if bang {
		kids = append(kids, Node{T: "word", Text: "!", Realises: "e.attitude"})
	}
	t.Tree = Node{T: "seq", Kids: kids}

	if frozen {
		t.Supports = Supports{Tense: []string{l.Said.Tense}, Polarity: []string{l.Said.Polarity}, Mood: []string{l.Said.Mood}}
	} else {
		t.Supports = Supports{Tense: []string{"past", "present", "future"}, Polarity: []string{"positive", "negative"}, Mood: []string{"statement"}}
		if simpleSubject {
			t.Supports.Mood = append(t.Supports.Mood, "question")
		}
	}
	return t, nil
}

func isAux(w string) bool { b, _ := negBase(strings.ToLower(w)); return auxes[b] }

func spanByKey(l Line, key string) Span {
	for _, s := range l.Spans {
		if s.Key == key {
			return s
		}
	}
	return Span{}
}

func kindOf(choice string) string {
	switch choice {
	case "Event":
		return "event"
	case "Attitude":
		return "attitude"
	}
	return "modifier"
}

// Compact is the text form of a template shown under a saved sentence (04/S1).
func Compact(id string, t Template) string {
	var b strings.Builder
	var args []string
	for role, v := range t.Means.Vars {
		args = append(args, role+": "+v)
	}
	sortStrings(args)
	fmt.Fprintf(&b, "template %s\n  kind: %s\n  means: e = %s(%s)", id, t.Kind, strings.TrimPrefix(t.Means.Frame, "frame:"), strings.Join(args, ", "))
	for _, a := range t.Means.Adds {
		fmt.Fprintf(&b, ", e.%s = %q", a.Key, a.Text)
	}
	fmt.Fprintf(&b, "\n  supports: tense %s, polarity %s, mood %s", strings.Join(t.Supports.Tense, "|"), strings.Join(t.Supports.Polarity, "|"), strings.Join(t.Supports.Mood, "|"))
	if len(t.Frozen) > 0 {
		fmt.Fprintf(&b, "   (frozen: waits on %s)", strings.Join(t.Frozen, ", "))
	}
	for _, n := range t.Needs {
		fmt.Fprintf(&b, "\n  needs: %s when tense %s", n.Modifier, n.Tense)
	}
	b.WriteString("\n  tree:\n")
	var walk func(n Node, depth int)
	walk = func(n Node, depth int) {
		pad := strings.Repeat("  ", depth)
		label := ""
		if n.Label != "" {
			label = n.Label + ": "
		}
		re := ""
		if n.Realises != "" {
			re = "          realises " + n.Realises
		}
		switch n.T {
		case "seq":
			if depth > 2 || n.Realises != "" {
				fmt.Fprintf(&b, "%s%sseq%s\n", pad, label, re)
				depth++
			}
			for _, k := range n.Kids {
				walk(k, depth)
			}
		case "word":
			fmt.Fprintf(&b, "%s%sword %q%s\n", pad, label, n.Text, re)
		case "lex":
			conv := ""
			if n.Converse {
				conv = " (converse)"
			}
			fmt.Fprintf(&b, "%s%slex %s%s%s\n", pad, label, n.Lemma, conv, re)
		case "slot":
			form := strings.TrimSpace(n.Form + " " + n.Case)
			if form != "" {
				form = "  " + form
			}
			fmt.Fprintf(&b, "%s%s%s(%s)%s%s\n", pad, label, n.Kind, n.Var, form, re)
		}
	}
	walk(t.Tree, 2)
	return strings.TrimRight(b.String(), "\n")
}
