package web

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"ganglion/internal/author"
	"ganglion/internal/jev"
)

// Jev as a baseline (design/synthetic-data/10/J14): four checks on every
// sample you've marked ✓ or ◐ (do they stay quiet?) and on samples broken in
// code (do they catch it, with the right check? 10/J11).

// jevItem is one sample to send, with what we expect.
type jevItem struct {
	Text    string         `json:"text"`
	Expect  string         `json:"expect"` // "" for a good sample, else the check that should fire
	Yours   string         `json:"yours"`  // your verdict, for good ones
	Broken  string         `json:"broken"` // how it was broken, for J11 ones
	Meaning author.Meaning `json:"-"`
	Line    string         `json:"-"`
	Sample  author.Sample  `json:"-"`
}

// jevSet is your ✓ and ◐ samples, then the broken ones.
func (x *writer) jevSet() []jevItem {
	var texts []string
	for t, v := range x.verdicts {
		if (v.Verdict == "ok" || v.Verdict == "but") && v.Meaning.Frame != "" {
			texts = append(texts, t)
		}
	}
	sort.Strings(texts)
	var out []jevItem
	for _, t := range texts {
		v := x.verdicts[t]
		out = append(out, jevItem{Text: t, Yours: describe(v), Meaning: v.Meaning, Line: v.Line, Sample: v.Sample})
	}
	return append(out, x.brokenSet(texts)...)
}

// brokenSet breaks your ✓ samples in known ways (10/J11). Each is labelled
// by construction with the check that should catch it.
func (x *writer) brokenSet(texts []string) []jevItem {
	find := func(frame, tense, pol, mood string) (priorVerdict, bool) {
		for _, t := range texts {
			v := x.verdicts[t]
			f := v.Sample.Features
			if v.Verdict == "ok" && v.Meaning.Frame == frame && f.Tense == tense && f.Polarity == pol && f.Mood == mood && v.Sample.Time == "" {
				return v, true
			}
		}
		return priorVerdict{}, false
	}
	var out []jevItem
	add := func(v priorVerdict, text, expect, how string, edit func(*author.Sample)) {
		if text == "" || text == v.Sample.Text {
			return
		}
		s := v.Sample
		s.Fillers = map[string]string{}
		for k, f := range v.Sample.Fillers {
			s.Fillers[k] = f
		}
		if edit != nil {
			edit(&s)
		}
		s.Text = text
		out = append(out, jevItem{Text: text, Expect: expect, Broken: how, Meaning: v.Meaning, Line: v.Line, Sample: s})
	}
	cap1 := func(s string) string { return strings.ToUpper(s[:1]) + s[1:] }
	if v, ok := find("frame:give", "past", "positive", "statement"); ok {
		t, g, r := v.Sample.Text, v.Sample.Fillers["giver"], v.Sample.Fillers["recipient"]
		add(v, strings.Replace(t, " gave ", " gived ", 1), "grammar", "wrong inflection: gived", nil)
		if g != "" && r != "" && strings.HasPrefix(t, cap1(g)+" gave "+r+" ") {
			rest := strings.TrimPrefix(t, cap1(g)+" gave "+r+" ")
			add(v, cap1(r)+" gave "+g+" "+rest, "meaning", "giver and recipient swapped", nil)
			add(v, strings.Replace(t, " gave "+r+" ", " gave ", 1), "meaning", "recipient dropped", nil)
			add(v, "The idea gave "+r+" "+rest, "filler", "an idea as the giver", func(s *author.Sample) { s.Fillers["giver"] = "the idea" })
		}
		add(v, strings.Replace(t, " gave ", " gives ", 1), "meaning", "present tense for a past meaning", nil)
	}
	if v, ok := find("frame:compare", "present", "positive", "statement"); ok {
		t, a := v.Sample.Text, v.Sample.Fillers["a"]
		// not "Ana are": a band named Ana takes "are" in British English (user)
		if at, ok := v.Meaning.Roles["attribute"]; ok && at.Value != nil && at.Value.Attribute == "age" {
			add(v, strings.Replace(t, " is older ", " is more older ", 1), "grammar", "double comparative: more older", nil)
		}
		add(v, strings.Replace(t, " is ", " ", 1), "grammar", "verb dropped", nil)
		if a != "" && strings.HasPrefix(t, cap1(a)+" is ") {
			add(v, "The idea is "+strings.TrimPrefix(t, cap1(a)+" is "), "filler", "an idea in a comparison", func(s *author.Sample) { s.Fillers["a"] = "the idea" })
		}
	}
	if v, ok := find("frame:has-property", "present", "positive", "statement"); ok {
		add(v, strings.Replace(v.Sample.Text, " is ", " isn't ", 1), "meaning", "negated text for a positive meaning", nil)
	}
	return out
}

// jevState is the minimal state: the sentence, and its meaning as flat
// fields. Fillers are the sample's own, which vary from the authored meaning's.
func (x *writer) jevState(m author.Meaning, s author.Sample) jev.State {
	f := x.world.Frames[m.Frame]
	mm := map[string]string{"frame": strings.TrimPrefix(m.Frame, "frame:"), "tense": s.Features.Tense, "polarity": s.Features.Polarity, "mood": s.Features.Mood}
	if s.Time != "" {
		mm["time"] = s.Time
	}
	for _, ro := range f.Roles {
		fl := m.Roles[ro.Name]
		switch {
		case fl.Value != nil && fl.Value.Value != "":
			mm[ro.Name] = fl.Value.Attribute + ": " + fl.Value.Value
		case fl.Value != nil:
			mm[ro.Name] = fl.Value.Attribute + ": " + fl.Value.Dir
		default:
			v := s.Fillers[ro.Name]
			if v == "" && fl.Entity != "" {
				v = x.world.Entities[fl.Entity].Display()
			}
			mm[ro.Name] = v
		}
	}
	return jev.State{Sample: s.Text, Meaning: mm}
}

// GET /api/model/jev/requests: the exact bodies Jev would get, without
// calling it (the user reviews them first).
func (s *Server) jevRequests(w http.ResponseWriter, r *http.Request) {
	x, err := s.wr()
	if err != nil {
		http.Error(w, err.Error(), 503)
		return
	}
	type item struct {
		jevItem
		Request json.RawMessage `json:"request"`
	}
	var out []item
	x.mu.RLock()
	for _, it := range x.jevSet() {
		raw, err := jev.Request(x.jevState(it.Meaning, it.Sample))
		if err != nil {
			x.mu.RUnlock()
			http.Error(w, err.Error(), 500)
			return
		}
		out = append(out, item{it, raw})
	}
	x.mu.RUnlock()
	jsonOut(w, map[string]any{"version": jev.Version, "configured": s.cfg.Jev != nil, "threshold": jev.Threshold, "items": out})
}

// POST /api/model/jev/run: send the whole set, then report per check how
// often it flagged good samples and whether it caught each broken one.
func (s *Server) jevRun(w http.ResponseWriter, r *http.Request) {
	x, err := s.wr()
	if err != nil || s.cfg.Jev == nil {
		http.Error(w, "no Jev key configured", 503)
		return
	}
	x.mu.RLock()
	set := x.jevSet()
	states := make([]jev.State, len(set))
	for i, it := range set {
		states[i] = x.jevState(it.Meaning, it.Sample)
	}
	x.mu.RUnlock()
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	type row struct {
		jevItem
		Result *jev.Result `json:"result,omitempty"`
		Error  string      `json:"error,omitempty"`
	}
	rows := make([]row, len(set))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i := range set {
		rows[i].jevItem = set[i]
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			res, err := s.cfg.Jev.Judge(ctx, states[i])
			ev := map[string]any{"sample": set[i].Sample, "meaning": set[i].Meaning, "line": set[i].Line, "expect": set[i].Expect, "broken": set[i].Broken, "prompt": jev.Version, "call": res.Call}
			if err != nil {
				rows[i].Error = err.Error()
				ev["error"] = err.Error()
				x.g.AddEvent("jev-check-failed", "model", "ganglion/jev", ev)
				return
			}
			rows[i].Result = &res
			ev["result"] = res
			x.g.AddEvent("jev-check", "model", "ganglion/jev", ev)
		}(i)
	}
	wg.Wait()
	// per check: [flagged, of good samples]; broken ones caught by the expected check
	good := map[string][2]int{}
	caught, broken := 0, 0
	for _, rw := range rows {
		if rw.Result == nil {
			continue
		}
		if rw.Expect != "" {
			broken++
		}
		for _, ch := range rw.Result.Checks {
			if rw.Expect == "" {
				g := good[ch.Name]
				g[1]++
				if ch.Flagged {
					g[0]++
				}
				good[ch.Name] = g
			} else if ch.Name == rw.Expect && ch.Flagged {
				caught++
			}
		}
	}
	jsonOut(w, map[string]any{"version": jev.Version, "threshold": jev.Threshold, "checks": jev.Names(), "good_flagged": good, "broken_caught": caught, "broken": broken, "rows": rows})
}
