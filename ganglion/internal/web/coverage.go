package web

import (
	"encoding/json"
	"net/http"
	"sort"

	"ganglion/internal/author"
	"ganglion/internal/grammar"
)

// The coverage view (design/synthetic-data/08/W9): for each saved template,
// which kinds of sample the user has judged, and samples aimed at the rest.

func (x *writer) noteMeaning(id string, d map[string]any) {
	b, _ := json.Marshal(d["meaning"])
	var m author.Meaning
	if json.Unmarshal(b, &m) == nil && m.Frame != "" {
		x.meanings[id] = m
	}
}

// judged lists dimension values with a verdict, for samples of one sentence.
func (x *writer) judged(line string) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, v := range x.verdicts {
		if v.Line != line {
			continue
		}
		for k, val := range author.ComboOf(v.Sample, x.world) {
			if out[k] == nil {
				out[k] = map[string]bool{}
			}
			out[k][val] = true
		}
	}
	return out
}

type dimOut struct {
	author.Dim
	Judged []bool `json:"judged"`
}

type coverageItem struct {
	ID      string         `json:"id"`
	Frame   string         `json:"frame"`
	Text    string         `json:"text"`
	Meaning author.Meaning `json:"meaning"`
	Dims    []dimOut       `json:"dims"`
	Done    int            `json:"done"`
	Total   int            `json:"total"`
	Needs   []author.Need  `json:"needs"`
	Frozen  []string       `json:"frozen"`
}

func (x *writer) coverageOf(id string) (coverageItem, author.Template, bool) {
	t, ok := x.world.Templates[id]
	if !ok {
		return coverageItem{}, t, false
	}
	m, ok := x.meanings[t.Source.Event]
	if !ok {
		return coverageItem{}, t, false
	}
	judged := x.judged(t.Source.Text)
	// the sentence the user wrote is right by definition, so its own kind counts
	for k, v := range author.AuthoredCombo(t, m, x.world) {
		if judged[k] == nil {
			judged[k] = map[string]bool{}
		}
		judged[k][v] = true
	}
	it := coverageItem{ID: id, Frame: x.world.Frames[t.Means.Frame].Title, Text: t.Source.Text, Meaning: m, Needs: t.Needs, Frozen: t.Frozen}
	for _, d := range author.Dims(t, m, x.world) {
		do := dimOut{Dim: d}
		for _, v := range d.Values {
			j := judged[d.Name][v]
			do.Judged = append(do.Judged, j)
			it.Total++
			if j {
				it.Done++
			}
		}
		it.Dims = append(it.Dims, do)
	}
	return it, t, true
}

// GET /api/coverage
func (s *Server) coverage(w http.ResponseWriter, r *http.Request) {
	x, err := s.wr()
	if err != nil {
		http.Error(w, err.Error(), 503)
		return
	}
	x.mu.RLock()
	defer x.mu.RUnlock()
	out := []coverageItem{}
	for id := range x.world.Templates {
		if it, _, ok := x.coverageOf(id); ok {
			out = append(out, it)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Frame != out[j].Frame {
			return out[i].Frame < out[j].Frame
		}
		return out[i].Text < out[j].Text
	})
	jsonOut(w, map[string]any{"templates": out})
}

// POST /api/coverage/show {id, seed}: samples aimed at unjudged values first.
func (s *Server) coverageShow(w http.ResponseWriter, r *http.Request) {
	x, err := s.wr()
	if err != nil {
		http.Error(w, err.Error(), 503)
		return
	}
	var req struct {
		ID   string `json:"id"`
		Seed uint64 `json:"seed"`
	}
	if !jsonIn(w, r, &req) {
		return
	}
	x.mu.RLock()
	defer x.mu.RUnlock()
	it, t, ok := x.coverageOf(req.ID)
	if !ok {
		http.Error(w, "unknown template", 404)
		return
	}
	samples := []sampleOut{}
	judged := map[string]map[string]bool{}
	for _, d := range it.Dims {
		judged[d.Name] = map[string]bool{}
		for i, v := range d.Values {
			judged[d.Name][v] = d.Judged[i]
		}
	}
	for _, smp := range author.Aimed(t, it.Meaning, x.world, x.lex, req.Seed, 8, judged) {
		so := sampleOut{Sample: smp, Prior: x.prior(smp)}
		so.Combo = author.ComboOf(smp, x.world)
		samples = append(samples, so)
	}
	jsonOut(w, map[string]any{"template": it, "samples": samples})
}

// POST /api/template/need {id, tense}: Sounds Odd on a saved template makes a
// new version that adds a time modifier for that tense (08/D98).
func (s *Server) templateNeed(w http.ResponseWriter, r *http.Request) {
	x, err := s.wr()
	if err != nil {
		http.Error(w, err.Error(), 503)
		return
	}
	var req struct {
		ID    string `json:"id"`
		Tense string `json:"tense"`
	}
	if !jsonIn(w, r, &req) {
		return
	}
	x.mu.RLock()
	t, ok := x.world.Templates[req.ID]
	x.mu.RUnlock()
	if !ok {
		http.Error(w, "unknown template", 404)
		return
	}
	for _, n := range t.Needs {
		if n.Tense == req.Tense {
			jsonOut(w, map[string]any{"ok": true})
			return
		}
	}
	t.Needs = append(append([]author.Need{}, t.Needs...), author.Need{Tense: req.Tense, Modifier: "time"})
	ev, err := x.g.AddEvent("need", s.cfg.User, "ganglion/coverage", map[string]any{"id": req.ID, "tense": req.Tense, "modifier": "time", "why": "Sounds Odd"})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if _, err := x.g.Put(req.ID, "template", t, grammar.Meta{By: s.cfg.User, Why: "fix", Client: "ganglion/coverage", Event: ev}); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := x.reload(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	jsonOut(w, map[string]any{"ok": true})
}

func (s *Server) coveragePage(w http.ResponseWriter, r *http.Request) {
	_, err := s.wr()
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	s.render(w, "coverage", Page{Title: "Coverage", Nav: "coverage", Data: map[string]any{"Error": msg}})
}
