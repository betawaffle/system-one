// Package jev asks TypeSafe's Jev the narrow checks as one request: the sample
// card as the state, each check as a typed question, plus a direct verdict for
// comparison (design/synthetic-data/10/J14). Code makes the verdict from the
// checks, the same way as the local model's (10/J3).
package jev

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"

	"ganglion/internal/judge"
	"ganglion/internal/llm"
)

//go:embed questions.json
var questionsFile []byte

const stateText = `How to read this: a program generated the sample sentence from a meaning. The meaning says what it's about. The sample's tags set its tense (past, present, future), whether it affirms or denies the meaning (positive or negative), and whether it's a statement or a question. So "This ladder wasn't old." is a correct sample of "holder: ladder, property: age = old" with the tags past, negative, statement. Tags for roles say who or what fills them, including words like "the", "this", "that".

Meaning: {{.Frame}}: {{.Explainer}}
Roles:
{{range .Roles}}- {{.}}
{{end}}
The author wrote this sentence for the meaning (it's known to be good): "{{.Authored}}"

Sample: "{{.Sample}}"
Its tags: {{.Tags}}
{{if .Known}}Already recorded for this sample: {{.Known}}
{{end}}`

var stateTmpl = template.Must(template.New("state").Parse(stateText))

const combineVersion = "jev-checks-1"

// Version is the hash of the questions, the state text and the combining rules.
var Version = func() string {
	h := sha256.New()
	h.Write([]byte(combineVersion + "\x00" + stateText + "\x00"))
	h.Write(questionsFile)
	return "jev-" + hex.EncodeToString(h.Sum(nil))[:8]
}()

// Model is what we ask for; the response names the exact version.
const Model = "jev-latest"

// Config is the key file: line 1 the API key, line 2 the endpoint.
type Config struct{ Key, URL, Dir string }

func Load(path, dir string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) == "" || !strings.HasPrefix(strings.TrimSpace(lines[1]), "https://") {
		return nil, fmt.Errorf("%s: want the key on line 1 and the https endpoint on line 2", path)
	}
	return &Config{Key: strings.TrimSpace(lines[0]), URL: strings.TrimSpace(lines[1]), Dir: dir}, nil
}

// Request is the exact body sent for one sample.
func Request(in judge.Input) ([]byte, error) {
	var st bytes.Buffer
	if err := stateTmpl.Execute(&st, in); err != nil {
		return nil, err
	}
	var qs json.RawMessage = questionsFile
	var compact bytes.Buffer
	if err := json.Compact(&compact, qs); err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		State     string          `json:"state"`
		Model     string          `json:"model"`
		Questions json.RawMessage `json:"questions"`
	}{st.String(), Model, compact.Bytes()})
}

type answer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice"`
	Confidence    float64            `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
}

// flags: the answer to each check that counts against the sample, and what it gives.
var flags = []struct {
	q, bad          string
	verdict, reason string
	butKind         string
}{
	{q: "grammar", bad: "broken", verdict: "bad", reason: "Ungrammatical"},
	{q: "filler", bad: "wrong_kind", verdict: "bad", reason: "Wrong Kind of Filler"},
	{q: "meaning", bad: "differs", verdict: "bad", reason: "Means Something Else"},
	{q: "change", bad: "impossible_change", verdict: "bad", reason: "Other"},
	{q: "implies", bad: "implies_more", verdict: "but", butKind: "implies"},
	{q: "context", bad: "needs_earlier", verdict: "but", butKind: "context"},
}

var direct = map[string]judge.Verdict{
	"fine": {Verdict: "ok"}, "fine_needs_context": {Verdict: "but", ButKind: "context"}, "fine_implies_more": {Verdict: "but", ButKind: "implies"},
	"word_form": {Verdict: "bad", Reason: "Word Form"}, "wrong_kind_of_filler": {Verdict: "bad", Reason: "Wrong Kind of Filler"},
	"ungrammatical": {Verdict: "bad", Reason: "Ungrammatical"}, "means_something_else": {Verdict: "bad", Reason: "Means Something Else"},
	"sounds_odd": {Verdict: "bad", Reason: "Sounds Odd"}, "other": {Verdict: "bad", Reason: "Other"},
}

func level(c float64) string {
	switch {
	case c >= 0.8:
		return "high"
	case c >= 0.5:
		return "medium"
	}
	return "low"
}

// Result is the combined verdict, each check, and Jev's direct verdict.
type Result struct {
	Verdict judge.Verdict
	Checks  []judge.Check
	Direct  judge.Verdict
	Call    string
}

// Judge sends one sample and combines the answers.
func (c *Config) Judge(ctx context.Context, in judge.Input) (Result, error) {
	raw, err := Request(in)
	if err != nil {
		return Result{}, err
	}
	rec, err := llm.Post(ctx, c.Dir, "typesafe", "judge", Version, c.URL, c.Key, Model, raw)
	if err != nil {
		return Result{Call: rec.ID}, err
	}
	var resp struct {
		Answers map[string]answer `json:"answers"`
	}
	if err := json.Unmarshal([]byte(rec.Response), &resp); err != nil {
		return Result{Call: rec.ID}, fmt.Errorf("the answer didn't fit: %w", err)
	}
	return combine(resp.Answers, rec.ID)
}

func combine(as map[string]answer, call string) (Result, error) {
	r := Result{Call: call}
	var decided *judge.Verdict
	lowest := 1.0
	for _, f := range flags {
		a, ok := as[f.q]
		if !ok || a.Choice == "" {
			return r, fmt.Errorf("the answer didn't fit: no answer to %s", f.q)
		}
		ch := judge.Check{Name: f.q, Answer: a.Choice, Confidence: fmt.Sprintf("%.2f", a.Confidence), Flagged: a.Choice == f.bad, Call: call,
			Why: fmt.Sprintf("p(%s) = %.2f", f.bad, a.Probabilities[f.bad])}
		r.Checks = append(r.Checks, ch)
		lowest = min(lowest, a.Confidence)
		if ch.Flagged && decided == nil {
			decided = &judge.Verdict{Verdict: f.verdict, Reason: f.reason, ButKind: f.butKind, Note: f.q + ": " + a.Choice, Confidence: level(a.Confidence), Decided: f.q}
		}
	}
	if decided != nil {
		r.Verdict = *decided
	} else {
		r.Verdict = judge.Verdict{Verdict: "ok", Confidence: level(lowest)}
	}
	if a, ok := as["verdict"]; ok {
		r.Direct = direct[a.Choice]
		r.Direct.Confidence = level(a.Confidence)
		r.Direct.Note = a.Choice
	}
	return r, nil
}
