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

	"ganglion/internal/judge"
	"ganglion/internal/llm"
)

//go:embed questions.json
var questionsFile []byte

// State is what Jev sees for one sample, kept minimal: the sentence, and its
// meaning as flat fields (frame, each role's filler, tense, polarity, mood).
type State struct {
	Sample  string            `json:"sample"`
	Meaning map[string]string `json:"meaning"`
}

const combineVersion = "jev-noul-1"

// Version is the hash of the questions and the combining rules.
var Version = func() string {
	h := sha256.New()
	h.Write([]byte(combineVersion + "\x00"))
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
func Request(st State) ([]byte, error) {
	var compact bytes.Buffer
	if err := json.Compact(&compact, questionsFile); err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		State     State           `json:"state"`
		Model     string          `json:"model"`
		Questions json.RawMessage `json:"questions"`
	}{st, Model, compact.Bytes()})
}

type answer struct {
	Type string  `json:"type"`
	Noul float64 `json:"noul"` // 0..1, how true the statement is
}

// Each question is a statement that's true of a good sample; a low value
// flags it. Threshold is a starting point, to be set from calibration.
const Threshold = 0.5

var checks = []struct {
	q               string
	verdict, reason string
	butKind         string
}{
	{q: "grammar", verdict: "bad", reason: "Ungrammatical"},
	{q: "filler", verdict: "bad", reason: "Wrong Kind of Filler"},
	{q: "meaning", verdict: "bad", reason: "Means Something Else"},
	{q: "context", verdict: "but", butKind: "context"},
}

// Names lists the checks in deciding order.
func Names() []string {
	var out []string
	for _, c := range checks {
		out = append(out, c.q)
	}
	return out
}

// Result is the combined verdict and each check.
type Result struct {
	Verdict judge.Verdict
	Checks  []judge.Check
	Call    string
}

// Judge sends one sample and combines the answers.
func (c *Config) Judge(ctx context.Context, st State) (Result, error) {
	raw, err := Request(st)
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

// combine: the first flagged check in order decides (10/J3); ✓ otherwise.
func combine(as map[string]answer, call string) (Result, error) {
	r := Result{Call: call, Verdict: judge.Verdict{Verdict: "ok", Confidence: "high"}}
	decided := false
	for _, c := range checks {
		a, ok := as[c.q]
		if !ok || a.Type != "noul" {
			return r, fmt.Errorf("the answer didn't fit: no noul answer to %s", c.q)
		}
		flag := a.Noul < Threshold
		r.Checks = append(r.Checks, judge.Check{Name: c.q, Answer: fmt.Sprintf("%.2f", a.Noul), Flagged: flag, Call: call, Confidence: level(a.Noul)})
		if flag && !decided {
			decided = true
			r.Verdict = judge.Verdict{Verdict: c.verdict, Reason: c.reason, ButKind: c.butKind, Note: fmt.Sprintf("%s: %.2f", c.q, a.Noul), Confidence: level(1 - a.Noul), Decided: c.q}
		}
	}
	return r, nil
}

// level says how far a value is from the threshold.
func level(v float64) string {
	d := v - Threshold
	if d < 0 {
		d = -d
	}
	switch {
	case d >= 0.4:
		return "high"
	case d >= 0.2:
		return "medium"
	}
	return "low"
}
