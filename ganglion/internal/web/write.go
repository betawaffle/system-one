package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"ganglion/internal/author"
	"ganglion/internal/grammar"
	"ganglion/internal/lexicon"
	"ganglion/internal/realize"
)

// The writing screen's API (design/synthetic-data/08). Alignment, templates
// and samples are plain code (03/D73); model features aren't connected yet.

type writer struct {
	mu    sync.RWMutex
	g     *grammar.Store
	lex   *lexicon.Lexicon
	world *author.World
}

func (s *Server) wr() (*writer, error) {
	if s.writer == nil {
		return nil, errors.New("the grammar isn't loaded: run `ganglion lexicon import` and restart")
	}
	return s.writer, nil
}

func newWriter(g *grammar.Store, lex *lexicon.Lexicon) (*writer, error) {
	if g == nil || lex == nil {
		return nil, nil
	}
	w, err := author.Load(g)
	if err != nil {
		return nil, err
	}
	return &writer{g: g, lex: lex, world: w}, nil
}

func (x *writer) reload() error {
	w, err := author.Load(x.g)
	if err != nil {
		return err
	}
	x.mu.Lock()
	x.world = w
	x.mu.Unlock()
	return nil
}

func jsonOut(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonIn(w http.ResponseWriter, r *http.Request, v any) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		http.Error(w, "same-origin JSON only", http.StatusForbidden)
		return false
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		http.Error(w, err.Error(), 400)
		return false
	}
	return true
}

type roleCard struct {
	Name, Explainer, Generic, Fill string
	Series                         int
}

type card struct {
	Meaning   author.Meaning `json:"meaning"`
	Title     string         `json:"title"`
	Explainer string         `json:"explainer"`
	Roles     []roleCard     `json:"roles"`
	Frames    [][2]string    `json:"frames"` // id, title
}

func (x *writer) card(m author.Meaning) card {
	w := x.world
	f := w.Frames[m.Frame]
	c := card{Meaning: m, Title: f.Title, Explainer: f.Explainer}
	for i, r := range f.Roles {
		fl := m.Roles[r.Name]
		fill := ""
		switch {
		case fl.Value != nil && fl.Value.Value != "":
			fill = fl.Value.Attribute + " = " + fl.Value.Value
		case fl.Value != nil:
			fill = fl.Value.Attribute + ", " + fl.Value.Dir
		case fl.Entity != "":
			e := w.Entities[fl.Entity]
			fill = e.Display() + " · " + e.Type
			if e.Name != "" {
				p, _ := realize.Pronoun(e.Pronouns, "subj")
				o, _ := realize.Pronoun(e.Pronouns, "obj")
				fill += " · " + p + "/" + o
			}
			if e.Coerced != "" {
				fill += ", treated as " + e.Coerced
			}
		}
		c.Roles = append(c.Roles, roleCard{Name: r.Name, Explainer: r.Explainer, Generic: r.Generic, Fill: fill, Series: i + 1})
	}
	for id, f := range w.Frames {
		c.Frames = append(c.Frames, [2]string{id, f.Title})
	}
	sort.Slice(c.Frames, func(i, j int) bool { return c.Frames[i][1] < c.Frames[j][1] })
	return c
}

// GET /api/write/next?n=N[&frame=ID&tense=…&polarity=…&mood=…]
func (s *Server) writeNext(w http.ResponseWriter, r *http.Request) {
	x, err := s.wr()
	if err != nil {
		http.Error(w, err.Error(), 503)
		return
	}
	x.mu.RLock()
	defer x.mu.RUnlock()
	n, _ := strconv.Atoi(r.URL.Query().Get("n"))
	m, err := author.Next(x.world, n, r.URL.Query().Get("frame"))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	q := r.URL.Query()
	for k, p := range map[string]*string{"tense": &m.Features.Tense, "polarity": &m.Features.Polarity, "mood": &m.Features.Mood} {
		if v := q.Get(k); v != "" {
			*p = v
		}
	}
	jsonOut(w, x.card(m))
}

// POST /api/write/fillers: the same meaning with different fillers (08/W2a)
func (s *Server) writeFillers(w http.ResponseWriter, r *http.Request) {
	x, err := s.wr()
	if err != nil {
		http.Error(w, err.Error(), 503)
		return
	}
	var req struct {
		Meaning author.Meaning `json:"meaning"`
		Seed    uint64         `json:"seed"`
	}
	if !jsonIn(w, r, &req) {
		return
	}
	x.mu.RLock()
	defer x.mu.RUnlock()
	m := req.Meaning
	used := map[string]bool{}
	f := x.world.Frames[m.Frame]
	roles := map[string]author.Filler{}
	for _, ro := range f.Roles {
		fl := m.Roles[ro.Name]
		if fl.Entity != "" && x.world.Entities[fl.Entity].Coerced == "" {
			cur := x.world.Entities[fl.Entity]
			var pool []string
			for _, id := range x.world.EntitiesOf(ro.Type) {
				e := x.world.Entities[id]
				if id != fl.Entity && !used[id] && e.Type == cur.Type && (e.Name == "") == (cur.Name == "") {
					pool = append(pool, id)
				}
			}
			if len(pool) > 0 {
				fl.Entity = pool[int(req.Seed%uint64(len(pool)))]
			}
		}
		used[fl.Entity] = true
		roles[ro.Name] = fl
	}
	m.Roles = roles
	jsonOut(w, x.card(m))
}

type lineReq struct {
	Text  string                       `json:"text"`
	Res   map[string]author.Resolution `json:"res"`
	Needs []author.Need                `json:"needs"`
	How   string                       `json:"how"`
}

type lineOut struct {
	author.Line
	Traced  bool            `json:"traced"`
	Compact string          `json:"compact,omitempty"`
	Samples []author.Sample `json:"samples"`
	Error   string          `json:"error,omitempty"`
}

// POST /api/write/preview: align each line, and for traced lines build the
// template and draw samples (08/W3a, W6). No model, no saving.
func (s *Server) writePreview(w http.ResponseWriter, r *http.Request) {
	x, err := s.wr()
	if err != nil {
		http.Error(w, err.Error(), 503)
		return
	}
	var req struct {
		Meaning author.Meaning `json:"meaning"`
		Lines   []lineReq      `json:"lines"`
		Seed    uint64         `json:"seed"`
	}
	if !jsonIn(w, r, &req) {
		return
	}
	x.mu.RLock()
	defer x.mu.RUnlock()
	if _, ok := x.world.Frames[req.Meaning.Frame]; !ok {
		http.Error(w, "unknown frame", 400)
		return
	}
	out := []lineOut{}
	for i, lr := range req.Lines {
		l := author.Align(lr.Text, req.Meaning, x.world, x.lex)
		lo := lineOut{Line: l, Traced: l.Traced(lr.Res), Samples: []author.Sample{}}
		if lo.Traced {
			t, err := author.Build(l, req.Meaning, x.world, lr.Res, lr.Needs)
			if err != nil {
				lo.Error = err.Error()
			} else {
				lo.Compact = author.Compact("(unsaved)", t)
				lo.Samples = author.Samples(t, req.Meaning, x.world, x.lex, req.Seed+uint64(i)*7919, 8)
			}
		}
		out = append(out, lo)
	}
	jsonOut(w, map[string]any{"lines": out})
}

// POST /api/write/save: log each sentence, and save traced ones as templates
// (07/D86-D89). Drafts are logged only.
func (s *Server) writeSave(w http.ResponseWriter, r *http.Request) {
	x, err := s.wr()
	if err != nil {
		http.Error(w, err.Error(), 503)
		return
	}
	var req struct {
		Meaning author.Meaning `json:"meaning"`
		Lines   []lineReq      `json:"lines"`
		Draft   bool           `json:"draft"`
	}
	if !jsonIn(w, r, &req) {
		return
	}
	type saved struct {
		ID, Text, Compact string
		Draft             bool
	}
	var out []saved
	x.mu.RLock()
	world := x.world
	x.mu.RUnlock()
	for _, lr := range req.Lines {
		if strings.TrimSpace(lr.Text) == "" {
			continue
		}
		l := author.Align(lr.Text, req.Meaning, world, x.lex)
		traced := l.Traced(lr.Res)
		if traced == req.Draft {
			continue // Save takes traced lines, Save as Draft takes the rest
		}
		how := lr.How
		if how != "pasted" {
			how = "typed"
		}
		ev, err := x.g.AddEvent("sentence", s.cfg.User, "ganglion/write", map[string]any{
			"text": lr.Text, "how": how, "meaning": req.Meaning, "resolutions": lr.Res, "needs": lr.Needs, "draft": req.Draft, "warnings": l.Warnings})
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if req.Draft {
			out = append(out, saved{Text: lr.Text, Draft: true})
			continue
		}
		t, err := author.Build(l, req.Meaning, world, lr.Res, lr.Needs)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		t.Source.Event = ev
		_, hash, _ := grammar.Canonical(t.Tree)
		id := "template:" + strings.TrimPrefix(req.Meaning.Frame, "frame:") + "-" + hash[:8]
		if _, err := x.g.Put(id, "template", t, grammar.Meta{By: s.cfg.User, Client: "ganglion/write", Event: ev}); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		out = append(out, saved{ID: id, Text: lr.Text, Compact: author.Compact(id, t)})
	}
	if err := x.reload(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	jsonOut(w, map[string]any{"saved": out})
}

// POST /api/write/verdict: a sample judged right or wrong, with a reason (08/W6a).
func (s *Server) writeVerdict(w http.ResponseWriter, r *http.Request) {
	x, err := s.wr()
	if err != nil {
		http.Error(w, err.Error(), 503)
		return
	}
	var req map[string]any
	if !jsonIn(w, r, &req) {
		return
	}
	id, err := x.g.AddEvent("verdict", s.cfg.User, "ganglion/write", req)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	jsonOut(w, map[string]any{"event": id})
}

// GET /api/write/saved: templates, newest first.
func (s *Server) writeSaved(w http.ResponseWriter, r *http.Request) {
	x, err := s.wr()
	if err != nil {
		http.Error(w, err.Error(), 503)
		return
	}
	type item struct {
		ID, Frame, Text, Compact, At string
		Frozen                       []string
	}
	var out []item
	x.mu.RLock()
	for _, v := range x.g.Heads("template") {
		t := x.world.Templates[v.ID]
		out = append(out, item{ID: v.ID, Frame: x.world.Frames[t.Means.Frame].Title, Text: t.Source.Text, Compact: author.Compact(v.ID, t), At: v.At, Frozen: t.Frozen})
	}
	x.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].At > out[j].At })
	jsonOut(w, map[string]any{"templates": out})
}

// GET /api/lexicon?word=W: what the lexicon says about a word (05/D74's "hey wait a minute").
func (s *Server) lexiconWord(w http.ResponseWriter, r *http.Request) {
	x, err := s.wr()
	if err != nil {
		http.Error(w, err.Error(), 503)
		return
	}
	word := strings.ToLower(r.URL.Query().Get("word"))
	var out []*lexicon.Entry
	seen := map[string]bool{}
	for _, ref := range x.lex.Lookup(word) {
		k := ref.POS + "|" + ref.Lemma
		if !seen[k] {
			seen[k] = true
			out = append(out, x.lex.Get(ref.POS, ref.Lemma))
		}
	}
	jsonOut(w, map[string]any{"word": word, "entries": out, "base": x.lex.Hash})
}
