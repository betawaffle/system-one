package web

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

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
// sources lists every sentence that has produced a template: a later sentence
// with the same shape becomes a new version of it ("Lee is shorter than Noor"
// after "Ana is older than Noor"), and verdicts on either count (08/W9f).
func (x *writer) sources(id string) []string {
	// one entry per sentence whatever its case, in its latest spelling
	seen := map[string]int{}
	var out []string
	for _, v := range x.g.History(id) {
		if v.Why == "retire" {
			continue
		}
		var t author.Template
		if b, err := x.g.Object(v.Hash); err == nil && json.Unmarshal(b, &t) == nil && t.Source.Text != "" {
			k := strings.ToLower(t.Source.Text)
			if i, ok := seen[k]; ok {
				out[i] = t.Source.Text
				continue
			}
			seen[k] = len(out)
			out = append(out, t.Source.Text)
		}
	}
	return out
}

func (x *writer) judged(lines []string) map[string]map[string]bool {
	in := map[string]bool{}
	for _, l := range lines {
		in[strings.ToLower(l)] = true // verdicts made before a sentence was capitalised still count
	}
	out := map[string]map[string]bool{}
	for _, v := range x.verdicts {
		if !in[strings.ToLower(v.Line)] {
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
	Sources []string       `json:"sources"` // every sentence that has produced this template
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
	srcs := x.sources(id)
	judged := x.judged(srcs)
	// the sentence the user wrote is right by definition, so its own kind counts
	for k, v := range author.AuthoredCombo(t, m, x.world) {
		if judged[k] == nil {
			judged[k] = map[string]bool{}
		}
		judged[k][v] = true
	}
	it := coverageItem{Sources: srcs, ID: id, Frame: x.world.Frames[t.Means.Frame].Title, Text: t.Source.Text, Meaning: m, Needs: t.Needs, Frozen: t.Frozen}
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

// GET /api/coverage/judged?id=ID: the samples of one template the user has
// judged, newest first, so a quick verdict can be looked at again (08/W9e).
func (s *Server) coverageJudged(w http.ResponseWriter, r *http.Request) {
	x, err := s.wr()
	if err != nil {
		http.Error(w, err.Error(), 503)
		return
	}
	x.mu.RLock()
	defer x.mu.RUnlock()
	id := r.URL.Query().Get("id")
	if _, ok := x.world.Templates[id]; !ok {
		http.Error(w, "unknown template", 404)
		return
	}
	srcs := x.sources(id)
	out := []sampleOut{}
	for _, v := range x.verdicts {
		if !inLines(v.Line, srcs) {
			continue
		}
		p := v
		out = append(out, sampleOut{Sample: v.Sample, Prior: &p, Combo: author.ComboOf(v.Sample, x.world)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Prior.At > out[j].Prior.At })
	jsonOut(w, map[string]any{"samples": out})
}

func inLines(l string, lines []string) bool {
	for _, x := range lines {
		if strings.EqualFold(x, l) {
			return true
		}
	}
	return false
}
