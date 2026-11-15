package web

import (
	"context"
	"ganglion/internal/jev"
	"net/http"
	"sort"
	"sync"
	"time"

	"ganglion/internal/author"
	"ganglion/internal/judge"
)

// Calibration (design/synthetic-data/10/M2): the model's first pass run over
// every sample the user has judged, and how often they agree, per prompt
// version and reasoning effort.

type calJob struct {
	mu      sync.Mutex
	Running bool   `json:"running"`
	Effort  string `json:"effort"`
	Done    int    `json:"done"`
	Total   int    `json:"total"`
	Started string `json:"started,omitempty"`
}

var cal calJob

// POST /api/model/calibrate {effort}
func (s *Server) modelCalibrate(w http.ResponseWriter, r *http.Request) {
	x, err := s.wr()
	if err != nil || s.cfg.LLM == nil {
		http.Error(w, "no model configured", 503)
		return
	}
	var req struct {
		Effort string `json:"effort"` // low | medium | high, or "jev" for the baseline (10/J14)
		Small  bool   `json:"small"`  // only the small set (10/J10)
	}
	if !jsonIn(w, r, &req) {
		return
	}
	if req.Effort == "jev" && s.cfg.Jev == nil {
		http.Error(w, "no Jev key configured", 503)
		return
	}
	if req.Effort != "medium" && req.Effort != "high" && req.Effort != "jev" {
		req.Effort = "low"
	}
	cal.mu.Lock()
	if cal.Running {
		cal.mu.Unlock()
		http.Error(w, "already running", 409)
		return
	}
	// group the user's judged samples by the sentence and meaning they came from
	type group struct {
		m       author.Meaning
		line    string
		samples []author.Sample
	}
	groups := map[string]*group{}
	x.mu.RLock()
	total := 0
	pool := x.verdicts
	if req.Small {
		pool = map[string]priorVerdict{}
		for _, v := range x.smallSet() {
			pool[v.Sample.Text] = v
		}
	}
	for _, v := range pool {
		if v.Verdict == "" || v.Meaning.Frame == "" {
			continue
		}
		k := v.Line + "\x00" + author.Signature(v.Meaning)
		if groups[k] == nil {
			groups[k] = &group{m: v.Meaning, line: v.Line}
		}
		groups[k].samples = append(groups[k].samples, v.Sample)
		total++
	}
	x.mu.RUnlock()
	cal.Running, cal.Effort, cal.Done, cal.Total, cal.Started = true, req.Effort, 0, total, time.Now().UTC().Format(time.RFC3339)
	cal.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
		defer cancel()
		for _, g := range groups {
			for i := 0; i < len(g.samples); i += 16 {
				end := min(i+16, len(g.samples))
				if req.Effort == "jev" {
					s.jevMany(ctx, x, g.m, g.line, g.samples[i:end])
				} else {
					s.judgeMany(ctx, x, g.m, g.line, g.samples[i:end], req.Effort)
				}
				cal.mu.Lock()
				cal.Done += end - i
				cal.mu.Unlock()
			}
		}
		cal.mu.Lock()
		cal.Running = false
		cal.mu.Unlock()
	}()
	jsonOut(w, map[string]any{"started": true, "total": total})
}

type disagreement struct {
	Text    string         `json:"text"`
	Line    string         `json:"line"`
	Yours   string         `json:"yours"`
	Mine    priorVerdict   `json:"mine"`
	Model   modelVerdict   `json:"model"`
	At      string         `json:"at"`
	Sample  author.Sample  `json:"sample"` // so the page can change your verdict here
	Meaning author.Meaning `json:"meaning"`
}

type calStats struct {
	Prompt   string                    `json:"prompt"`
	Effort   string                    `json:"effort"`
	Current  bool                      `json:"current"` // the prompt in use now
	N        int                       `json:"n"`
	Agree    int                       `json:"agree"`
	ByYours  map[string][2]int         `json:"by_yours"` // your verdict -> [agree, n]
	Matrix   map[string]map[string]int `json:"matrix"`   // yours -> model's -> count
	Disagree []disagreement            `json:"disagree"`
	// per narrow check: [fired on your ✓, your ✓, fired on your ◐ or ✕, your ◐ or ✕] (10/J6)
	Checks map[string][4]int `json:"checks,omitempty"`
}

// GET /api/model/calibration
func (s *Server) modelCalibration(w http.ResponseWriter, r *http.Request) {
	x, err := s.wr()
	if err != nil {
		http.Error(w, err.Error(), 503)
		return
	}
	x.mu.RLock()
	stats := map[string]*calStats{}
	for k, mv := range x.modelVerdicts {
		text := k[:len(k)-len(mv.Prompt)-len(mv.Effort)-2]
		v, ok := x.verdicts[text]
		if !ok || v.Verdict == "" {
			continue
		}
		key := mv.Prompt + "/" + mv.Effort
		st := stats[key]
		if st == nil {
			st = &calStats{Prompt: mv.Prompt, Effort: mv.Effort, Current: mv.Prompt == judge.Version || mv.Prompt == jev.Version, ByYours: map[string][2]int{}, Matrix: map[string]map[string]int{}, Disagree: []disagreement{}}
			stats[key] = st
		}
		st.N++
		for _, ch := range mv.Checks {
			if st.Checks == nil {
				st.Checks = map[string][4]int{}
			}
			c := st.Checks[ch.Name]
			j := 0
			if v.Verdict != "ok" {
				j = 2
			}
			c[j+1]++
			if ch.Flagged {
				c[j]++
			}
			st.Checks[ch.Name] = c
		}
		b := st.ByYours[v.Verdict]
		b[1]++
		if st.Matrix[v.Verdict] == nil {
			st.Matrix[v.Verdict] = map[string]int{}
		}
		st.Matrix[v.Verdict][mv.Verdict.Verdict]++
		if mv.Verdict.Verdict == v.Verdict {
			st.Agree++
			b[0]++
		} else {
			st.Disagree = append(st.Disagree, disagreement{Text: text, Line: v.Line, Yours: describe(v), Mine: v, Model: mv, At: v.At, Sample: v.Sample, Meaning: v.Meaning})
		}
		st.ByYours[v.Verdict] = b
	}
	x.mu.RUnlock()
	var out []*calStats
	for _, st := range stats {
		sort.Slice(st.Disagree, func(i, j int) bool { return st.Disagree[i].At > st.Disagree[j].At })
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Current != out[j].Current {
			return out[i].Current
		}
		return out[i].Prompt+out[i].Effort < out[j].Prompt+out[j].Effort
	})
	cal.mu.Lock()
	job := map[string]any{"running": cal.Running, "effort": cal.Effort, "done": cal.Done, "total": cal.Total, "started": cal.Started}
	cal.mu.Unlock()
	x.mu.RLock()
	judged := 0
	for _, v := range x.verdicts {
		if v.Verdict != "" && v.Meaning.Frame != "" {
			judged++
		}
	}
	x.mu.RUnlock()
	var names [][2]string
	for _, d := range judge.Checks {
		names = append(names, [2]string{d.Name, d.Title})
	}
	jsonOut(w, map[string]any{"check_names": names, "stats": out, "job": job, "judged": judged, "prompt": judge.Version, "few_shot": s.cfg.FewShot})
}
