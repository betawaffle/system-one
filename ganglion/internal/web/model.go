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
	"ganglion/internal/judge"
)

// The model's first pass over samples (design/synthetic-data/10/M1, D110):
// suggestions the user reviews; only the user's verdicts count.

// modelVerdict is a stored first-pass answer.
type modelVerdict struct {
	judge.Verdict
	Checks []judge.Check  `json:"checks,omitempty"` // each narrow check's answer (10/J2)
	Direct *judge.Verdict `json:"direct,omitempty"` // Jev's own overall verdict, beside the one made from its checks (10/J14)
	Call   string         `json:"call"`
	Prompt string         `json:"prompt"`
	Effort string         `json:"effort"`
	At     string         `json:"at"`
}

func mvKey(text, prompt, effort string) string { return text + "\x00" + prompt + "\x00" + effort }

func (x *writer) noteModelVerdict(at string, d map[string]any) {
	b, _ := json.Marshal(d)
	var v struct {
		Sample author.Sample `json:"sample"`
		Result modelVerdict  `json:"result"`
	}
	if json.Unmarshal(b, &v) == nil && v.Sample.Text != "" {
		v.Result.At = at
		x.modelVerdicts[mvKey(v.Sample.Text, v.Result.Prompt, v.Result.Effort)] = v.Result
	}
}

// judgeInput describes one sample for the prompt.
func (x *writer) judgeInput(m author.Meaning, line string, s author.Sample, fewShot bool) judge.Input {
	w := x.world
	f := w.Frames[m.Frame]
	in := judge.Input{Frame: f.Title, Explainer: f.Explainer, Kind: f.Kind, Authored: line, Sample: s.Text}
	for _, ro := range f.Roles {
		fl := m.Roles[ro.Name]
		switch {
		case fl.Value != nil && fl.Value.Value != "":
			in.Roles = append(in.Roles, ro.Name+": "+fl.Value.Attribute+" = "+fl.Value.Value)
		case fl.Value != nil:
			in.Roles = append(in.Roles, ro.Name+": "+fl.Value.Attribute+", "+fl.Value.Dir)
		default:
			disp := s.Fillers[ro.Name]
			if disp == "" && fl.Entity != "" {
				disp = w.Entities[fl.Entity].Display()
			}
			in.Roles = append(in.Roles, ro.Name+" ("+ro.Explainer+"): "+disp)
		}
	}
	tags := []string{s.Features.Tense, s.Features.Polarity, s.Features.Mood}
	for _, ro := range f.Roles {
		if d := s.Fillers[ro.Name]; d != "" {
			tags = append(tags, ro.Name+": "+d)
		}
	}
	if s.Time != "" {
		tags = append(tags, "time: "+s.Time)
	}
	in.Tags = strings.Join(tags, ", ")
	if len(s.Implies) > 0 {
		in.Known = "implies " + strings.Join(s.Implies, ", implies ")
	}
	for kind, ns := range x.notes {
		label := map[string]string{"context": "needs context: ", "implies": "implies: "}[kind]
		for n := range ns {
			in.Notes = append(in.Notes, label+n)
		}
	}
	sort.Strings(in.Notes)
	if fewShot { // off by default until the prompt settles (10/D113)
		for _, v := range x.verdicts {
			if v.Line == line && len(in.Examples) < 8 {
				in.Examples = append(in.Examples, judge.Example{Text: v.Sample.Text, Verdict: describe(v)})
			}
		}
	}
	return in
}

func describe(v priorVerdict) string {
	switch v.Verdict {
	case "ok":
		return "ok"
	case "but":
		return "but (" + v.ButKind + "): " + v.Note
	}
	if v.Note == "" {
		return "bad (" + v.Reason + ")"
	}
	return "bad (" + v.Reason + "): " + v.Note
}

type judgeOut struct {
	Text   string        `json:"text"`
	Result *modelVerdict `json:"result,omitempty"`
	Error  string        `json:"error,omitempty"`
}

// judgeMany runs the first pass on samples, reusing earlier answers for the
// same prompt version and effort, one call per sample in parallel (10/D112).
func (s *Server) judgeMany(ctx context.Context, x *writer, m author.Meaning, line string, samples []author.Sample, effort string) []judgeOut {
	out := make([]judgeOut, len(samples))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i, smp := range samples {
		out[i].Text = smp.Text
		x.mu.RLock()
		cached, ok := x.modelVerdicts[mvKey(smp.Text, judge.Version, effort)]
		in := x.judgeInput(m, line, smp, s.cfg.FewShot)
		x.mu.RUnlock()
		if ok {
			c := cached
			out[i].Result = &c
			continue
		}
		wg.Add(1)
		go func(i int, smp author.Sample, in judge.Input) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			v, checks, call, err := judge.Judge(ctx, s.cfg.LLM, in, effort)
			if err != nil {
				out[i].Error = err.Error()
				x.g.AddEvent("model-verdict-failed", "model", "ganglion/model", map[string]any{"line": line, "sample": smp, "call": call, "checks": checks, "error": err.Error(), "prompt": judge.Version, "effort": effort})
				return
			}
			mv := modelVerdict{Verdict: v, Checks: checks, Call: call, Prompt: judge.Version, Effort: effort, At: time.Now().UTC().Format(time.RFC3339Nano)}
			x.g.AddEvent("model-verdict", "model", "ganglion/model", map[string]any{"line": line, "meaning": m, "sample": smp, "result": mv, "few_shot": s.cfg.FewShot})
			x.mu.Lock()
			x.modelVerdicts[mvKey(smp.Text, judge.Version, effort)] = mv
			x.mu.Unlock()
			out[i].Result = &mv
		}(i, smp, in)
	}
	wg.Wait()
	return out
}

// POST /api/model/judge {meaning, line, samples}
func (s *Server) modelJudge(w http.ResponseWriter, r *http.Request) {
	x, err := s.wr()
	if err != nil || s.cfg.LLM == nil {
		http.Error(w, "no model configured", 503)
		return
	}
	var req struct {
		Meaning author.Meaning  `json:"meaning"`
		Line    string          `json:"line"`
		Samples []author.Sample `json:"samples"`
	}
	if !jsonIn(w, r, &req) {
		return
	}
	if len(req.Samples) > 16 {
		req.Samples = req.Samples[:16]
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Minute)
	defer cancel()
	jsonOut(w, map[string]any{"results": s.judgeMany(ctx, x, req.Meaning, req.Line, req.Samples, "low"), "prompt": judge.Version})
}

// GET /api/model/status
func (s *Server) modelStatus(w http.ResponseWriter, r *http.Request) {
	if s.cfg.LLM == nil {
		jsonOut(w, map[string]any{"up": false, "state": "not configured"})
		return
	}
	up, state := s.cfg.LLM.Status(r.Context())
	jsonOut(w, map[string]any{"up": up, "state": state, "model": s.cfg.LLM.Model, "prompt": judge.Version, "few_shot": s.cfg.FewShot})
}

// GET /api/model/prompt: the current judge prompt template (10/D113a).
func (s *Server) modelPrompt(w http.ResponseWriter, r *http.Request) {
	jsonOut(w, map[string]any{"version": judge.Version, "text": judge.PromptFile(), "few_shot": s.cfg.FewShot,
		"file": "ganglion/internal/judge/prompts/checks.md"})
}

// GET /api/model/call?id=ID: exactly what was sent and what came back for
// one call, from the call log, including the model's reasoning.
func (s *Server) modelCall(w http.ResponseWriter, r *http.Request) {
	if s.cfg.LLM == nil {
		http.Error(w, "no model configured", 503)
		return
	}
	rec, err := s.cfg.LLM.Find(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, "no such call", 404)
		return
	}
	var req struct {
		Messages        []map[string]string `json:"messages"`
		ReasoningEffort string              `json:"reasoning_effort"`
		Temperature     float64             `json:"temperature"`
		Seed            int                 `json:"seed"`
	}
	json.Unmarshal(rec.Request, &req)
	var resp struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
		Usage   map[string]any `json:"usage"`
		Timings map[string]any `json:"timings"`
	}
	json.Unmarshal([]byte(rec.Response), &resp)
	out := map[string]any{"id": rec.ID, "prompt": rec.Prompt, "sent": rec.Sent, "done": rec.Done, "model": rec.Returned,
		"messages": req.Messages, "effort": req.ReasoningEffort, "temperature": req.Temperature, "seed": req.Seed,
		"usage": resp.Usage, "timings": resp.Timings, "error": rec.Error}
	if len(resp.Choices) > 0 {
		out["answer"] = resp.Choices[0].Message.Content
		out["reasoning"] = resp.Choices[0].Message.ReasoningContent
	}
	jsonOut(w, out)
}
