package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"ganglion/internal/author"
	"ganglion/internal/jev"
	"ganglion/internal/judge"
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
		// not "The idea is older than Noor": an idea can be (user)
		if b := v.Sample.Fillers["b"]; a != "" && b != "" && strings.HasPrefix(t, cap1(a)+" is ") && strings.HasSuffix(t, " "+b+".") {
			mid := strings.TrimSuffix(strings.TrimPrefix(t, cap1(a)+" is "), " "+b+".")
			add(v, cap1(b)+" is "+mid+" "+a+".", "meaning", "the two compared swapped", nil)
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
		case fl.Value != nil: // say who is more: "a" and "b" carry no order (user)
			rel := map[string]string{"more": "a more than b", "less": "a less than b", "same": "a and b the same"}[fl.Value.Dir]
			if rel == "" {
				rel = fl.Value.Dir
			}
			mm[ro.Name] = fl.Value.Attribute + ": " + rel
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
			mv, err := s.jevJudge(ctx, x, set[i].Meaning, set[i].Line, set[i].Sample)
			if err != nil {
				rows[i].Error = err.Error()
				return
			}
			rows[i].Result = &jev.Result{Verdict: mv.Verdict, Checks: mv.Checks, Call: mv.Call}
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

// jevKey is the sha256 of the exact request body: the same request is never
// sent twice (user: "make sure we never ask the same thing twice").
func jevKey(raw []byte) string {
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}

// noteJevVerdict replays a stored answer into the cache.
func (x *writer) noteJevVerdict(d map[string]any) {
	b, _ := json.Marshal(d)
	var v struct {
		Key    string       `json:"request_sha256"`
		Result modelVerdict `json:"result"`
	}
	if json.Unmarshal(b, &v) == nil && v.Key != "" {
		x.jevCache[v.Key] = v.Result
	}
}

// jevJudge is Jev's first pass on one sample: from the cache if this exact
// request was sent before, else one call, recorded as a jev-verdict event.
// Concurrent asks for the same request wait for the one in flight.
func (s *Server) jevJudge(ctx context.Context, x *writer, m author.Meaning, line string, smp author.Sample) (modelVerdict, error) {
	x.mu.RLock()
	st := x.jevState(m, smp)
	x.mu.RUnlock()
	raw, err := jev.Request(st)
	if err != nil {
		return modelVerdict{}, err
	}
	key := jevKey(raw)
	for {
		x.mu.Lock()
		if mv, ok := x.jevCache[key]; ok {
			x.mu.Unlock()
			return mv, nil
		}
		wait, busy := x.jevInflight[key]
		if !busy {
			x.jevInflight[key] = make(chan struct{})
			x.mu.Unlock()
			break
		}
		x.mu.Unlock()
		select {
		case <-wait:
		case <-ctx.Done():
			return modelVerdict{}, ctx.Err()
		}
	}
	defer func() {
		x.mu.Lock()
		close(x.jevInflight[key])
		delete(x.jevInflight, key)
		x.mu.Unlock()
	}()
	res, err := s.cfg.Jev.Judge(ctx, st)
	if err != nil {
		x.g.AddEvent("jev-verdict-failed", "model", "ganglion/jev", map[string]any{"request_sha256": key, "line": line, "meaning": m, "sample": smp, "call": res.Call, "error": err.Error(), "prompt": jev.Version})
		return modelVerdict{}, err
	}
	mv := modelVerdict{Verdict: res.Verdict, Checks: res.Checks, Call: res.Call, Prompt: jev.Version, Effort: "jev", At: time.Now().UTC().Format(time.RFC3339Nano)}
	x.g.AddEvent("jev-verdict", "model", "ganglion/jev", map[string]any{"request_sha256": key, "line": line, "meaning": m, "sample": smp, "result": mv})
	x.mu.Lock()
	x.jevCache[key] = mv
	x.mu.Unlock()
	return mv, nil
}

// jevMany runs the first pass on samples, a few at a time.
func (s *Server) jevMany(ctx context.Context, x *writer, m author.Meaning, line string, samples []author.Sample) []judgeOut {
	out := make([]judgeOut, len(samples))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, smp := range samples {
		out[i].Text = smp.Text
		wg.Add(1)
		go func(i int, smp author.Sample) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			mv, err := s.jevJudge(ctx, x, m, line, smp)
			if err != nil {
				out[i].Error = err.Error()
				return
			}
			out[i].Result = &mv
		}(i, smp)
	}
	wg.Wait()
	return out
}

// firstPassVersion names what's doing the first pass now.
func (s *Server) firstPassVersion() string {
	if s.cfg.Jev != nil {
		return jev.Version
	}
	return judge.Version
}
