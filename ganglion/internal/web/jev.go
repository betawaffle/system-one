package web

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"sort"
	"sync"
	"time"

	"ganglion/internal/author"
	"ganglion/internal/jev"
	"ganglion/internal/realize"
)

// Jev as a baseline (design/synthetic-data/10/J14), and the small set to
// iterate on (10/J10).

// smallSet is every ◐ and ✕ you've judged, plus up to ten of your ✓ chosen to
// cover the traps seen so far, one per kind, in a fixed order.
func (x *writer) smallSet() []priorVerdict {
	var texts []string
	for t, v := range x.verdicts {
		if v.Verdict != "" && v.Meaning.Frame != "" {
			texts = append(texts, t)
		}
	}
	sort.Strings(texts)
	var out []priorVerdict
	var oks []priorVerdict
	for _, t := range texts {
		if v := x.verdicts[t]; v.Verdict == "ok" {
			oks = append(oks, v)
		} else {
			out = append(out, v)
		}
	}
	deixis := regexp.MustCompile(`(?i)\b(this|that)\b`)
	poss := regexp.MustCompile(`(?i)\b(his|her|their)\b`)
	kind := func(v priorVerdict) string { return x.world.Frames[v.Meaning.Frame].Kind }
	f := func(v priorVerdict) realize.Features { return v.Sample.Features }
	traps := []func(priorVerdict) bool{
		func(v priorVerdict) bool { return f(v).Polarity == "negative" && f(v).Mood == "statement" },
		func(v priorVerdict) bool { return f(v).Polarity == "positive" && f(v).Mood == "question" },
		func(v priorVerdict) bool { return f(v).Polarity == "negative" && f(v).Mood == "question" },
		func(v priorVerdict) bool { return deixis.MatchString(v.Sample.Text) },
		func(v priorVerdict) bool { return poss.MatchString(v.Sample.Text) },
		func(v priorVerdict) bool { return kind(v) == "action" && f(v).Tense == "past" },
		func(v priorVerdict) bool { return f(v).Tense == "future" },
		func(v priorVerdict) bool { return kind(v) == "state" && f(v).Tense == "past" },
		func(v priorVerdict) bool { return v.Meaning.Frame == "frame:compare" && f(v).Tense == "present" },
		func(v priorVerdict) bool {
			return f(v).Polarity == "positive" && f(v).Mood == "statement" && f(v).Tense == "present"
		},
	}
	used := map[string]bool{}
	for _, trap := range traps {
		for _, v := range oks {
			if !used[v.Sample.Text] && trap(v) {
				used[v.Sample.Text] = true
				out = append(out, v)
				break
			}
		}
	}
	return out
}

// GET /api/model/jev/requests: the exact bodies Jev would get for the small
// set, without calling it (the user reviews them first).
func (s *Server) jevRequests(w http.ResponseWriter, r *http.Request) {
	x, err := s.wr()
	if err != nil {
		http.Error(w, err.Error(), 503)
		return
	}
	x.mu.RLock()
	set := x.smallSet()
	type item struct {
		Yours   string          `json:"yours"`
		Request json.RawMessage `json:"request"`
	}
	var out []item
	for _, v := range set {
		raw, err := jev.Request(x.judgeInput(v.Meaning, v.Line, v.Sample, false))
		if err != nil {
			x.mu.RUnlock()
			http.Error(w, err.Error(), 500)
			return
		}
		out = append(out, item{Yours: describe(v), Request: raw})
	}
	x.mu.RUnlock()
	jsonOut(w, map[string]any{"version": jev.Version, "url": s.jevURL(), "configured": s.cfg.Jev != nil, "items": out})
}

func (s *Server) jevURL() string {
	if s.cfg.Jev == nil {
		return ""
	}
	return s.cfg.Jev.URL
}

// jevMany runs Jev on samples, stored like the local model's answers with
// effort "jev" so calibration compares them side by side.
func (s *Server) jevMany(ctx context.Context, x *writer, m author.Meaning, line string, samples []author.Sample) []judgeOut {
	out := make([]judgeOut, len(samples))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, smp := range samples {
		out[i].Text = smp.Text
		x.mu.RLock()
		cached, ok := x.modelVerdicts[mvKey(smp.Text, jev.Version, "jev")]
		in := x.judgeInput(m, line, smp, false)
		x.mu.RUnlock()
		if ok {
			c := cached
			out[i].Result = &c
			continue
		}
		wg.Add(1)
		go func(i int, smp author.Sample) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			res, err := s.cfg.Jev.Judge(ctx, in)
			if err != nil {
				out[i].Error = err.Error()
				x.g.AddEvent("model-verdict-failed", "model", "ganglion/jev", map[string]any{"line": line, "sample": smp, "call": res.Call, "error": err.Error(), "prompt": jev.Version, "effort": "jev"})
				return
			}
			mv := modelVerdict{Verdict: res.Verdict, Checks: res.Checks, Direct: &res.Direct, Call: res.Call, Prompt: jev.Version, Effort: "jev", At: time.Now().UTC().Format(time.RFC3339Nano)}
			x.g.AddEvent("model-verdict", "model", "ganglion/jev", map[string]any{"line": line, "meaning": m, "sample": smp, "result": mv})
			x.mu.Lock()
			x.modelVerdicts[mvKey(smp.Text, jev.Version, "jev")] = mv
			x.mu.Unlock()
			out[i].Result = &mv
		}(i, smp)
	}
	wg.Wait()
	return out
}
